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
		tokenType string
		withToken bool
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Store credentials for ywiki",
		Long: `Store a Wiki token and organization ID in the ywiki config file.

The token is read interactively without echoing, or from stdin with
--with-token so it never lands in your shell history. Credentials are verified
against the API before they are saved, and written with 0600 permissions.

Three combinations are supported:

  Yandex 360 for Business    --org-type 360    --token-type oauth
  Yandex Identity Hub        --org-type cloud  --token-type oauth
  Yandex Cloud               --org-type cloud  --token-type iam

Omitted types are detected by trying each combination and keeping the one the
API accepts. IAM tokens expire after 12 hours, so prefer OAuth where you can.`,
		Example: `  # Log in interactively
  ywiki auth login --org-id 1234567

  # Log in from a secret store
  cat token.txt | ywiki auth login --org-id 1234567 --with-token`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogin(cmd, token, orgID, orgType, tokenType, withToken)
		},
	}

	cmd.Flags().StringVar(&token, "token", "", "Token to store (prefer --with-token or the prompt)")
	cmd.Flags().StringVar(&orgID, "org-id", "", "Wiki organization ID (required)")
	cmd.Flags().StringVar(&orgType, "org-type", "", "Organization type: 360 or cloud (detected if omitted)")
	cmd.Flags().StringVar(&tokenType, "token-type", "", "Token type: oauth or iam (detected if omitted)")
	cmd.Flags().BoolVar(&withToken, "with-token", false, "Read the token from stdin")

	return cmd
}

func runLogin(cmd *cobra.Command, token, orgID, orgType, tokenType string, withToken bool) error {
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

	resolved, user, err := verify(cmd.Context(), token, orgID, orgType, tokenType)
	if err != nil {
		return err
	}

	if err := config.Save(&config.Config{
		Token:     resolved.Token,
		OrgID:     resolved.OrgID,
		OrgType:   resolved.OrgType,
		TokenType: resolved.TokenType,
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

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Logged in as %s (org %s, type %s, token %s)\n",
		name, resolved.OrgID, resolved.OrgType, resolved.TokenType)
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

// authCombination is one organization/token pairing the Wiki API accepts.
type authCombination struct {
	orgType   config.OrgType
	tokenType config.TokenType
}

// supportedCombinations lists every pairing documented by the Wiki API, most
// common first so detection usually succeeds on the first request.
var supportedCombinations = []authCombination{
	{config.OrgType360, config.TokenTypeOAuth},
	{config.OrgTypeCloud, config.TokenTypeOAuth},
	{config.OrgTypeCloud, config.TokenTypeIAM},
}

// verify checks the credentials against the API before they are stored, so a
// bad token fails here rather than on some later command. Any type left
// unspecified is detected by trying the matching supported combinations; they
// differ only in headers, so one round trip each tells them apart.
func verify(
	ctx context.Context, token, orgID, orgTypeRaw, tokenTypeRaw string,
) (*config.ResolvedAuth, *api.User, error) {
	var orgType config.OrgType
	if orgTypeRaw != "" {
		parsed, err := config.ParseOrgType(orgTypeRaw)
		if err != nil {
			return nil, nil, wikierrors.NewUserError(err.Error(), "Use --org-type 360 or --org-type cloud")
		}
		orgType = parsed
	}

	tokenType, err := config.ParseTokenType(tokenTypeRaw)
	if err != nil {
		return nil, nil, wikierrors.NewUserError(err.Error(), "Use --token-type oauth or --token-type iam")
	}

	var candidates []authCombination
	for _, combo := range supportedCombinations {
		if orgType != "" && combo.orgType != orgType {
			continue
		}
		if tokenType != "" && combo.tokenType != tokenType {
			continue
		}
		candidates = append(candidates, combo)
	}
	if len(candidates) == 0 {
		return nil, nil, wikierrors.NewUserError(
			fmt.Sprintf("org type %s does not accept %s tokens", orgType, tokenType),
			"Yandex 360 organizations only accept OAuth tokens",
		)
	}

	var lastErr error
	for _, combo := range candidates {
		auth := &config.ResolvedAuth{
			Token:       token,
			OrgID:       orgID,
			OrgType:     combo.orgType,
			TokenType:   combo.tokenType,
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
		"Check the token and organization ID. If you know the organization and token type, "+
			"pass --org-type and --token-type to see the exact API error.",
	)
}
