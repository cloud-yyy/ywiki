// Package version provides the ywiki version command.
package version

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	"github.com/cloud-yyy/ywiki/internal/cmd/jsonfields"
	"github.com/cloud-yyy/ywiki/internal/output"
	ver "github.com/cloud-yyy/ywiki/internal/version"
)

// VersionFields lists the available JSON field names for version output.
var VersionFields = []string{"version", "commit", "date", "goVersion", "os", "arch"}

// NewCmd creates the version command that displays build information.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show ywiki version information",
		Long: `Display the version, commit, build date, Go version, and platform of the ywiki binary.

JSON FIELDS
  version, commit, date, goVersion, os, arch`,
		Example: `  # Show version
  ywiki version

  # Get version as JSON
  ywiki version --json version,commit

  # Get just the version string
  ywiki version --json version --jq '.version'`,
		Args: cobra.NoArgs,
		RunE: runVersion,
	}

	jsonfields.Register("ywiki version", VersionFields)

	return cmd
}

func runVersion(cmd *cobra.Command, _ []string) error {
	if err := cmdutil.PrepareFields(cmd, "version", VersionFields); err != nil {
		return err
	}

	info := ver.Get()
	w := cmd.OutOrStdout()

	if output.IsJSON() {
		return cmdutil.RenderJSON(w, info)
	}

	if output.IsQuiet() {
		output.PrintQuiet(w, info.Version)
		return nil
	}

	_, _ = fmt.Fprintf(w, "ywiki version %s\n", info.Version)
	_, _ = fmt.Fprintf(w, "commit: %s\n", info.Commit)
	_, _ = fmt.Fprintf(w, "date: %s\n", info.Date)
	_, _ = fmt.Fprintf(w, "go: %s\n", info.GoVersion)
	_, _ = fmt.Fprintf(w, "os/arch: %s/%s\n", info.OS, info.Arch)

	return nil
}
