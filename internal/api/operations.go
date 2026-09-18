package api

import (
	"context"
	"net/http"
)

// Operation statuses reported by the asynchronous operations endpoints.
const (
	OperationScheduled  = "scheduled"
	OperationInProgress = "in_progress"
	OperationSuccess    = "success"
	OperationFailed     = "failed"
)

// CloneOperation is the status of an asynchronous page clone.
type CloneOperation struct {
	Status   string `json:"status"`
	Progress *struct {
		Percentage float64 `json:"percentage"`
		Details    string  `json:"details"`
	} `json:"progress"`
	Result *struct {
		Page PageRef `json:"page"`
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
