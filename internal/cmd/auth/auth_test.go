package auth_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloud-yyy/ywiki/internal/cmd/auth"
	"github.com/cloud-yyy/ywiki/internal/testutil"
)

const userJSON = `{"id":7,"username":"me","display_name":"Me","affiliation":"employee"}`

// clearEnvAuth removes the environment credentials StubAPI sets, so login and
// logout tests exercise the config-file tier.
func clearEnvAuth(t *testing.T) {
	t.Helper()
	t.Setenv("YWIKI_TOKEN", "")
	t.Setenv("YWIKI_ORG_ID", "")
	t.Setenv("YWIKI_ORG_TYPE", "")
}

func TestLoginStoresVerifiedCredentials(t *testing.T) {
	handler, log := testutil.JSONHandler(http.StatusOK, userJSON)
	testutil.StubAPI(t, handler)
	clearEnvAuth(t)

	stdout, _, err := testutil.ExecuteWithStdin(t, auth.NewCmd, "my-token\n",
		"login", "--org-id", "org-1", "--org-type", "360", "--with-token")
	if err != nil {
		t.Fatalf("auth login returned error: %v", err)
	}
	if !strings.Contains(stdout, "Logged in as Me") {
		t.Errorf("stdout = %q, want the resolved display name", stdout)
	}

	if got := log.Last().Header.Get("Authorization"); got != "OAuth my-token" {
		t.Errorf("Authorization = %q, want the piped token", got)
	}

	data, err := os.ReadFile(filepath.Join(os.Getenv("YWIKI_CONFIG_DIR"), "config.yaml"))
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	for _, want := range []string{"my-token", "org-1", "360"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("config does not contain %q:\n%s", want, data)
		}
	}
}

func TestLoginDetectsOrgType(t *testing.T) {
	// The 360 attempt is rejected, so login must fall through to cloud.
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Cloud-Org-Id") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error_code":"UNAUTHORIZED","debug_message":"nope"}`))
			return
		}
		_, _ = w.Write([]byte(userJSON))
	}
	testutil.StubAPI(t, handler)
	clearEnvAuth(t)

	stdout, _, err := testutil.ExecuteWithStdin(t, auth.NewCmd, "iam-token\n",
		"login", "--org-id", "org-1", "--with-token")
	if err != nil {
		t.Fatalf("auth login returned error: %v", err)
	}
	if !strings.Contains(stdout, "type cloud") {
		t.Errorf("stdout = %q, want the detected cloud org type", stdout)
	}
}

func TestLoginRejectsBadCredentials(t *testing.T) {
	handler, _ := testutil.JSONHandler(http.StatusUnauthorized,
		`{"error_code":"UNAUTHORIZED","debug_message":"Token is invalid"}`)
	testutil.StubAPI(t, handler)
	clearEnvAuth(t)

	_, _, err := testutil.ExecuteWithStdin(t, auth.NewCmd, "bad-token\n",
		"login", "--org-id", "org-1", "--org-type", "360", "--with-token")
	if err == nil {
		t.Fatal("auth login stored credentials the API rejected")
	}

	if _, statErr := os.Stat(
		filepath.Join(os.Getenv("YWIKI_CONFIG_DIR"), "config.yaml"),
	); !os.IsNotExist(statErr) {
		t.Error("auth login wrote a config file despite failing verification")
	}
}

func TestLoginRequiresOrgID(t *testing.T) {
	handler, _ := testutil.JSONHandler(http.StatusOK, userJSON)
	testutil.StubAPI(t, handler)
	clearEnvAuth(t)

	_, _, err := testutil.ExecuteWithStdin(t, auth.NewCmd, "tok\n", "login", "--with-token")
	if err == nil {
		t.Fatal("auth login ran without an organization ID")
	}
	if !strings.Contains(err.Error(), "organization ID") {
		t.Errorf("error = %q, want it to ask for the organization ID", err)
	}
}

func TestStatusReportsValidCredentials(t *testing.T) {
	handler, _ := testutil.JSONHandler(http.StatusOK, userJSON)
	testutil.StubAPI(t, handler)

	stdout, _, err := testutil.Execute(t, auth.NewCmd, "status")
	if err != nil {
		t.Fatalf("auth status returned error: %v", err)
	}

	for _, want := range []string{"Logged in as Me", "test-org", "token source: env"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout does not contain %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "test-token") {
		t.Error("auth status printed the token itself")
	}
}

func TestStatusReportsRejectedCredentials(t *testing.T) {
	handler, _ := testutil.JSONHandler(http.StatusUnauthorized,
		`{"error_code":"UNAUTHORIZED","debug_message":"Token is invalid"}`)
	testutil.StubAPI(t, handler)

	stdout, _, err := testutil.Execute(t, auth.NewCmd, "status")
	if err == nil {
		t.Fatal("auth status returned nil error for rejected credentials")
	}
	if !strings.Contains(stdout, "Not logged in") {
		t.Errorf("stdout = %q, want it to report the rejection", stdout)
	}
}

func TestLogoutRemovesConfig(t *testing.T) {
	handler, _ := testutil.JSONHandler(http.StatusOK, userJSON)
	testutil.StubAPI(t, handler)
	clearEnvAuth(t)

	path := filepath.Join(os.Getenv("YWIKI_CONFIG_DIR"), "config.yaml")
	if err := os.WriteFile(path, []byte("token: t\norg_id: o\norg_type: \"360\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	if _, _, err := testutil.Execute(t, auth.NewCmd, "logout"); err != nil {
		t.Fatalf("auth logout returned error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("auth logout left the config file in place")
	}
}
