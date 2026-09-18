// Package testutil provides shared test helpers for ywiki command tests.
package testutil

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	"github.com/cloud-yyy/ywiki/internal/config"
	"github.com/cloud-yyy/ywiki/internal/output"
)

// ResetOutputFlags resets global output flags and restores them after the test.
func ResetOutputFlags(t *testing.T) {
	t.Helper()
	output.ResetFlags()
	t.Cleanup(func() {
		output.ResetFlags()
	})
}

// Ptr returns a pointer to v.
func Ptr[T any](v T) *T { return new(v) }

// StubAPI starts a test server with the given handler, points every command's
// client at it, and supplies complete credentials through the environment so
// config.ResolveAuth succeeds without touching the user's real config file.
// Everything is undone when the test ends.
func StubAPI(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	t.Setenv("YWIKI_TOKEN", "test-token")
	t.Setenv("YWIKI_ORG_ID", "test-org")
	t.Setenv("YWIKI_ORG_TYPE", "360")
	t.Setenv("YWIKI_CONFIG_DIR", t.TempDir())

	original := cmdutil.NewClient
	cmdutil.NewClient = func(auth *config.ResolvedAuth) *api.Client {
		return api.NewClient(auth, api.WithBaseURL(server.URL))
	}
	t.Cleanup(func() {
		cmdutil.NewClient = original
	})

	ResetOutputFlags(t)

	return server
}

// JSONHandler returns a handler that replies to every request with status and
// the given raw JSON body, recording the last request path and method.
func JSONHandler(status int, body string) (http.HandlerFunc, *RequestLog) {
	log := &RequestLog{}

	return func(w http.ResponseWriter, r *http.Request) {
		log.Record(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}, log
}

// newTestRoot builds a root command carrying the same persistent flags as the
// real one, so commands under test see the global output and auth flags.
func newTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "ywiki", SilenceErrors: true, SilenceUsage: true}

	root.PersistentFlags().StringSliceVar(&output.JSONFields, "json", nil, "")
	root.PersistentFlags().StringVar(&output.JQFilter, "jq", "", "")
	root.PersistentFlags().BoolVar(&output.QuietFlag, "quiet", false, "")
	root.PersistentFlags().BoolVar(&output.DebugFlag, "debug", false, "")
	root.PersistentFlags().String("token", "", "")
	root.PersistentFlags().String("org-id", "", "")
	root.PersistentFlags().String("org-type", "", "")

	return root
}

// Execute runs a command with the given args, capturing stdout and stderr.
// The command is built fresh by newCmd so flag state never leaks between
// tests, and root persistent flags are attached so commands that read the
// global auth flags behave as they do under the real root command.
func Execute(
	t *testing.T, newCmd func() *cobra.Command, args ...string,
) (string, string, error) {
	t.Helper()

	root := newTestRoot()

	cmd := newCmd()
	root.AddCommand(cmd)

	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetIn(strings.NewReader(""))
	root.SetArgs(append([]string{cmd.Name()}, args...))

	err := root.ExecuteContext(context.Background())

	return outBuf.String(), errBuf.String(), err
}

// ExecuteWithStdin is Execute with the given text on the command's stdin.
func ExecuteWithStdin(
	t *testing.T, newCmd func() *cobra.Command, stdin string, args ...string,
) (string, string, error) {
	t.Helper()

	root := newTestRoot()

	cmd := newCmd()
	root.AddCommand(cmd)

	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs(append([]string{cmd.Name()}, args...))

	err := root.ExecuteContext(context.Background())

	return outBuf.String(), errBuf.String(), err
}

// RouteHandler returns a handler that replies with the body whose path prefix
// matches the request. The longest matching prefix wins, so "/pages" can serve
// a slug lookup while "/pages/12345/comments" serves the call that follows it.
// Unmatched paths return 404 with a Wiki-shaped error body.
func RouteHandler(routes map[string]string) (http.HandlerFunc, *RequestLog) {
	log := &RequestLog{}

	prefixes := make([]string, 0, len(routes))
	for prefix := range routes {
		prefixes = append(prefixes, prefix)
	}
	sort.Slice(prefixes, func(i, j int) bool {
		return len(prefixes[i]) > len(prefixes[j])
	})

	return func(w http.ResponseWriter, r *http.Request) {
		log.Record(r)
		w.Header().Set("Content-Type", "application/json")

		for _, prefix := range prefixes {
			if strings.HasPrefix(r.URL.Path, prefix) {
				_, _ = w.Write([]byte(routes[prefix]))
				return
			}
		}

		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error_code":"NOT_FOUND","debug_message":"no route"}`))
	}, log
}
