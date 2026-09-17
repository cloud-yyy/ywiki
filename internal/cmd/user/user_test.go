package user_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/cloud-yyy/ywiki/internal/cmd/user"
	"github.com/cloud-yyy/ywiki/internal/testutil"
)

func TestUserMe(t *testing.T) {
	handler, log := testutil.JSONHandler(http.StatusOK,
		`{"id":7,"username":"me","display_name":"Me","affiliation":"employee"}`)
	testutil.StubAPI(t, handler)

	stdout, _, err := testutil.Execute(t, user.NewCmd, "me")
	if err != nil {
		t.Fatalf("user me returned error: %v", err)
	}

	if got := log.Last().Path; got != "/users/me" {
		t.Errorf("request path = %q, want /users/me", got)
	}
	for _, want := range []string{"Me", "username: me", "id: 7"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout does not contain %q:\n%s", want, stdout)
		}
	}
}

func TestUserMeJQ(t *testing.T) {
	handler, _ := testutil.JSONHandler(http.StatusOK, `{"id":7,"username":"me"}`)
	testutil.StubAPI(t, handler)

	stdout, _, err := testutil.Execute(t, user.NewCmd, "me", "--jq", ".username")
	if err != nil {
		t.Fatalf("user me --jq returned error: %v", err)
	}
	if strings.TrimSpace(stdout) != "me" {
		t.Errorf("stdout = %q, want the username", stdout)
	}
}
