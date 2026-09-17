// Package user provides account lookup commands for the ywiki CLI.
package user

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	"github.com/cloud-yyy/ywiki/internal/cmd/jsonfields"
	"github.com/cloud-yyy/ywiki/internal/output"
)

// UserFields lists the available JSON field names for user output.
var UserFields = []string{"id", "username", "displayName", "affiliation"}

type userItem struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName,omitempty"`
	Affiliation string `json:"affiliation,omitempty"`
}

// NewCmd creates the parent "user" command with its subcommands.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Look up Wiki users",
		Long:  "Look up the account ywiki is acting as.",
	}

	cmd.AddCommand(newMeCmd())

	return cmd
}

func newMeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "me",
		Short: "Show the authenticated user",
		Long: `Show the Wiki account the current token belongs to.

Wiki API requests always act as a person, never a service account, so this is
the identity whose permissions apply to every other command.

JSON FIELDS
  id, username, displayName, affiliation`,
		Example: `  # Show the current user
  ywiki user me

  # Get the username only
  ywiki user me --json username --jq '.username'`,
		Args: cobra.NoArgs,
		RunE: runMe,
	}

	jsonfields.Register("ywiki user me", UserFields)

	return cmd
}

func runMe(cmd *cobra.Command, _ []string) error {
	if err := cmdutil.PrepareFields(cmd, "user me", UserFields); err != nil {
		return err
	}

	client, err := cmdutil.Client(cmd)
	if err != nil {
		return err
	}

	user, err := client.GetCurrentUser(cmd.Context())
	if err != nil {
		return err
	}

	item := userItem{
		ID:          strconv.Itoa(user.ID),
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Affiliation: user.Affiliation,
	}

	w := cmd.OutOrStdout()

	if output.IsJSON() {
		return cmdutil.RenderJSON(w, item)
	}

	if output.IsQuiet() {
		output.PrintQuiet(w, item.Username)
		return nil
	}

	_, _ = fmt.Fprintf(w, "%s\n", item.DisplayName)
	_, _ = fmt.Fprintf(w, "username: %s\n", item.Username)
	_, _ = fmt.Fprintf(w, "id: %s\n", item.ID)

	return nil
}
