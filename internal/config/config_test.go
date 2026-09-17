package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloud-yyy/ywiki/internal/config"
	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
)

// isolate points the config package at a temporary directory and clears the
// environment tier so each test starts from a known state.
func isolate(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("YWIKI_CONFIG_DIR", dir)
	t.Setenv("YWIKI_TOKEN", "")
	t.Setenv("YWIKI_ORG_ID", "")
	t.Setenv("YWIKI_ORG_TYPE", "")

	return dir
}

func TestSaveAndLoad(t *testing.T) {
	dir := isolate(t)

	want := &config.Config{Token: "tok", OrgID: "org", OrgType: config.OrgType360}
	if err := config.Save(want); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if *got != *want {
		t.Errorf("Load = %+v, want %+v", got, want)
	}

	info, err := os.Stat(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config file mode = %o, want 600", perm)
	}
}

func TestSaveForcesDirectoryPermissions(t *testing.T) {
	dir := isolate(t)
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("Chmod returned error: %v", err)
	}

	if err := config.Save(&config.Config{Token: "tok"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("config dir mode = %o, want 700", perm)
	}
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	isolate(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Token != "" || cfg.OrgID != "" {
		t.Errorf("Load = %+v, want an empty config", cfg)
	}
}

func TestDeleteIsIdempotent(t *testing.T) {
	isolate(t)

	if err := config.Delete(); err != nil {
		t.Errorf("Delete on a missing file returned error: %v", err)
	}

	if err := config.Save(&config.Config{Token: "tok"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if err := config.Delete(); err != nil {
		t.Errorf("Delete returned error: %v", err)
	}
	if err := config.Delete(); err != nil {
		t.Errorf("second Delete returned error: %v", err)
	}
}

func TestResolveAuthPrecedence(t *testing.T) {
	isolate(t)

	if err := config.Save(&config.Config{
		Token: "config-token", OrgID: "config-org", OrgType: config.OrgType360,
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	t.Run("config file is the last resort", func(t *testing.T) {
		auth, err := config.ResolveAuth("", "", "")
		if err != nil {
			t.Fatalf("ResolveAuth returned error: %v", err)
		}
		if auth.Token != "config-token" || auth.TokenSource != "config" {
			t.Errorf("auth = %+v, want the config tier", auth)
		}
	})

	t.Run("environment beats config", func(t *testing.T) {
		t.Setenv("YWIKI_TOKEN", "env-token")
		t.Setenv("YWIKI_ORG_ID", "env-org")
		t.Setenv("YWIKI_ORG_TYPE", "cloud")

		auth, err := config.ResolveAuth("", "", "")
		if err != nil {
			t.Fatalf("ResolveAuth returned error: %v", err)
		}
		if auth.Token != "env-token" || auth.TokenSource != "env" {
			t.Errorf("auth = %+v, want the env tier", auth)
		}
		if auth.OrgType != config.OrgTypeCloud {
			t.Errorf("OrgType = %q, want cloud", auth.OrgType)
		}
	})

	t.Run("flags beat everything", func(t *testing.T) {
		t.Setenv("YWIKI_TOKEN", "env-token")
		t.Setenv("YWIKI_ORG_ID", "env-org")
		t.Setenv("YWIKI_ORG_TYPE", "cloud")

		auth, err := config.ResolveAuth("flag-token", "flag-org", "360")
		if err != nil {
			t.Fatalf("ResolveAuth returned error: %v", err)
		}
		if auth.Token != "flag-token" || auth.TokenSource != "flag" {
			t.Errorf("auth = %+v, want the flag tier", auth)
		}
	})
}

func TestResolveAuthErrors(t *testing.T) {
	tests := []struct {
		name         string
		token        string
		orgID        string
		orgType      string
		wantExitCode int
	}{
		{
			name:         "no credentials anywhere",
			wantExitCode: wikierrors.ExitAuthError,
		},
		{
			name:         "partial flags are rejected rather than ignored",
			token:        "tok",
			wantExitCode: wikierrors.ExitUserError,
		},
		{
			name:         "an invalid org type is rejected",
			token:        "tok",
			orgID:        "org",
			orgType:      "yandex",
			wantExitCode: wikierrors.ExitUserError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolate(t)

			_, err := config.ResolveAuth(tt.token, tt.orgID, tt.orgType)
			if err == nil {
				t.Fatal("ResolveAuth returned nil error, want an error")
			}

			var exitErr *wikierrors.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("error %v is not an *ExitError", err)
			}
			if exitErr.ExitCode != tt.wantExitCode {
				t.Errorf("ExitCode = %d, want %d", exitErr.ExitCode, tt.wantExitCode)
			}
		})
	}
}

func TestResolveAuthRejectsPartialConfig(t *testing.T) {
	isolate(t)

	if err := config.Save(&config.Config{Token: "tok"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	_, err := config.ResolveAuth("", "", "")
	if err == nil {
		t.Fatal("ResolveAuth accepted a partial config")
	}

	var exitErr *wikierrors.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error %v is not an *ExitError", err)
	}
	if exitErr.ExitCode != wikierrors.ExitUserError {
		t.Errorf("ExitCode = %d, want %d", exitErr.ExitCode, wikierrors.ExitUserError)
	}
}

func TestOrgTypeHeaders(t *testing.T) {
	tests := []struct {
		orgType    config.OrgType
		wantScheme string
		wantHeader string
	}{
		{config.OrgType360, "OAuth", "X-Org-Id"},
		{config.OrgTypeCloud, "Bearer", "X-Cloud-Org-Id"},
	}

	for _, tt := range tests {
		t.Run(string(tt.orgType), func(t *testing.T) {
			if got := tt.orgType.AuthScheme(); got != tt.wantScheme {
				t.Errorf("AuthScheme = %q, want %q", got, tt.wantScheme)
			}
			if got := tt.orgType.OrgHeader(); got != tt.wantHeader {
				t.Errorf("OrgHeader = %q, want %q", got, tt.wantHeader)
			}
		})
	}
}
