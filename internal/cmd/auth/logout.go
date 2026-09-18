package auth

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/config"
	"github.com/cloud-yyy/ywiki/internal/output"
)

func newLogoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Remove stored credentials",
		Long: `Delete the ywiki config file holding your token.

Credentials supplied through environment variables or flags are unaffected.`,
		Example: `  # Remove the stored token
  ywiki auth logout`,
		Args: cobra.NoArgs,
		RunE: runLogout,
	}

	return cmd
}

func runLogout(cmd *cobra.Command, _ []string) error {
	path, err := config.ConfigFilePath()
	if err != nil {
		return err
	}

	if deleteErr := config.Delete(); deleteErr != nil {
		return deleteErr
	}

	if output.IsQuiet() {
		return nil
	}

	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Removed credentials from %s\n", path)

	return err
}
