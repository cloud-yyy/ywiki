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

func TestDownloadAllPaginatesAndKeepsNamesInsideDirectory(t *testing.T) {
	var cursors []string
	testutil.StubAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected mutation: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		switch r.URL.Path {
		case "/pages/12345/attachments":
			cursor := r.URL.Query().Get("cursor")
			cursors = append(cursors, cursor)
			if cursor == "" {
				_, _ = w.Write([]byte(`{"results":[{"id":1,"name":"../../same.txt"}],"next_cursor":"next"}`))
			} else {
				_, _ = w.Write([]byte(`{"results":[{"id":2,"name":"same.txt"},{"id":3,"name":"..\\outside.txt"}]}`))
			}
		case "/pages/12345/attachments/1/download":
			_, _ = w.Write([]byte("first"))
		case "/pages/12345/attachments/2/download":
			_, _ = w.Write([]byte("second"))
		case "/pages/12345/attachments/3/download":
			_, _ = w.Write([]byte("third"))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	dir := filepath.Join(t.TempDir(), "downloads")
	stdout, _, err := testutil.Execute(
		t,
		attachment.NewCmd,
		"download",
		"12345",
		"--all",
		"--out",
		dir,
		"--json",
		"id,name,path",
	)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"1-same.txt": "first", "2-same.txt": "second", "3-outside.txt": "third"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(data) != want {
			t.Errorf("%s = %q, %v; want %q", name, data, err, want)
		}
	}
	if strings.Join(cursors, ",") != ",next" {
		t.Errorf("cursors = %v", cursors)
	}
	var items []struct {
		ID   string `json:"id"`
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(stdout), &items); err != nil || len(items) != 3 {
		t.Fatalf("manifest = %s, error %v", stdout, err)
	}
	if items[0].ID != "1" || items[0].Path != filepath.Join(dir, "1-same.txt") {
		t.Errorf("manifest first item = %+v", items[0])
	}
}

func TestDownloadAllPreservesExistingFiles(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		t.Run(map[bool]string{false: "file", true: "symlink"}[symlink], func(t *testing.T) {
			handler, log := testutil.RouteHandler(map[string]string{"/pages/12345/attachments": attachmentsJSON})
			testutil.StubAPI(t, handler)
			dir := t.TempDir()
			target := filepath.Join(dir, "original")
			if err := os.WriteFile(target, []byte("keep"), 0o600); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "987-report.pdf")
			if symlink {
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
				t.Fatal(err)
			}
			_, _, err := testutil.Execute(t, attachment.NewCmd, "download", "12345", "--all", "--out", dir)
			if err == nil {
				t.Fatal("accepted existing destination")
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != "keep" {
				t.Fatalf("existing file changed: %q, %v", data, err)
			}
			if log.Len() != 1 {
				t.Errorf("made %d requests; want only the listing", log.Len())
			}
		})
	}
}

func TestDownloadAllRemovesIncompleteFile(t *testing.T) {
	testutil.StubAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/download") {
			w.Header().Set("Content-Length", "100")
			_, _ = w.Write([]byte("truncated"))
			return
		}
		_, _ = w.Write([]byte(attachmentsJSON))
	})
	dir := t.TempDir()
	_, _, err := testutil.Execute(t, attachment.NewCmd, "download", "12345", "--all", "--out", dir)
	if err == nil {
		t.Fatal("accepted truncated response")
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 0 {
		t.Fatalf("partial files remain: %v, %v", files, err)
	}
}

func TestDownloadAllEmptyAndQuiet(t *testing.T) {
	for _, quiet := range []bool{false, true} {
		t.Run(map[bool]string{false: "empty", true: "quiet"}[quiet], func(t *testing.T) {
			routes := map[string]string{"/pages/12345/attachments": `{"results":[]}`}
			if quiet {
				routes["/pages/12345/attachments"] = attachmentsJSON
				routes["/pages/12345/attachments/987/download"] = "bytes"
			}
			handler, _ := testutil.RouteHandler(routes)
			testutil.StubAPI(t, handler)
			dir := t.TempDir()
			args := []string{"download", "12345", "--all", "--out", dir}
			if quiet {
				args = append(args, "--quiet")
			}
			stdout, _, err := testutil.Execute(t, attachment.NewCmd, args...)
			if err != nil {
				t.Fatal(err)
			}
			if quiet && stdout != filepath.Join(dir, "987-report.pdf")+"\n" {
				t.Errorf("quiet output = %q", stdout)
			}
			if !quiet && !strings.Contains(stdout, "Downloaded 0 attachment(s)") {
				t.Errorf("empty output = %q", stdout)
			}
		})
	}
}

func TestDownloadRequiresFileIDOrAll(t *testing.T) {
	for _, args := range [][]string{{"download", "12345"}, {"download", "12345", "987", "--all"}} {
		if _, _, err := testutil.Execute(t, attachment.NewCmd, args...); err == nil {
			t.Errorf("accepted invalid arguments %v", args)
		}
	}
}

func TestDownloadAllKeepsCompletedFilesAfterLaterFailure(t *testing.T) {
	testutil.StubAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/pages/12345/attachments":
			_, _ = w.Write([]byte(`{"results":[{"id":1,"name":"a.txt"},{"id":2,"name":"b.txt"}]}`))
		case "/pages/12345/attachments/1/download":
			_, _ = w.Write([]byte("complete"))
		default:
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error_code":"FORBIDDEN"}`))
		}
	})
	dir := t.TempDir()
	_, stderr, err := testutil.Execute(t, attachment.NewCmd, "download", "12345", "--all", "--out", dir)
	if err == nil || !strings.Contains(err.Error(), "attachment 2") {
		t.Fatalf("error = %v, want the failed attachment ID", err)
	}
	data, readErr := os.ReadFile(filepath.Join(dir, "1-a.txt"))
	if readErr != nil || string(data) != "complete" {
		t.Fatalf("completed download = %q, %v", data, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "2-b.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("failed download remains: %v", statErr)
	}
	if !strings.Contains(stderr, "1-a.txt") {
		t.Errorf("stderr does not identify completed file: %q", stderr)
	}
}

func TestDownloadAllRejectsRepeatedCursor(t *testing.T) {
	handler, log := testutil.JSONHandler(http.StatusOK, `{"results":[],"next_cursor":"same"}`)
	testutil.StubAPI(t, handler)
	_, _, err := testutil.Execute(t, attachment.NewCmd, "download", "12345", "--all", "--out", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "repeated cursor") {
		t.Fatalf("error = %v, want a repeated cursor error", err)
	}
	if log.Len() != 2 {
		t.Errorf("made %d requests, want 2", log.Len())
	}
}
