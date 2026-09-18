package page

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
	"github.com/cloud-yyy/ywiki/internal/output"
)

func newDeleteCmd() *cobra.Command {
	var confirm bool

	cmd := &cobra.Command{
		Use:   "delete <slug|id>",
		Short: "Delete a page",
		Long: `Delete a Yandex Wiki page.

Deleting is destructive, so the command asks for confirmation when run in a
terminal and refuses to run unattended without --yes.`,
		Example: `  # Delete a page, with a confirmation prompt
  ywiki page delete users/me/scratch

  # Delete a page from a script
  ywiki page delete users/me/scratch --yes`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd, args[0], confirm)
		},
	}

	cmd.Flags().BoolVarP(&confirm, "yes", "y", false, "Skip the confirmation prompt")

	return cmd
}

func runDelete(cmd *cobra.Command, ref string, confirm bool) error {
	client, err := cmdutil.Client(cmd)
	if err != nil {
		return err
	}

	locator := api.ParsePageLocator(ref)

	// Resolve first so the prompt names the page the user is about to lose,
	// and so a typo fails as "not found" before anything is deleted.
	page, err := client.GetPage(cmd.Context(), locator, api.GetPageOptions{})
	if err != nil {
		return err
	}

	if !confirm {
		if !output.IsTTY() {
			return wikierrors.NewUserError(
				"refusing to delete without confirmation",
				"Rerun with --yes to confirm",
			)
		}
		ok, promptErr := confirmDelete(cmd, page)
		if promptErr != nil {
			return promptErr
		}
		if !ok {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Canceled")
			return nil
		}
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

func confirmDelete(cmd *cobra.Command, page *api.Page) (bool, error) {
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Delete %q (%s)? [y/N] ", page.Title, page.Slug)

	reader := bufio.NewReader(cmd.InOrStdin())
	answer, err := reader.ReadString('\n')
	if err != nil && answer == "" {
		return false, fmt.Errorf("failed to read confirmation: %w", err)
	}

	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
