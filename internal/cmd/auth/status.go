package auth

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	"github.com/cloud-yyy/ywiki/internal/cmd/jsonfields"
	"github.com/cloud-yyy/ywiki/internal/config"
	"github.com/cloud-yyy/ywiki/internal/output"
)

// StatusFields lists the available JSON field names for auth status output.
var StatusFields = []string{
	"username", "displayName", "orgId", "orgType", "tokenType", "tokenSource", "valid",
}

type statusItem struct {
	Username    string `json:"username,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	OrgID       string `json:"orgId"`
	OrgType     string `json:"orgType"`
	TokenType   string `json:"tokenType"`
	TokenSource string `json:"tokenSource"`
	Valid       bool   `json:"valid"`
}

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the active credentials",
		Long: `Show which credentials ywiki resolved and whether they still work.

The token itself is never printed; only its source (flag, env, or config).

JSON FIELDS
  username, displayName, orgId, orgType, tokenType, tokenSource, valid`,
		Example: `  # Check the current login
  ywiki auth status

  # Check it in a script
  ywiki auth status --json valid --jq '.valid'`,
		Args: cobra.NoArgs,
		RunE: runStatus,
	}

	jsonfields.Register("ywiki auth status", StatusFields)

	return cmd
}

func runStatus(cmd *cobra.Command, _ []string) error {
	if err := cmdutil.PrepareFields(cmd, "auth status", StatusFields); err != nil {
		return err
	}

	auth, err := config.ResolveAuth(cmdutil.AuthFlags(cmd))
	if err != nil {
		return err
	}

	item := statusItem{
		OrgID:       auth.OrgID,
		OrgType:     string(auth.OrgType),
		TokenType:   string(auth.TokenType),
		TokenSource: auth.TokenSource,
	}

	// Credentials can be well-formed but revoked or expired, which matters
	// most for Cloud IAM tokens: they last 12 hours. Confirm with a live call.
	user, err := cmdutil.NewClient(auth).GetCurrentUser(cmd.Context())
	if err == nil {
		item.Valid = true
		item.Username = user.Username
		item.DisplayName = user.DisplayName
	}

	w := cmd.OutOrStdout()

	if output.IsJSON() {
		return cmdutil.RenderJSON(w, item)
	}

	if output.IsQuiet() {
		output.PrintQuiet(w, item.Username)
		return nil
	}

	if !item.Valid {
		_, _ = fmt.Fprintf(w, "Not logged in (credentials from %s were rejected)\n", item.TokenSource)
		_, _ = fmt.Fprintf(w, "org: %s (%s)\n", item.OrgID, item.OrgType)
		return err
	}

	name := item.DisplayName
	if name == "" {
		name = item.Username
	}

	_, _ = fmt.Fprintf(w, "Logged in as %s\n", name)
	_, _ = fmt.Fprintf(w, "org: %s (%s)\n", item.OrgID, item.OrgType)
	_, _ = fmt.Fprintf(w, "token type: %s\n", item.TokenType)
	_, _ = fmt.Fprintf(w, "token source: %s\n", item.TokenSource)

	return nil
}
