package cmd_test

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/cmd"
)

func TestRootRegistersEveryCommand(t *testing.T) {
	want := []string{
		"page", "search", "comment", "attachment",
		"user", "auth", "version", "completion",
	}

	got := make(map[string]bool)
	for _, sub := range cmd.RootCmd().Commands() {
		got[sub.Name()] = true
	}

	for _, name := range want {
		if !got[name] {
			t.Errorf("root command is missing %q", name)
		}
	}
}

func TestRootDefinesGlobalFlags(t *testing.T) {
	flags := cmd.RootCmd().PersistentFlags()

	for _, name := range []string{"json", "jq", "quiet", "debug", "token", "org-id", "org-type"} {
		if flags.Lookup(name) == nil {
			t.Errorf("root command is missing the --%s flag", name)
		}
	}
}

// TestEveryCommandIsDocumented guards the help output an agent reads to
// discover what the CLI can do: a command with no description is invisible.
func TestEveryCommandIsDocumented(t *testing.T) {
	var check func(c *cobra.Command)
	check = func(c *cobra.Command) {
		for _, sub := range c.Commands() {
			if sub.Name() == "help" {
				continue
			}
			if sub.Short == "" {
				t.Errorf("%q has no short description", sub.CommandPath())
			}
			if strings.ToLower(sub.Name()) != sub.Name() {
				t.Errorf("%q is not lowercase", sub.CommandPath())
			}
			check(sub)
		}
	}

	check(cmd.RootCmd())
}
