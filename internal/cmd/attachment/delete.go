package attachment

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
		Use:   "delete <slug|id> <file-id>",
		Short: "Delete an attached file",
		Long:  "Remove a file attached to a Yandex Wiki page.",
		Example: `  # Delete an attachment
  ywiki attachment delete users/me/notes 987`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			fileID, err := strconv.Atoi(args[1])
			if err != nil || fileID <= 0 {
				return wikierrors.NewUserError(
					"invalid file ID: "+args[1],
					"Pass the numeric file ID shown by: ywiki attachment list <page>",
				)
			}

			return runDelete(cmd, args[0], fileID)
		},
	}

	return cmd
}

func runDelete(cmd *cobra.Command, ref string, fileID int) error {
	client, err := cmdutil.Client(cmd)
	if err != nil {
		return err
	}

	pageID, err := client.ResolvePageID(cmd.Context(), api.ParsePageLocator(ref))
	if err != nil {
		return err
	}

	if deleteErr := client.DeleteAttachment(cmd.Context(), pageID, fileID); deleteErr != nil {
		return deleteErr
	}

	id := strconv.Itoa(fileID)

	if output.IsQuiet() {
		output.PrintQuiet(cmd.OutOrStdout(), id)
		return nil
	}

	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Deleted attachment %s\n", id)

	return err
}
