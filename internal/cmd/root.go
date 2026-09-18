// Package cmd provides the root Cobra command and command tree for the ywiki CLI.
package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/cmd/attachment"
	"github.com/cloud-yyy/ywiki/internal/cmd/auth"
	"github.com/cloud-yyy/ywiki/internal/cmd/comment"
	"github.com/cloud-yyy/ywiki/internal/cmd/completion"
	"github.com/cloud-yyy/ywiki/internal/cmd/jsonfields"
	"github.com/cloud-yyy/ywiki/internal/cmd/page"
	"github.com/cloud-yyy/ywiki/internal/cmd/search"
	"github.com/cloud-yyy/ywiki/internal/cmd/user"
	versioncmd "github.com/cloud-yyy/ywiki/internal/cmd/version"
	"github.com/cloud-yyy/ywiki/internal/output"
)

// Command group IDs.
const (
	groupContent = "content"
	groupAccount = "account"
	groupSystem  = "system"
)

var rootCmd = &cobra.Command{
	Use:           "ywiki",
	Short:         "Yandex Wiki CLI",
	Long:          "Command-line client for Yandex Wiki. Designed for LLM agents and human developers.",
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	rootCmd.PersistentFlags().
		StringSliceVar(&output.JSONFields, "json", nil, "Output JSON with selected fields (comma-separated)")
	rootCmd.PersistentFlags().
		StringVar(&output.JQFilter, "jq", "", "Filter JSON output with a jq expression (implies --json)")
	rootCmd.PersistentFlags().
		BoolVar(&output.QuietFlag, "quiet", false, "Output minimal text, one item per line")
	rootCmd.PersistentFlags().
		BoolVar(&output.DebugFlag, "debug", false, "Emit sanitized debug diagnostics to stderr")

	// Global auth flags allow a per-invocation override on every command.
	rootCmd.PersistentFlags().String("token", "", "OAuth or IAM token (use with --org-id and --org-type)")
	rootCmd.PersistentFlags().String("org-id", "", "Wiki organization ID (use with --token and --org-type)")
	rootCmd.PersistentFlags().
		String("org-type", "", "Organization type, 360 or cloud (use with --token and --org-id)")
	rootCmd.PersistentFlags().
		String("token-type", "", "Token type, oauth or iam (default: oauth for 360, iam for cloud)")

	rootCmd.MarkFlagsMutuallyExclusive("json", "quiet")
	rootCmd.MarkFlagsMutuallyExclusive("jq", "quiet")

	rootCmd.AddGroup(
		&cobra.Group{ID: groupContent, Title: "Content:"},
		&cobra.Group{ID: groupAccount, Title: "Account:"},
		&cobra.Group{ID: groupSystem, Title: "System:"},
	)
	rootCmd.SetHelpCommandGroupID(groupSystem)

	registerSubcommands()

	_ = rootCmd.RegisterFlagCompletionFunc("json",
		func(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			if fields, ok := jsonfields.Get(cmd.CommandPath()); ok {
				return fields, cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
	)
}

func addGroupedCommand(cmd *cobra.Command, groupID string) {
	cmd.GroupID = groupID
	rootCmd.AddCommand(cmd)
}

func registerSubcommands() {
	addGroupedCommand(page.NewCmd(), groupContent)
	addGroupedCommand(search.NewCmd(), groupContent)
	addGroupedCommand(comment.NewCmd(), groupContent)
	addGroupedCommand(attachment.NewCmd(), groupContent)

	addGroupedCommand(user.NewCmd(), groupAccount)
	addGroupedCommand(auth.NewCmd(), groupAccount)

	addGroupedCommand(versioncmd.NewCmd(), groupSystem)
	addGroupedCommand(completion.NewCmd(rootCmd), groupSystem)
}

// Execute runs the root command and returns the process exit code.
// Interrupt and termination signals cancel the command context so an in-flight
// request is dropped instead of leaving the terminal hanging.
func Execute() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := rootCmd.ExecuteContext(ctx)

	return output.HandleError(os.Stderr, err)
}

// RootCmd returns the root command for testing purposes.
func RootCmd() *cobra.Command {
	return rootCmd
}
