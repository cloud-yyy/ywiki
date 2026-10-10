package grid

import (
	"cmp"
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

// CloneFields lists the available JSON field names for table clone output.
var CloneFields = []string{"operationId", "status", "id"}

type cloneItem struct {
	OperationID string `json:"operationId"`
	Status      string `json:"status"`
	ID          string `json:"id,omitempty"`
}

func newCloneCmd() *cobra.Command {
	var (
		target   string
		title    string
		withData bool
		noWait   bool
		timeout  time.Duration
	)

	cmd := &cobra.Command{
		Use:   "clone <id>",
		Short: "Copy a table to another page",
		Long: `Copy a Yandex Wiki dynamic table to another page.

By default only the structure is copied; pass --with-data to copy the rows
too. The page is created if it does not exist. Cloning runs asynchronously on
the server: the command waits for it to finish, or with --no-wait returns as
soon as it is queued. Once finished, the output carries the ID of the copy.

JSON FIELDS
  operationId, status, id`,
		Example: `  # Copy a table with its rows to another page
  ywiki grid clone 5f0c2d1e-... --to users/me/archive --with-data

  # Queue the copy and return immediately
  ywiki grid clone 5f0c2d1e-... --to users/me/archive --no-wait`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if target == "" {
				return wikierrors.NewUserError(
					"missing target page",
					"Pass the destination page slug with --to <slug>",
				)
			}

			return runClone(cmd, args[0], api.CloneGridInput{
				Target:   target,
				Title:    title,
				WithData: withData,
			}, noWait, timeout)
		},
	}

	cmd.Flags().StringVar(&target, "to", "", "Slug of the destination page (required)")
	cmd.Flags().StringVar(&title, "title", "", "Title for the copy")
	cmd.Flags().BoolVar(&withData, "with-data", false, "Copy the rows as well as the structure")
	cmd.Flags().BoolVar(&noWait, "no-wait", false, "Return as soon as the copy is queued")
	cmd.Flags().DurationVar(&timeout, "timeout", cmdutil.OperationPollTimeout, "How long to wait for the copy")

	jsonfields.Register("ywiki grid clone", CloneFields)

	return cmd
}

func runClone(
	cmd *cobra.Command, arg string, in api.CloneGridInput, noWait bool, timeout time.Duration,
) error {
	if err := cmdutil.PrepareFields(cmd, "grid clone", CloneFields); err != nil {
		return err
	}

	id, err := gridID(arg)
	if err != nil {
		return err
	}

	client, err := cmdutil.Client(cmd)
	if err != nil {
		return err
	}

	started, err := client.CloneGrid(cmd.Context(), id, in)
	if err != nil {
		return err
	}

	item := cloneItem{OperationID: started.Operation.ID, Status: api.OperationScheduled}

	if !noWait && started.StatusURL != "" {
		op, waitErr := cmdutil.WaitForOperation(cmd.Context(),
			func(ctx context.Context) (*api.CloneOperation, error) {
				return client.GetOperationByURL(ctx, started.StatusURL)
			}, timeout, "table copy", started.Operation.ID)
		if waitErr != nil {
			return waitErr
		}

		item.Status = op.Status
		if op.Result != nil {
			item.ID = op.Result.GridID
		}
		if op.Status == api.OperationFailed {
			return wikierrors.NewUserError(
				"table copy failed",
				"Check that you can write to the destination page",
			)
		}
	}

	w := cmd.OutOrStdout()

	if output.IsJSON() {
		return cmdutil.RenderJSON(w, item)
	}

	if output.IsQuiet() {
		output.PrintQuiet(w, cmp.Or(item.ID, item.OperationID))
		return nil
	}

	if item.Status != api.OperationSuccess {
		_, err = fmt.Fprintf(w, "Copy queued (operation %s)\n", item.OperationID)
		return err
	}

	_, err = fmt.Fprintf(w, "Copied to %s as table %s\n", in.Target, item.ID)

	return err
}
