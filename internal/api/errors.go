package api

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
	"github.com/cloud-yyy/ywiki/internal/output"
)

// errorBodyLimit caps how much of an error response is read. Wiki error
// payloads are small; anything larger is a proxy or gateway page.
const errorBodyLimit = 64 * 1024

// APIError is the error envelope returned by the Yandex Wiki API:
// {"error_code": "...", "debug_message": "...", "details": {...}}.
type APIError struct {
	ErrorCode    string          `json:"error_code"`
	DebugMessage string          `json:"debug_message"`
	Details      json.RawMessage `json:"details"`
}

// mapHTTPError converts a non-2xx response into a typed ExitError carrying the
// matching semantic exit code. The HTTP status drives the classification; the
// API's own error_code and debug_message supply the message text.
func mapHTTPError(resp *http.Response) error {
	apiErr := decodeAPIError(resp)
	message := errorMessage(apiErr, resp.StatusCode)

	debugAPIError(resp, apiErr)

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return wikierrors.NewAuthError(
			message,
			"Check your token and organization ID, or run: ywiki auth login",
		)
	case http.StatusForbidden:
		return wikierrors.NewAuthError(
			message,
			"Your account lacks permission for this page. "+
				"Wiki API requests act as you, so Wiki access rules apply.",
		)
	case http.StatusNotFound:
		return wikierrors.NewNotFoundError(message, "Check the page slug or ID")
	case http.StatusConflict:
		return wikierrors.NewUserError(
			message,
			"The page changed since it was read. Retry, or pass --allow-merge to merge edits.",
		)
	case http.StatusTooManyRequests:
		return wikierrors.NewRateLimitedError(message, "Wait and retry")
	}

	if resp.StatusCode >= http.StatusInternalServerError {
		return &wikierrors.ExitError{
			ExitCode: wikierrors.ExitUserError,
			Code:     wikierrors.CodeUserError,
			Message:  message,
			Suggestion: "The Wiki API returned a server error. " +
				"Retry later; if it persists, report it with --debug output.",
		}
	}

	return wikierrors.NewUserError(message, validationSuggestion(apiErr))
}

// mapTransportError converts a network-level failure into a typed error so the
// CLI does not surface raw url.Error text.
func mapTransportError(err error) error {
	if stderrors.Is(err, context.Canceled) {
		return &wikierrors.ExitError{
			ExitCode: wikierrors.ExitInterrupted,
			Code:     wikierrors.CodeUserError,
			Message:  "request canceled",
		}
	}

	if stderrors.Is(err, context.DeadlineExceeded) {
		return wikierrors.NewUserError(
			"request timed out",
			"Check your network connection and retry",
		)
	}

	output.Debugf("transport_error error=%q", output.SanitizeDebugString(err.Error()))

	return wikierrors.NewUserError(
		"failed to reach the Wiki API",
		"Check your network connection and retry",
	)
}

func decodeAPIError(resp *http.Response) *APIError {
	if resp.Body == nil {
		return nil
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, errorBodyLimit))
	if err != nil || len(data) == 0 {
		return nil
	}

	var apiErr APIError
	if err := json.Unmarshal(data, &apiErr); err != nil {
		return nil
	}
	if apiErr.ErrorCode == "" && apiErr.DebugMessage == "" {
		return nil
	}

	return &apiErr
}

func errorMessage(apiErr *APIError, statusCode int) string {
	if apiErr == nil {
		return fmt.Sprintf("Wiki API error (HTTP %d)", statusCode)
	}

	message := strings.TrimSpace(apiErr.DebugMessage)
	switch {
	case message == "" && apiErr.ErrorCode == "":
		return fmt.Sprintf("Wiki API error (HTTP %d)", statusCode)
	case message == "":
		return "Wiki API error: " + apiErr.ErrorCode
	case apiErr.ErrorCode == "":
		return message
	}

	return fmt.Sprintf("%s (%s)", message, apiErr.ErrorCode)
}

// validationSuggestion surfaces the offending field names for VALIDATION_ERROR
// responses, whose details map sources ("body", "query") to per-field errors.
func validationSuggestion(apiErr *APIError) string {
	if apiErr == nil || len(apiErr.Details) == 0 {
		return ""
	}

	var details map[string]map[string]json.RawMessage
	if err := json.Unmarshal(apiErr.Details, &details); err != nil {
		return ""
	}

	var fields []string
	for source, sourceFields := range details {
		for field := range sourceFields {
			fields = append(fields, source+"."+field)
		}
	}
	if len(fields) == 0 {
		return ""
	}

	return "Invalid fields: " + strings.Join(fields, ", ")
}

func debugAPIError(resp *http.Response, apiErr *APIError) {
	if !output.DebugEnabled() || resp == nil {
		return
	}

	method, path := "-", "-"
	if resp.Request != nil {
		method = resp.Request.Method
		path = requestPath(resp.Request.URL)
	}

	output.Debugf("api_error status=%d method=%s path=%s", resp.StatusCode, method, path)

	if apiErr != nil {
		output.Debugf("api_error_code code=%q message=%q",
			apiErr.ErrorCode, output.SanitizeDebugString(apiErr.DebugMessage))
	}
}
