// Package auth provides credential management commands for the ywiki CLI.
package auth

import (
	"github.com/spf13/cobra"
)

// NewCmd creates the parent "auth" command with its subcommands.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage ywiki credentials",
		Long: `Store, inspect, and remove the credentials ywiki uses.

Credentials resolve in three tiers, highest first: the --token, --org-id, and
--org-type flags; the YWIKI_TOKEN, YWIKI_ORG_ID, and YWIKI_ORG_TYPE
environment variables; and the config file written by "ywiki auth login".`,
	}

	cmd.AddCommand(
		newLoginCmd(),
		newStatusCmd(),
		newLogoutCmd(),
	)

	return cmd
}
