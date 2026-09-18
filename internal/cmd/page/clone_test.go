package page_test

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/cloud-yyy/ywiki/internal/cmd/page"
	"github.com/cloud-yyy/ywiki/internal/testutil"
)

func TestPageCloneWaitsForCompletion(t *testing.T) {
	var polls atomic.Int32

	testutil.StubAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.HasSuffix(r.URL.Path, "/clone"):
			_, _ = w.Write([]byte(`{"operation":{"type":"clone","id":"op-1"},"dry_run":false}`))
		case strings.HasPrefix(r.URL.Path, "/operations/clone/"):
			// The first poll is still running, so the command must poll again
			// rather than reporting an unfinished copy as done.
			if polls.Add(1) == 1 {
				_, _ = w.Write([]byte(`{"status":"in_progress"}`))
				return
			}
			_, _ = w.Write([]byte(
				`{"status":"success","result":{"page":{"id":999,"slug":"users/me/copy"}}}`))
		default:
			_, _ = w.Write([]byte(`{"id":12345,"slug":"users/me/template"}`))
		}
	})

	stdout, _, err := testutil.Execute(t, page.NewCmd,
		"clone", "12345", "--to", "users/me/copy")
	if err != nil {
		t.Fatalf("page clone returned error: %v", err)
	}

	if polls.Load() < 2 {
		t.Errorf("polled %d times, want the command to keep polling until done", polls.Load())
	}
	if !strings.Contains(stdout, "Copied to users/me/copy") {
		t.Errorf("stdout = %q, want the new page slug", stdout)
	}
}

func TestPageCloneNoWait(t *testing.T) {
	testutil.StubAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.HasPrefix(r.URL.Path, "/operations/") {
			t.Error("page clone --no-wait polled the operation")
		}
		if strings.HasSuffix(r.URL.Path, "/clone") {
			_, _ = w.Write([]byte(`{"operation":{"type":"clone","id":"op-1"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":12345,"slug":"users/me/template"}`))
	})

	stdout, _, err := testutil.Execute(t, page.NewCmd,
		"clone", "12345", "--to", "users/me/copy", "--no-wait")
	if err != nil {
		t.Fatalf("page clone --no-wait returned error: %v", err)
	}
	if !strings.Contains(stdout, "op-1") {
		t.Errorf("stdout = %q, want the operation ID", stdout)
	}
}

func TestPageCloneReportsFailure(t *testing.T) {
	testutil.StubAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.HasSuffix(r.URL.Path, "/clone"):
			_, _ = w.Write([]byte(`{"operation":{"type":"clone","id":"op-1"}}`))
		case strings.HasPrefix(r.URL.Path, "/operations/clone/"):
			_, _ = w.Write([]byte(`{"status":"failed"}`))
		default:
			_, _ = w.Write([]byte(`{"id":12345,"slug":"users/me/template"}`))
		}
	})

	_, _, err := testutil.Execute(t, page.NewCmd, "clone", "12345", "--to", "users/me/copy")
	if err == nil {
		t.Fatal("page clone reported success for a failed operation")
	}
}

func TestPageCloneRequiresTarget(t *testing.T) {
	handler, _ := testutil.JSONHandler(http.StatusOK, `{"id":12345}`)
	testutil.StubAPI(t, handler)

	_, _, err := testutil.Execute(t, page.NewCmd, "clone", "12345")
	if err == nil {
		t.Fatal("page clone ran without a destination slug")
	}
}
