package page_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/cloud-yyy/ywiki/internal/cmd/page"
	"github.com/cloud-yyy/ywiki/internal/testutil"
)

const pageJSON = `{
	"id": 12345,
	"slug": "users/me/notes",
	"title": "Notes",
	"page_type": "page",
	"content": "line one\nline two",
	"attributes": {"modified_at": "2026-01-02T03:04:05Z"},
	"owner": {"user": {"id": 7, "username": "me", "display_name": "Me"}}
}`

func TestPageGetHumanOutput(t *testing.T) {
	handler, log := testutil.JSONHandler(http.StatusOK, pageJSON)
	testutil.StubAPI(t, handler)

	stdout, _, err := testutil.Execute(t, page.NewCmd, "get", "users/me/notes")
	if err != nil {
		t.Fatalf("page get returned error: %v", err)
	}

	for _, want := range []string{"Notes", "slug: users/me/notes", "id: 12345", "line one"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout does not contain %q:\n%s", want, stdout)
		}
	}

	if got := log.Last().Path; got != "/pages" {
		t.Errorf("request path = %q, want /pages", got)
	}
}

func TestPageGetContentOnly(t *testing.T) {
	handler, _ := testutil.JSONHandler(http.StatusOK, pageJSON)
	testutil.StubAPI(t, handler)

	stdout, _, err := testutil.Execute(t, page.NewCmd, "get", "12345", "--content")
	if err != nil {
		t.Fatalf("page get --content returned error: %v", err)
	}

	if stdout != "line one\nline two\n" {
		t.Errorf("stdout = %q, want the raw body only", stdout)
	}
}

