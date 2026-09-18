package version_test

import (
	"encoding/json"
	"strings"
	"testing"

	versioncmd "github.com/cloud-yyy/ywiki/internal/cmd/version"
	"github.com/cloud-yyy/ywiki/internal/testutil"
)

func TestVersionHuman(t *testing.T) {
	testutil.ResetOutputFlags(t)

	stdout, _, err := testutil.Execute(t, versioncmd.NewCmd)
	if err != nil {
		t.Fatalf("version returned error: %v", err)
	}

	for _, want := range []string{"ywiki version", "commit:", "os/arch:"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout does not contain %q:\n%s", want, stdout)
		}
	}
}

func TestVersionJSON(t *testing.T) {
	testutil.ResetOutputFlags(t)

	stdout, _, err := testutil.Execute(t, versioncmd.NewCmd, "--json", "version,os")
	if err != nil {
		t.Fatalf("version --json returned error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if len(got) != 2 {
		t.Errorf("got %d fields, want only the 2 requested: %v", len(got), got)
	}
}

func TestVersionFieldHint(t *testing.T) {
	testutil.ResetOutputFlags(t)

	_, stderr, err := testutil.Execute(t, versioncmd.NewCmd, "--json=")
	if err == nil {
		t.Fatal("version accepted --json with no fields")
	}
	if !strings.Contains(stderr, "Available fields") {
		t.Errorf("stderr = %q, want the available-fields hint", stderr)
	}
}
