package search_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/cloud-yyy/ywiki/internal/cmd/search"
	"github.com/cloud-yyy/ywiki/internal/testutil"
)

const resultsJSON = `{"results":[
	{"url":"https://wiki.yandex.ru/a","slug":"a","title":"Alpha","content":"alpha snippet",
	 "type":"page","modified_at":"2026-01-02T03:04:05Z"},
	{"url":"https://wiki.yandex.ru/b","slug":"b","title":"Beta","content":"beta snippet",
	 "type":"page","modified_at":"2026-01-03T03:04:05Z"}
],"next_cursor":"2"}`

func TestSearchSendsQuery(t *testing.T) {
	handler, log := testutil.JSONHandler(http.StatusOK, resultsJSON)
	testutil.StubAPI(t, handler)

	stdout, _, err := testutil.Execute(t, search.NewCmd, "release checklist", "--limit", "25")
	if err != nil {
		t.Fatalf("search returned error: %v", err)
	}

	last := log.Last()
	if last.Method != http.MethodPost || last.Path != "/search" {
		t.Errorf("request = %s %s, want POST /search", last.Method, last.Path)
	}

	var sent map[string]any
	if err := json.Unmarshal([]byte(last.Body), &sent); err != nil {
		t.Fatalf("request body is not JSON: %v", err)
	}
	if sent["query"] != "release checklist" {
		t.Errorf("query = %v, want the search term", sent["query"])
	}
	if sent["limit"] != float64(25) {
		t.Errorf("limit = %v, want 25", sent["limit"])
	}

	if !strings.Contains(stdout, "Alpha") || !strings.Contains(stdout, "Beta") {
		t.Errorf("stdout does not list both hits:\n%s", stdout)
	}
}

func TestSearchQuietListsSlugs(t *testing.T) {
	handler, _ := testutil.JSONHandler(http.StatusOK, resultsJSON)
	testutil.StubAPI(t, handler)

	stdout, _, err := testutil.Execute(t, search.NewCmd, "anything", "--quiet")
	if err != nil {
		t.Fatalf("search --quiet returned error: %v", err)
	}
	if stdout != "a\nb\n" {
		t.Errorf("stdout = %q, want one slug per line", stdout)
	}
}

func TestSearchJSONFields(t *testing.T) {
	handler, _ := testutil.JSONHandler(http.StatusOK, resultsJSON)
	testutil.StubAPI(t, handler)

	stdout, _, err := testutil.Execute(t, search.NewCmd, "anything", "--json", "slug,title")
	if err != nil {
		t.Fatalf("search --json returned error: %v", err)
	}

	var got []map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not a JSON array: %v\n%s", err, stdout)
	}
	if len(got) != 2 {
		t.Fatalf("got %d results, want 2", len(got))
	}
	if len(got[0]) != 2 {
		t.Errorf("result has %d fields, want only the 2 requested: %v", len(got[0]), got[0])
	}
}

func TestSearchNoResults(t *testing.T) {
	handler, _ := testutil.JSONHandler(http.StatusOK, `{"results":[],"next_cursor":""}`)
	testutil.StubAPI(t, handler)

	stdout, _, err := testutil.Execute(t, search.NewCmd, "nothing")
	if err != nil {
		t.Fatalf("search returned error: %v", err)
	}
	if !strings.Contains(stdout, "No results found") {
		t.Errorf("stdout = %q, want the empty-result message", stdout)
	}
}
