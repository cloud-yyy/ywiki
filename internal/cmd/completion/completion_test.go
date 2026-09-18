package completion_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/cmd/completion"
)

func TestCompletionScripts(t *testing.T) {
	tests := []struct {
		shell string
		want  string
	}{
		{shell: "bash", want: "bash completion"},
		{shell: "zsh", want: "compdef"},
		{shell: "fish", want: "fish completion"},
	}

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			root := &cobra.Command{Use: "ywiki"}
			root.AddCommand(completion.NewCmd(root))

			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&out)
			root.SetArgs([]string{"completion", tt.shell})

			if err := root.Execute(); err != nil {
				t.Fatalf("completion %s returned error: %v", tt.shell, err)
			}
			if !strings.Contains(out.String(), tt.want) {
				t.Errorf("completion %s output does not mention %q", tt.shell, tt.want)
			}
		})
	}
}

func TestCompletionRejectsUnknownShell(t *testing.T) {
	root := &cobra.Command{Use: "ywiki", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(completion.NewCmd(root))

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"completion", "powershell"})

	if err := root.Execute(); err == nil {
		t.Fatal("completion accepted an unsupported shell")
	}
}
