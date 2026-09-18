// Package completion provides shell completion script generation for ywiki.
package completion

import (
	"github.com/spf13/cobra"

	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
)

// NewCmd creates the "completion" command with bash, zsh, and fish subcommands.
// rootCmd is needed to generate completions for the full command tree.
func NewCmd(rootCmd *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for ywiki.

To load completions:

Bash:
  source <(ywiki completion bash)

  # To load completions for each session, execute once:
  # Linux:
  ywiki completion bash > /etc/bash_completion.d/ywiki
  # macOS:
  ywiki completion bash > $(brew --prefix)/etc/bash_completion.d/ywiki

Zsh:
  source <(ywiki completion zsh)

  # To load completions for each session, execute once:
  ywiki completion zsh > "${fpath[1]}/_ytr"

Fish:
  ywiki completion fish | source

  # To load completions for each session, execute once:
  ywiki completion fish > ~/.config/fish/completions/ywiki.fish

SEE ALSO
  ywiki --help    - Show all available commands`,
		// Without this, cobra prints help and exits 0 for an unsupported
		// shell, which reads as success in a script that pipes the output
		// into a completion file.
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}

			return wikierrors.NewUserError(
				"unsupported shell: "+args[0],
				"Supported shells: bash, zsh, fish",
			)
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "bash",
		Short: "Generate bash completion script",
		Long:  "Generate bash completion script for ywiki. Output to stdout.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return rootCmd.GenBashCompletionV2(cmd.OutOrStdout(), true)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "zsh",
		Short: "Generate zsh completion script",
		Long:  "Generate zsh completion script for ywiki. Output to stdout.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return rootCmd.GenZshCompletion(cmd.OutOrStdout())
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "fish",
		Short: "Generate fish completion script",
		Long:  "Generate fish completion script for ywiki. Output to stdout.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return rootCmd.GenFishCompletion(cmd.OutOrStdout(), true)
		},
	})

	return cmd
}
