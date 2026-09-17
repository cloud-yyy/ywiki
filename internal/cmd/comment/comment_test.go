package comment_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloud-yyy/ywiki/internal/cmd/comment"
	"github.com/cloud-yyy/ywiki/internal/testutil"
)

const commentsJSON = `{"results":[
	{"id":1,"body":"first","author":{"username":"me","display_name":"Me"},
	 "created_at":"2026-01-02T03:04:05Z"},
	{"id":2,"body":"second","author":{"username":"you","display_name":"You"},
	 "created_at":"2026-01-02T04:04:05Z"}
],"next_cursor":""}`

func TestCommentList(t *testing.T) {
	handler, log := testutil.RouteHandler(map[string]string{
		"/pages/12345/comments": commentsJSON,
		"/pages":                `{"id":12345,"slug":"users/me/notes","title":"Notes"}`,
	})
	testutil.StubAPI(t, handler)

	stdout, _, err := testutil.Execute(t, comment.NewCmd, "list", "users/me/notes")
	if err != nil {
		t.Fatalf("comment list returned error: %v", err)
	}

	for _, want := range []string{"first", "second", "Me", "You"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout does not contain %q:\n%s", want, stdout)
		}
	}
	if log.At(0).Path != "/pages" {
		t.Errorf("first request = %q, want the slug lookup", log.At(0).Path)
	}
}

func TestCommentListThread(t *testing.T) {
	handler, log := testutil.RouteHandler(map[string]string{
		"/pages/12345/comments/7/thread": commentsJSON,
		"/pages/12345":                   `{"id":12345,"slug":"s","title":"t"}`,
	})
	testutil.StubAPI(t, handler)

	_, _, err := testutil.Execute(t, comment.NewCmd, "list", "12345", "--thread", "7")
	if err != nil {
		t.Fatalf("comment list --thread returned error: %v", err)
	}

	if got := log.Last().Path; got != "/pages/12345/comments/7/thread" {
		t.Errorf("request path = %q, want the thread endpoint", got)
	}
}

func TestCommentCreateFromStdin(t *testing.T) {
	handler, log := testutil.RouteHandler(map[string]string{
		"/pages/12345/comments": `{"id":9,"body":"looks good"}`,
		"/pages":                `{"id":12345,"slug":"users/me/notes"}`,
	})
	testutil.StubAPI(t, handler)

	stdout, _, err := testutil.ExecuteWithStdin(t, comment.NewCmd, "looks good\n",
		"create", "12345", "--body-file", "-")
	if err != nil {
		t.Fatalf("comment create returned error: %v", err)
	}

	var sent map[string]any
	if err := json.Unmarshal([]byte(log.Last().Body), &sent); err != nil {
		t.Fatalf("request body is not JSON: %v", err)
	}
	if sent["body"] != "looks good" {
		t.Errorf("body = %v, want the piped text", sent["body"])
	}
	if !strings.Contains(stdout, "Added comment 9") {
		t.Errorf("stdout = %q, want the new comment ID", stdout)
	}
}

func TestCommentCreateRejectsEmptyBody(t *testing.T) {
	handler, _ := testutil.RouteHandler(map[string]string{"/pages": `{"id":1}`})
	testutil.StubAPI(t, handler)

	_, _, err := testutil.Execute(t, comment.NewCmd, "create", "12345")
	if err == nil {
		t.Fatal("comment create accepted an empty comment")
	}
}

func TestCommentCreateReplyTo(t *testing.T) {
	handler, log := testutil.RouteHandler(map[string]string{
		"/pages/12345/comments": `{"id":9,"body":"agreed","parent_id":42}`,
		"/pages":                `{"id":12345}`,
	})
	testutil.StubAPI(t, handler)

	_, _, err := testutil.Execute(t, comment.NewCmd,
		"create", "12345", "--body", "agreed", "--reply-to", "42")
	if err != nil {
		t.Fatalf("comment create --reply-to returned error: %v", err)
	}

	var sent map[string]any
	if err := json.Unmarshal([]byte(log.Last().Body), &sent); err != nil {
		t.Fatalf("request body is not JSON: %v", err)
	}
	if sent["parent_id"] != float64(42) {
		t.Errorf("parent_id = %v, want 42", sent["parent_id"])
	}
}

func TestCommentDeleteRejectsBadID(t *testing.T) {
	handler, _ := testutil.RouteHandler(map[string]string{"/pages": `{"id":1}`})
	testutil.StubAPI(t, handler)

	_, _, err := testutil.Execute(t, comment.NewCmd, "delete", "12345", "abc")
	if err == nil {
		t.Fatal("comment delete accepted a non-numeric comment ID")
	}
	if !strings.Contains(err.Error(), "invalid comment ID") {
		t.Errorf("error = %q, want it to name the bad ID", err)
	}
}
