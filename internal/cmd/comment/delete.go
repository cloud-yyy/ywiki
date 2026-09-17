package comment

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
	"github.com/cloud-yyy/ywiki/internal/output"
)

func newDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <slug|id> <comment-id>",
		Short: "Delete a comment",
		Long:  "Delete a comment from a Yandex Wiki page.",
		Example: `  # Delete a comment
  ywiki comment delete users/me/notes 42`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			commentID, err := strconv.Atoi(args[1])
			if err != nil || commentID <= 0 {
				return wikierrors.NewUserError(
					"invalid comment ID: "+args[1],
					"Pass the numeric comment ID shown by: ywiki comment list <page>",
				)
			}

			return runDelete(cmd, args[0], commentID)
		},
	}

	return cmd
}

func runDelete(cmd *cobra.Command, ref string, commentID int) error {
	client, err := cmdutil.Client(cmd)
	if err != nil {
		return err
	}

	pageID, err := client.ResolvePageID(cmd.Context(), api.ParsePageLocator(ref))
	if err != nil {
		return err
	}

	if deleteErr := client.DeleteComment(cmd.Context(), pageID, commentID); deleteErr != nil {
		return deleteErr
	}

	id := strconv.Itoa(commentID)

	if output.IsQuiet() {
		output.PrintQuiet(cmd.OutOrStdout(), id)
		return nil
	}

	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Deleted comment %s\n", id)

	return err
}
