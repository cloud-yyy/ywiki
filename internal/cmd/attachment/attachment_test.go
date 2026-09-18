package attachment_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloud-yyy/ywiki/internal/cmd/attachment"
	"github.com/cloud-yyy/ywiki/internal/testutil"
)

const attachmentsJSON = `{"results":[
	{"id":987,"name":"report.pdf","size":"1024","mimetype":"application/pdf",
	 "created_at":"2026-01-02T03:04:05Z","user":{"username":"me","display_name":"Me"}}
],"next_cursor":""}`

func TestAttachmentList(t *testing.T) {
	handler, _ := testutil.RouteHandler(map[string]string{
		"/pages/12345/attachments": attachmentsJSON,
		"/pages":                   `{"id":12345,"slug":"users/me/notes"}`,
	})
	testutil.StubAPI(t, handler)

	stdout, _, err := testutil.Execute(t, attachment.NewCmd, "list", "12345")
	if err != nil {
		t.Fatalf("attachment list returned error: %v", err)
	}
	if !strings.Contains(stdout, "report.pdf") || !strings.Contains(stdout, "987") {
		t.Errorf("stdout does not list the attachment:\n%s", stdout)
	}
}

func TestAttachmentUploadRunsTheWholeFlow(t *testing.T) {
	handler, log := testutil.RouteHandler(map[string]string{
		"/upload_sessions":         `{"session_id":"sess-1","file_name":"a.txt","file_size":5}`,
		"/pages/12345/attachments": attachmentsJSON,
		"/pages":                   `{"id":12345,"slug":"users/me/notes"}`,
	})
	testutil.StubAPI(t, handler)

	path := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	stdout, _, err := testutil.Execute(t, attachment.NewCmd, "upload", "12345", path)
	if err != nil {
		t.Fatalf("attachment upload returned error: %v", err)
	}
	if !strings.Contains(stdout, "Attached a.txt") {
		t.Errorf("stdout = %q, want an upload confirmation", stdout)
	}

	var sawCreate, sawPart, sawFinish, sawAttach bool
	for i := range log.Len() {
		req := log.At(i)
		switch {
		case req.Method == http.MethodPost && req.Path == "/upload_sessions":
			sawCreate = true
			var sent map[string]any
			if err := json.Unmarshal([]byte(req.Body), &sent); err == nil {
				if sent["file_size"] != float64(5) {
					t.Errorf("file_size = %v, want 5", sent["file_size"])
				}
			}
		case req.Method == http.MethodPut && strings.HasSuffix(req.Path, "/upload_part"):
			sawPart = true
			if req.Body != "hello" {
				t.Errorf("uploaded part = %q, want the file contents", req.Body)
			}
			if req.Query != "part_number=1" {
				t.Errorf("part query = %q, want part_number=1", req.Query)
			}
		case strings.HasSuffix(req.Path, "/finish"):
			sawFinish = true
		case req.Method == http.MethodPost && strings.HasSuffix(req.Path, "/attachments"):
			sawAttach = true
		}
	}

	if !sawCreate || !sawPart || !sawFinish || !sawAttach {
		t.Errorf(
			"upload steps: create=%v part=%v finish=%v attach=%v, want all four",
			sawCreate, sawPart, sawFinish, sawAttach,
		)
	}
}

func TestAttachmentUploadRejectsMissingFile(t *testing.T) {
	handler, _ := testutil.RouteHandler(map[string]string{"/pages": `{"id":1}`})
	testutil.StubAPI(t, handler)

	_, _, err := testutil.Execute(t, attachment.NewCmd,
		"upload", "12345", filepath.Join(t.TempDir(), "absent.txt"))
	if err == nil {
		t.Fatal("attachment upload accepted a missing file")
	}
}

func TestAttachmentDownloadToFile(t *testing.T) {
	handler, _ := testutil.RouteHandler(map[string]string{
		"/pages/12345/attachments/987/download": "file-bytes",
		"/pages":                                `{"id":12345}`,
	})
	testutil.StubAPI(t, handler)

	dst := filepath.Join(t.TempDir(), "out.bin")
	_, _, err := testutil.Execute(t, attachment.NewCmd, "download", "12345", "987", "--out", dst)
	if err != nil {
		t.Fatalf("attachment download returned error: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(data) != "file-bytes" {
		t.Errorf("downloaded file = %q, want the response body", data)
	}
}

func TestAttachmentDownloadToStdout(t *testing.T) {
	handler, _ := testutil.RouteHandler(map[string]string{
		"/pages/12345/attachments/987/download": "piped-bytes",
		"/pages":                                `{"id":12345}`,
	})
	testutil.StubAPI(t, handler)

	stdout, _, err := testutil.Execute(t, attachment.NewCmd, "download", "12345", "987")
	if err != nil {
		t.Fatalf("attachment download returned error: %v", err)
	}
	if stdout != "piped-bytes" {
		t.Errorf("stdout = %q, want the raw file bytes", stdout)
	}
}

func TestAttachmentUploadRejectsEmptyFile(t *testing.T) {
	handler, log := testutil.RouteHandler(map[string]string{"/pages": `{"id":12345}`})
	testutil.StubAPI(t, handler)

	path := filepath.Join(t.TempDir(), "empty.txt")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	_, _, err := testutil.Execute(t, attachment.NewCmd, "upload", "12345", path)
	if err == nil {
		t.Fatal("attachment upload accepted an empty file")
	}

	for i := range log.Len() {
		if strings.Contains(log.At(i).Path, "upload_sessions") {
			t.Error("attachment upload opened a session for an empty file")
		}
	}
}
