package auth

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	"github.com/cloud-yyy/ywiki/internal/config"
	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
	"github.com/cloud-yyy/ywiki/internal/output"
)

func newLoginCmd() *cobra.Command {
	var (
		token     string
		orgID     string
		orgType   string
		withToken bool
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Store credentials for ywiki",
		Long: `Store a Wiki token and organization ID in the ywiki config file.

The token is read interactively without echoing, or from stdin with
--with-token so it never lands in your shell history. Credentials are verified
against the API before they are saved, and written with 0600 permissions.

Yandex 360 and Identity Hub organizations use an OAuth token (org type 360);
Yandex Cloud organizations use an IAM token (org type cloud). When --org-type
is omitted, ywiki tries both and keeps the one that works.`,
		Example: `  # Log in interactively
  ywiki auth login --org-id 1234567

  # Log in from a secret store
  cat token.txt | ywiki auth login --org-id 1234567 --with-token`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogin(cmd, token, orgID, orgType, withToken)
		},
	}

	cmd.Flags().StringVar(&token, "token", "", "Token to store (prefer --with-token or the prompt)")
	cmd.Flags().StringVar(&orgID, "org-id", "", "Wiki organization ID (required)")
	cmd.Flags().StringVar(&orgType, "org-type", "", "Organization type: 360 or cloud (detected if omitted)")
	cmd.Flags().BoolVar(&withToken, "with-token", false, "Read the token from stdin")

	return cmd
}

func runLogin(cmd *cobra.Command, token, orgID, orgType string, withToken bool) error {
	if orgID == "" {
		return wikierrors.NewUserError(
			"missing organization ID",
			"Pass --org-id <id>. It is shown in your Yandex 360, Identity Hub, or Cloud console.",
		)
	}

	token, err := resolveToken(cmd, token, withToken)
	if err != nil {
		return err
	}

	resolved, user, err := verify(cmd.Context(), token, orgID, orgType)
	if err != nil {
		return err
	}

	if err := config.Save(&config.Config{
		Token:   resolved.Token,
		OrgID:   resolved.OrgID,
		OrgType: resolved.OrgType,
	}); err != nil {
		return err
	}

	path, _ := config.ConfigFilePath()

	if output.IsQuiet() {
		output.PrintQuiet(cmd.OutOrStdout(), user.Username)
		return nil
	}

	name := user.DisplayName
	if name == "" {
		name = user.Username
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Logged in as %s (org %s, type %s)\n",
		name, resolved.OrgID, resolved.OrgType)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Credentials saved to %s\n", path)

	return nil
}

// resolveToken obtains the token without putting it in argv where possible:
// from stdin with --with-token, or from a no-echo terminal prompt.
func resolveToken(cmd *cobra.Command, token string, withToken bool) (string, error) {
	switch {
	case withToken && token != "":
		return "", wikierrors.NewUserError(
			"--token and --with-token are mutually exclusive",
			"Pass the token one way, not both",
		)
	case token != "":
		return strings.TrimSpace(token), nil
	case withToken:
		reader := bufio.NewReader(cmd.InOrStdin())
		line, err := reader.ReadString('\n')
		if err != nil && line == "" {
			return "", wikierrors.NewUserError(
				"no token on stdin",
				"Pipe the token in: cat token.txt | ywiki auth login --org-id <id> --with-token",
			)
		}
		return strings.TrimSpace(line), nil
	}

	if !isatty.IsTerminal(os.Stdin.Fd()) {
		return "", wikierrors.NewUserError(
			"no token provided",
			"Pipe the token in with --with-token, or run the command in a terminal",
		)
	}

	fd, ok := output.FileDescriptor(os.Stdin)
	if !ok {
		return "", wikierrors.NewUserError(
			"cannot read from stdin",
			"Pipe the token in with --with-token",
		)
	}

	_, _ = fmt.Fprint(cmd.ErrOrStderr(), "Token: ")
	raw, err := term.ReadPassword(fd)
	_, _ = fmt.Fprintln(cmd.ErrOrStderr())
	if err != nil {
		return "", fmt.Errorf("failed to read token: %w", err)
	}

	value := strings.TrimSpace(string(raw))
	if value == "" {
		return "", wikierrors.NewUserError("empty token", "Paste the token at the prompt")
	}

	return value, nil
}

// verify checks the credentials against the API before they are stored, so a
// bad token fails here rather than on some later command. With no explicit
// org type, both are tried: the two differ only in header and token scheme, so
// one round trip each is enough to tell them apart.
func verify(
	ctx context.Context, token, orgID, orgType string,
) (*config.ResolvedAuth, *api.User, error) {
	candidates := []config.OrgType{config.OrgType360, config.OrgTypeCloud}
	if orgType != "" {
		parsed, err := config.ParseOrgType(orgType)
		if err != nil {
			return nil, nil, wikierrors.NewUserError(
				err.Error(),
				"Use --org-type 360 or --org-type cloud",
			)
		}
		candidates = []config.OrgType{parsed}
	}

	var lastErr error
	for _, candidate := range candidates {
		auth := &config.ResolvedAuth{
			Token:       token,
			OrgID:       orgID,
			OrgType:     candidate,
			TokenSource: "flag",
		}

		user, err := cmdutil.NewClient(auth).GetCurrentUser(ctx)
		if err == nil {
			return auth, user, nil
		}
		lastErr = err
	}

	if len(candidates) == 1 {
		return nil, nil, lastErr
	}

	return nil, nil, wikierrors.NewAuthError(
		"could not authenticate with the given token and organization ID",
		"Check the token and organization ID. If the organization type is known, "+
			"pass --org-type 360 or --org-type cloud for the exact error.",
	)
}
