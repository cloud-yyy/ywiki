package api

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
)

// Operation statuses reported by the asynchronous operations endpoints.
const (
	OperationScheduled  = "scheduled"
	OperationInProgress = "in_progress"
	OperationSuccess    = "success"
	OperationFailed     = "failed"
)

// CloneOperation is the status of an asynchronous page or table clone.
type CloneOperation struct {
	Status   string `json:"status"`
	Progress *struct {
		Percentage float64 `json:"percentage"`
		Details    string  `json:"details"`
	} `json:"progress"`
	Result *struct {
		Page PageRef `json:"page"`

		// GridID is the ID of the new table, set by table clones only.
		GridID string `json:"grid_id"`
	} `json:"result"`
}

// Done reports whether the operation reached a terminal state.
func (o *CloneOperation) Done() bool {
	return o.Status == OperationSuccess || o.Status == OperationFailed
}

// GetCloneOperation reads the status of a clone operation.
func (c *Client) GetCloneOperation(ctx context.Context, taskID string) (*CloneOperation, error) {
	var op CloneOperation
	if err := c.do(ctx, request{
		method: http.MethodGet,
		path:   "/operations/clone/" + taskID,
	}, &op); err != nil {
		return nil, err
	}

	return &op, nil
}

// GetOperationByURL reads an operation from the status_url its start response
// gave. Only the part from "/operations/" on is used, so the request goes
// through this client's base URL and credentials whatever host the API reports.
func (c *Client) GetOperationByURL(ctx context.Context, statusURL string) (*CloneOperation, error) {
	parsed, err := url.Parse(statusURL)
	if err != nil {
		return nil, wikierrors.NewUserError("invalid operation URL: "+statusURL, "")
	}

	_, path, found := strings.Cut(parsed.Path, "/operations/")
	if !found {
		return nil, wikierrors.NewUserError("unexpected operation URL: "+statusURL, "")
	}

	var op CloneOperation
	if err := c.do(ctx, request{
		method: http.MethodGet,
		path:   "/operations/" + path,
	}, &op); err != nil {
		return nil, err
	}

	return &op, nil
}
