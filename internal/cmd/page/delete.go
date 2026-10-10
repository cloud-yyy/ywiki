package page

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	"github.com/cloud-yyy/ywiki/internal/output"
)

func newDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <slug|id>",
		Short: "Delete a page",
		Long: `Delete a Yandex Wiki page.

Deleting is permanent and does not ask for confirmation.`,
		Example: `  # Delete a page
  ywiki page delete users/me/scratch`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd, args[0])
		},
	}

	// The command used to ask for confirmation. Scripts that pass --yes keep
	// working; the flag does nothing now.
	cmd.Flags().BoolP("yes", "y", false, "Accepted for compatibility; deleting no longer asks")
	_ = cmd.Flags().MarkDeprecated("yes", "deleting no longer asks for confirmation")

	return cmd
}

func runDelete(cmd *cobra.Command, ref string) error {
	client, err := cmdutil.Client(cmd)
	if err != nil {
		return err
	}

	locator := api.ParsePageLocator(ref)

	// Resolve first: the delete endpoint takes an ID, and the result names the
	// page by its slug.
	page, err := client.GetPage(cmd.Context(), locator, api.GetPageOptions{})
	if err != nil {
		return err
	}

	if deleteErr := client.DeletePage(cmd.Context(), page.ID); deleteErr != nil {
		return deleteErr
	}

	if output.IsQuiet() {
		output.PrintQuiet(cmd.OutOrStdout(), page.Slug)
		return nil
	}

	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Deleted %s\n", page.Slug)

	return err
}
