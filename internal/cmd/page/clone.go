package page

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	"github.com/cloud-yyy/ywiki/internal/cmd/jsonfields"
	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
	"github.com/cloud-yyy/ywiki/internal/output"
)

const (
	// clonePollInterval is how often the clone operation status is polled.
	clonePollInterval = time.Second

	// clonePollTimeout bounds the wait for a clone to finish.
	clonePollTimeout = 5 * time.Minute
)

// CloneFields lists the available JSON field names for clone output.
var CloneFields = []string{"operationId", "status", "id", fieldSlug}

type cloneItem struct {
	OperationID string `json:"operationId"`
	Status      string `json:"status"`
	ID          int    `json:"id,omitempty"`
	Slug        string `json:"slug,omitempty"`
}

func newCloneCmd() *cobra.Command {
	var (
		target  string
		title   string
		noWait  bool
		timeout time.Duration
	)

	cmd := &cobra.Command{
		Use:   "clone <slug|id>",
		Short: "Copy a page to a new slug",
		Long: `Copy a Yandex Wiki page to a new slug.

Cloning runs asynchronously on the server. By default the command waits for it
to finish and reports the new page; pass --no-wait to return as soon as the
operation is queued.

JSON FIELDS
  operationId, status, id, slug`,
		Example: `  # Copy a page and wait for the result
  ywiki page clone users/me/template --to users/me/q1-report

  # Queue the copy and return immediately
  ywiki page clone 12345 --to users/me/q1-report --no-wait`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if target == "" {
				return wikierrors.NewUserError(
					"missing target slug",
					"Pass the destination with --to <slug>",
				)
			}

			return runClone(cmd, args[0], api.ClonePageInput{
				Target: target,
				Title:  title,
			}, noWait, timeout)
		},
	}

	cmd.Flags().StringVar(&target, "to", "", "Destination slug (required)")
	cmd.Flags().StringVar(&title, "title", "", "Title for the copy")
	cmd.Flags().BoolVar(&noWait, "no-wait", false, "Return as soon as the copy is queued")
	cmd.Flags().DurationVar(&timeout, "timeout", clonePollTimeout, "How long to wait for the copy")

	jsonfields.Register("ywiki page clone", CloneFields)

	return cmd
}

func runClone(
	cmd *cobra.Command, ref string, in api.ClonePageInput, noWait bool, timeout time.Duration,
) error {
	if err := cmdutil.PrepareFields(cmd, "page clone", CloneFields); err != nil {
		return err
	}

	client, err := cmdutil.Client(cmd)
	if err != nil {
		return err
	}

	id, err := client.ResolvePageID(cmd.Context(), api.ParsePageLocator(ref))
	if err != nil {
		return err
	}

	started, err := client.ClonePage(cmd.Context(), id, in)
	if err != nil {
		return err
	}

	item := cloneItem{OperationID: started.Operation.ID, Status: api.OperationScheduled}

	if !noWait {
		op, waitErr := waitForClone(cmd.Context(), client, started.Operation.ID, timeout)
		if waitErr != nil {
			return waitErr
		}
		item.Status = op.Status
		if op.Result != nil {
			item.ID = op.Result.Page.ID
			item.Slug = op.Result.Page.Slug
		}
		if op.Status == api.OperationFailed {
			return wikierrors.NewUserError(
				"page copy failed",
				"Check that the destination slug is free and you can write there",
			)
		}
	}

	w := cmd.OutOrStdout()

	if output.IsJSON() {
		return cmdutil.RenderJSON(w, item)
	}

	if output.IsQuiet() {
		value := item.Slug
		if value == "" {
			value = item.OperationID
		}
		output.PrintQuiet(w, value)
		return nil
	}

	if noWait {
		_, err = fmt.Fprintf(w, "Copy queued (operation %s)\n", item.OperationID)
		return err
	}

	_, err = fmt.Fprintf(w, "Copied to %s\n%s\n", item.Slug, pageURL(item.Slug))

	return err
}

// waitForClone polls the clone operation until it finishes or the deadline
// passes. The API gives no completion callback, so polling is the only way to
// report the resulting page.
func waitForClone(
	ctx context.Context, client *api.Client, operationID string, timeout time.Duration,
) (*api.CloneOperation, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(clonePollInterval)
	defer ticker.Stop()

	for {
		op, err := client.GetCloneOperation(ctx, operationID)
		if err != nil {
			return nil, err
		}
		if op.Done() {
			return op, nil
		}

		if time.Now().After(deadline) {
			return nil, wikierrors.NewUserError(
				"timed out waiting for the page copy",
				fmt.Sprintf(
					"The copy is still running. Check it later, or rerun with --timeout. "+
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
