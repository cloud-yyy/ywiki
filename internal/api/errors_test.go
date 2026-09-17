package api_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/cloud-yyy/ywiki/internal/config"
	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
)

func TestErrorMapping(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		body         string
		wantExitCode int
		wantCode     string
		wantMessage  string
	}{
		{
			name:         "401 is an auth error",
			status:       http.StatusUnauthorized,
			body:         `{"error_code":"UNAUTHORIZED","debug_message":"Token is invalid"}`,
			wantExitCode: wikierrors.ExitAuthError,
			wantCode:     wikierrors.CodeAuthError,
			wantMessage:  "Token is invalid (UNAUTHORIZED)",
		},
		{
			name:         "403 is an auth error",
			status:       http.StatusForbidden,
			body:         `{"error_code":"FORBIDDEN","debug_message":"No access"}`,
			wantExitCode: wikierrors.ExitAuthError,
			wantCode:     wikierrors.CodeAuthError,
			wantMessage:  "No access (FORBIDDEN)",
		},
		{
			name:         "404 is a not-found error",
			status:       http.StatusNotFound,
			body:         `{"error_code":"PAGE_NOT_FOUND","debug_message":"Page not found"}`,
			wantExitCode: wikierrors.ExitNotFound,
			wantCode:     wikierrors.CodeNotFound,
			wantMessage:  "Page not found (PAGE_NOT_FOUND)",
		},
		{
			name:         "429 is a rate-limit error",
			status:       http.StatusTooManyRequests,
			body:         `{"error_code":"TOO_MANY_REQUESTS","debug_message":"Slow down"}`,
			wantExitCode: wikierrors.ExitRateLimited,
			wantCode:     wikierrors.CodeRateLimited,
			wantMessage:  "Slow down (TOO_MANY_REQUESTS)",
		},
		{
			name:         "400 is a user error",
			status:       http.StatusBadRequest,
			body:         `{"error_code":"VALIDATION_ERROR","debug_message":"Validation failed"}`,
			wantExitCode: wikierrors.ExitUserError,
			wantCode:     wikierrors.CodeUserError,
			wantMessage:  "Validation failed (VALIDATION_ERROR)",
		},
		{
			name:         "a body that is not the error envelope still reports the status",
			status:       http.StatusBadGateway,
			body:         `<html>gateway error</html>`,
			wantExitCode: wikierrors.ExitUserError,
			wantCode:     wikierrors.CodeUserError,
			wantMessage:  "Wiki API error (HTTP 502)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}, config.OrgType360)

			_, err := client.GetCurrentUser(context.Background())
			if err == nil {
				t.Fatal("GetCurrentUser returned nil error, want an error")
			}

			var exitErr *wikierrors.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("error %v is not an *ExitError", err)
			}
			if exitErr.ExitCode != tt.wantExitCode {
				t.Errorf("ExitCode = %d, want %d", exitErr.ExitCode, tt.wantExitCode)
			}
			if exitErr.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", exitErr.Code, tt.wantCode)
			}
			if exitErr.Message != tt.wantMessage {
				t.Errorf("Message = %q, want %q", exitErr.Message, tt.wantMessage)
			}
		})
	}
}

func TestValidationErrorNamesFields(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{
			"error_code": "VALIDATION_ERROR",
			"debug_message": "Validation failed",
			"details": {"body": {"title": [{"error_code": "value_error.missing"}]}}
		}`))
	}, config.OrgType360)

	_, err := client.GetCurrentUser(context.Background())

	var exitErr *wikierrors.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error %v is not an *ExitError", err)
	}
	if exitErr.Suggestion != "Invalid fields: body.title" {
		t.Errorf("Suggestion = %q, want %q", exitErr.Suggestion, "Invalid fields: body.title")
	}
}

func TestMissingCredentialsFailBeforeRequest(t *testing.T) {
	client := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Error("request was sent despite invalid credentials")
	}, config.OrgType("bogus"))

	_, err := client.GetCurrentUser(context.Background())
	if err == nil {
		t.Fatal("GetCurrentUser returned nil error for an invalid org type")
	}
}
