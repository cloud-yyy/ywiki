// Package api implements a client for the Yandex Wiki public API.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cloud-yyy/ywiki/internal/config"
	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
	"github.com/cloud-yyy/ywiki/internal/version"
)

// DefaultBaseURL is the Yandex Wiki API root.
const DefaultBaseURL = "https://api.wiki.yandex.net/v1"

const httpTimeout = 30 * time.Second

// Client talks to the Yandex Wiki API on behalf of a single user.
// The API has no service-account mode: every request carries a personal OAuth
// or IAM token and is authorized with that user's own Wiki permissions.
type Client struct {
	baseURL    string
	httpClient *http.Client
	auth       *config.ResolvedAuth

	// authErr, when set, is returned by every request. It lets NewClient
	// report a bad credential set lazily, at the point a command actually
	// needs the network, instead of failing commands that never call the API.
	authErr error
}

// Option customizes a Client.
type Option func(*Client)

// WithBaseURL overrides the API root. Tests use it to point at a local server.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimSuffix(baseURL, "/")
	}
}

// WithHTTPClient overrides the underlying HTTP client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// NewClient builds a Client from resolved credentials. A nil or invalid auth
// set produces a Client whose requests fail with an actionable error.
func NewClient(auth *config.ResolvedAuth, opts ...Option) *Client {
	c := &Client{
		baseURL: DefaultBaseURL,
		auth:    auth,
	}

	if auth == nil {
		c.authErr = wikierrors.NewAuthError(
			"not authenticated",
			"Run: ywiki auth login\n"+
				"Or set: export YWIKI_TOKEN=<token> YWIKI_ORG_ID=<org> YWIKI_ORG_TYPE=<360|cloud>",
		)
	} else if _, err := config.ParseOrgType(string(auth.OrgType)); err != nil {
		c.authErr = wikierrors.NewUserError(
			err.Error(),
			"Use org-type 360 or cloud, or rerun ywiki auth login",
		)
	} else if _, err := config.ParseTokenType(string(auth.TokenType)); err != nil {
		c.authErr = wikierrors.NewUserError(
			err.Error(),
			"Use token-type oauth or iam, or rerun ywiki auth login",
		)
	}

	source := ""
	if auth != nil {
		source = auth.TokenSource
	}
	c.httpClient = &http.Client{
		Timeout:   httpTimeout,
		Transport: newDebugTransport(nil, source),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// request describes a single API call. body is marshaled as JSON; rawBody
// sends bytes verbatim (used for binary upload parts) and takes precedence.
type request struct {
	method      string
	path        string
	query       url.Values
	body        any
	rawBody     io.Reader
	contentType string
}

// do executes an API call and decodes a JSON response into out.
// out may be nil for endpoints that return no useful body.
func (c *Client) do(ctx context.Context, r request, out any) error {
	resp, err := c.send(ctx, r)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= http.StatusBadRequest {
		return mapHTTPError(resp)
	}

	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("failed to decode API response: %w", err)
	}

	return nil
}

// doRaw executes an API call and returns the undecoded response body. The
// caller owns the returned ReadCloser. It is used for file downloads, where
// the payload is arbitrary bytes rather than JSON.
func (c *Client) doRaw(ctx context.Context, r request) (io.ReadCloser, error) {
	resp, err := c.send(ctx, r)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		defer func() { _ = resp.Body.Close() }()
		return nil, mapHTTPError(resp)
	}

	return resp.Body, nil
}

func (c *Client) send(ctx context.Context, r request) (*http.Response, error) {
	if c.authErr != nil {
		return nil, c.authErr
	}

	var bodyReader io.Reader
	contentType := r.contentType
	switch {
	case r.rawBody != nil:
		bodyReader = r.rawBody
		if contentType == "" {
			contentType = "application/octet-stream"
		}
	case r.body != nil:
		encoded, err := json.Marshal(r.body)
		if err != nil {
			return nil, fmt.Errorf("failed to encode request body: %w", err)
		}
		bodyReader = bytes.NewReader(encoded)
		contentType = "application/json"
	}

	endpoint := c.baseURL + r.path
	if len(r.query) > 0 {
		endpoint += "?" + r.query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, r.method, endpoint, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ywiki/"+version.Version)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.auth != nil {
		tokenType := c.auth.TokenType
		if tokenType == "" {
			tokenType = c.auth.OrgType.DefaultTokenType()
		}
		req.Header.Set("Authorization", tokenType.AuthScheme()+" "+c.auth.Token)
		req.Header.Set(c.auth.OrgType.OrgHeader(), c.auth.OrgID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, mapTransportError(err)
	}

	return resp, nil
}