func TestPageGetJSONFields(t *testing.T) {
	handler, _ := testutil.JSONHandler(http.StatusOK, pageJSON)
	testutil.StubAPI(t, handler)

	stdout, _, err := testutil.Execute(t, page.NewCmd, "get", "12345", "--json", "id,title")
	if err != nil {
		t.Fatalf("page get --json returned error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if len(got) != 2 {
		t.Errorf("got %d fields, want only the 2 requested: %v", len(got), got)
	}
	if got["title"] != "Notes" {
		t.Errorf("title = %v, want Notes", got["title"])
	}
}

func TestPageGetRejectsUnknownField(t *testing.T) {
	handler, _ := testutil.JSONHandler(http.StatusOK, pageJSON)
	testutil.StubAPI(t, handler)

	_, _, err := testutil.Execute(t, page.NewCmd, "get", "12345", "--json", "nope")
	if err == nil {
		t.Fatal("page get accepted an unknown --json field")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error %q does not name the bad field", err)
	}
}

func TestPageCreateSendsBody(t *testing.T) {
	handler, log := testutil.JSONHandler(http.StatusOK, pageJSON)
	testutil.StubAPI(t, handler)

	_, _, err := testutil.ExecuteWithStdin(t, page.NewCmd, "piped content\n",
		"create", "users/me/notes", "--title", "Notes", "--body-file", "-")
	if err != nil {
		t.Fatalf("page create returned error: %v", err)
	}

	var sent map[string]any
	if err := json.Unmarshal([]byte(log.Last().Body), &sent); err != nil {
		t.Fatalf("request body is not JSON: %v", err)
	}
	if sent["content"] != "piped content" {
		t.Errorf("content = %v, want the piped text with the trailing newline trimmed", sent["content"])
	}
	if sent["slug"] != "users/me/notes" || sent["title"] != "Notes" {
		t.Errorf("request body = %v, want the slug and title from the flags", sent)
	}
}

func TestPageEditRequiresAChange(t *testing.T) {
	handler, _ := testutil.JSONHandler(http.StatusOK, pageJSON)
	testutil.StubAPI(t, handler)

	_, _, err := testutil.Execute(t, page.NewCmd, "edit", "12345")
	if err == nil {
		t.Fatal("page edit ran with nothing to change")
	}
	if !strings.Contains(err.Error(), "nothing to update") {
		t.Errorf("error = %q, want it to say there is nothing to update", err)
	}
}

func TestPageEditResolvesSlugToID(t *testing.T) {
	handler, log := testutil.JSONHandler(http.StatusOK, pageJSON)
	testutil.StubAPI(t, handler)

	_, _, err := testutil.Execute(t, page.NewCmd,
		"edit", "users/me/notes", "--title", "Renamed")
	if err != nil {
		t.Fatalf("page edit returned error: %v", err)
	}

	if log.Len() != 2 {
		t.Fatalf("got %d requests, want a slug lookup followed by the update", log.Len())
	}
	if got := log.At(0).Path; got != "/pages" {
		t.Errorf("lookup path = %q, want /pages", got)
	}
	if got := log.At(1).Path; got != "/pages/12345" {
		t.Errorf("update path = %q, want /pages/12345", got)
	}
}

func TestPageAppendRequiresContent(t *testing.T) {
	handler, _ := testutil.JSONHandler(http.StatusOK, pageJSON)
	testutil.StubAPI(t, handler)

	_, _, err := testutil.Execute(t, page.NewCmd, "append", "12345")
	if err == nil {
		t.Fatal("page append ran with no content")
	}
}

func TestPageAppendSendsLocation(t *testing.T) {
	handler, log := testutil.JSONHandler(http.StatusOK, pageJSON)
	testutil.StubAPI(t, handler)

	_, _, err := testutil.Execute(t, page.NewCmd,
		"append", "12345", "--body", "new line", "--location", "top")
	if err != nil {
		t.Fatalf("page append returned error: %v", err)
	}

	var sent struct {
		Content string `json:"content"`
		Body    struct {
			Location string `json:"location"`
		} `json:"body"`
	}
	if err := json.Unmarshal([]byte(log.Last().Body), &sent); err != nil {
		t.Fatalf("request body is not JSON: %v", err)
	}
	if sent.Content != "new line" || sent.Body.Location != "top" {
		t.Errorf("request body = %+v, want the content appended at the top", sent)
	}
}

func TestPageDeleteRefusesWithoutConfirmation(t *testing.T) {
	handler, log := testutil.JSONHandler(http.StatusOK, pageJSON)
	testutil.StubAPI(t, handler)

	_, _, err := testutil.Execute(t, page.NewCmd, "delete", "12345")
	if err == nil {
		t.Fatal("page delete ran unattended without --yes")
	}

	for i := range log.Len() {
		if log.At(i).Method == http.MethodDelete {
			t.Fatal("page delete issued a DELETE without confirmation")
		}
	}
}

func TestPageDeleteWithYes(t *testing.T) {
	handler, log := testutil.JSONHandler(http.StatusOK, pageJSON)
	testutil.StubAPI(t, handler)

	stdout, _, err := testutil.Execute(t, page.NewCmd, "delete", "12345", "--yes")
	if err != nil {
		t.Fatalf("page delete --yes returned error: %v", err)
	}
	if !strings.Contains(stdout, "Deleted users/me/notes") {
		t.Errorf("stdout = %q, want a deletion confirmation", stdout)
	}
	if last := log.Last(); last.Method != http.MethodDelete {
		t.Errorf("last request = %s %s, want a DELETE", last.Method, last.Path)
	}
}

func TestPageListQuiet(t *testing.T) {
	handler, _ := testutil.JSONHandler(http.StatusOK,
		`{"results":[{"id":1,"slug":"a"},{"id":2,"slug":"b"}],"next_cursor":""}`)
	testutil.StubAPI(t, handler)

	stdout, _, err := testutil.Execute(t, page.NewCmd, "list", "users/me", "--quiet")
	if err != nil {
		t.Fatalf("page list --quiet returned error: %v", err)
	}
	if stdout != "a\nb\n" {
		t.Errorf("stdout = %q, want one slug per line", stdout)
	}
}

func TestPageListJSONCarriesCursor(t *testing.T) {
	handler, _ := testutil.JSONHandler(http.StatusOK,
		`{"results":[{"id":1,"slug":"a"}],"next_cursor":"next-page"}`)
	testutil.StubAPI(t, handler)

	stdout, _, err := testutil.Execute(t, page.NewCmd, "list", "users/me", "--jq", ".pagination.cursor")
	if err != nil {
		t.Fatalf("page list --jq returned error: %v", err)
	}
	if strings.TrimSpace(stdout) != "next-page" {
		t.Errorf("stdout = %q, want the next cursor", stdout)
	}
}

func TestPageGetNotFound(t *testing.T) {
	handler, _ := testutil.JSONHandler(http.StatusNotFound,
		`{"error_code":"PAGE_NOT_FOUND","debug_message":"Page not found"}`)
	testutil.StubAPI(t, handler)

	_, _, err := testutil.Execute(t, page.NewCmd, "get", "users/me/missing")
	if err == nil {
		t.Fatal("page get returned nil error for a missing page")
	}
	if !strings.Contains(err.Error(), "Page not found") {
		t.Errorf("error = %q, want the API message", err)
	}
}
