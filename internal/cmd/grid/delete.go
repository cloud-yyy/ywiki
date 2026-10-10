package grid

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	"github.com/cloud-yyy/ywiki/internal/output"
)

func newDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a table",
		Long: `Delete a Yandex Wiki dynamic table.

Deleting is permanent and does not ask for confirmation.`,
		Example: `  # Delete a table
  ywiki grid delete 5f0c2d1e-...`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd, args[0])
		},
	}

	return cmd
}

func runDelete(cmd *cobra.Command, arg string) error {
	id, err := gridID(arg)
	if err != nil {
		return err
	}

	client, err := cmdutil.Client(cmd)
	if err != nil {
		return err
	}

	if deleteErr := client.DeleteGrid(cmd.Context(), id); deleteErr != nil {
		return deleteErr
	}

	if output.IsQuiet() {
		output.PrintQuiet(cmd.OutOrStdout(), id)
		return nil
	}

	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Deleted table %s\n", id)

	return err
}
