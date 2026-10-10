package cmdutil

import (
	"context"
	"fmt"
	"time"

	"github.com/cloud-yyy/ywiki/internal/api"
	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
)

const (
	// OperationPollInterval is how often an asynchronous operation is polled.
	OperationPollInterval = time.Second

	// OperationPollTimeout is the default bound on waiting for an operation.
	OperationPollTimeout = 5 * time.Minute
)

// WaitForOperation polls an asynchronous operation until it finishes or the
// timeout passes. The API gives no completion callback, so polling is the only
// way to report the outcome. what names the operation in the timeout message.
func WaitForOperation(
	ctx context.Context,
	fetch func(context.Context) (*api.CloneOperation, error),
	timeout time.Duration,
	what, operationID string,
) (*api.CloneOperation, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(OperationPollInterval)
	defer ticker.Stop()

	for {
		op, err := fetch(ctx)
		if err != nil {
			return nil, err
		}
		if op.Done() {
			return op, nil
		}

		if time.Now().After(deadline) {
			return nil, wikierrors.NewUserError(
				"timed out waiting for the "+what,
				fmt.Sprintf(
					"It is still running. Check it later, or rerun with --timeout. "+
						"Operation: %s", operationID,
				),
			)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
