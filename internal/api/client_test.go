package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/config"
)

func newTestClient(t *testing.T, handler http.HandlerFunc, orgType config.OrgType) *api.Client {
	t.Helper()

	return newTestClientWithToken(t, handler, orgType, "")
}

func newTestClientWithToken(
	t *testing.T, handler http.HandlerFunc, orgType config.OrgType, tokenType config.TokenType,
) *api.Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return api.NewClient(&config.ResolvedAuth{
		Token:       "secret-token",
		OrgID:       "org-42",
		OrgType:     orgType,
		TokenType:   tokenType,
		TokenSource: "env",
	}, api.WithBaseURL(server.URL))
}

func TestClientSendsAuthHeaders(t *testing.T) {
	tests := []struct {
		name       string
		orgType    config.OrgType
		tokenType  config.TokenType
		wantAuth   string
		wantHeader string
	}{
		{
			name:       "Yandex 360 uses OAuth and X-Org-Id",
			orgType:    config.OrgType360,
			tokenType:  config.TokenTypeOAuth,
			wantAuth:   "OAuth secret-token",
			wantHeader: "X-Org-Id",
		},
		{
			name:       "Identity Hub uses OAuth and X-Cloud-Org-Id",
			orgType:    config.OrgTypeCloud,
			tokenType:  config.TokenTypeOAuth,
			wantAuth:   "OAuth secret-token",
			wantHeader: "X-Cloud-Org-Id",
		},
		{
			name:       "Yandex Cloud uses Bearer and X-Cloud-Org-Id",
			orgType:    config.OrgTypeCloud,
			tokenType:  config.TokenTypeIAM,
			wantAuth:   "Bearer secret-token",
			wantHeader: "X-Cloud-Org-Id",
		},
		{
			name:       "cloud with no token type keeps the pre-0.2 Bearer default",
			orgType:    config.OrgTypeCloud,
			wantAuth:   "Bearer secret-token",
			wantHeader: "X-Cloud-Org-Id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *http.Request
			client := newTestClientWithToken(t, func(w http.ResponseWriter, r *http.Request) {
				got = r
				_, _ = w.Write([]byte(`{"id":1,"username":"me"}`))
			}, tt.orgType, tt.tokenType)

			if _, err := client.GetCurrentUser(context.Background()); err != nil {
				t.Fatalf("GetCurrentUser returned error: %v", err)
			}

			if auth := got.Header.Get("Authorization"); auth != tt.wantAuth {
				t.Errorf("Authorization = %q, want %q", auth, tt.wantAuth)
			}
			if org := got.Header.Get(tt.wantHeader); org != "org-42" {
				t.Errorf("%s = %q, want %q", tt.wantHeader, org, "org-42")
			}
		})
	}
}

func TestGetPageByIDAndSlug(t *testing.T) {
	tests := []struct {
		name      string
		ref       string
		wantPath  string
		wantQuery string
	}{
		{
			name:     "numeric ref reads by ID",
			ref:      "12345",
			wantPath: "/pages/12345",
		},
		{
			name:      "slug ref reads by slug",
			ref:       "users/me/notes",
			wantPath:  "/pages",
			wantQuery: "slug=users%2Fme%2Fnotes",
		},
		{
			name:      "leading slash is trimmed from a slug",
			ref:       "/users/me/notes",
			wantPath:  "/pages",
			wantQuery: "slug=users%2Fme%2Fnotes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *http.Request
			client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				got = r
				_, _ = w.Write([]byte(`{"id":12345,"slug":"users/me/notes","title":"Notes"}`))
			}, config.OrgType360)

			page, err := client.GetPage(
				context.Background(), api.ParsePageLocator(tt.ref), api.GetPageOptions{},
			)
			if err != nil {
				t.Fatalf("GetPage returned error: %v", err)
			}
			if page.ID != 12345 {
				t.Errorf("page.ID = %d, want 12345", page.ID)
			}
			if got.URL.Path != tt.wantPath {
				t.Errorf("path = %q, want %q", got.URL.Path, tt.wantPath)
			}
			if got.URL.RawQuery != tt.wantQuery {
				t.Errorf("query = %q, want %q", got.URL.RawQuery, tt.wantQuery)
			}
		})
	}
}

func TestParsePageLocator(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantID   int
		wantSlug string
	}{
		{name: "numeric is an ID", input: "42", wantID: 42},
		{name: "slug stays a slug", input: "users/me", wantSlug: "users/me"},
		{name: "zero is not an ID", input: "0", wantSlug: "0"},
		{name: "negative is not an ID", input: "-1", wantSlug: "-1"},
		{name: "digits with text is a slug", input: "42-notes", wantSlug: "42-notes"},
		{name: "surrounding space is trimmed", input: "  users/me  ", wantSlug: "users/me"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc := api.ParsePageLocator(tt.input)
			if loc.ID != tt.wantID {
				t.Errorf("ID = %d, want %d", loc.ID, tt.wantID)
			}
			if loc.Slug != tt.wantSlug {
				t.Errorf("Slug = %q, want %q", loc.Slug, tt.wantSlug)
			}
		})
	}
}

func TestListDescendantsBuildsQuery(t *testing.T) {
	var got *http.Request
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		got = r
		_, _ = w.Write([]byte(`{"results":[{"id":1,"slug":"a"}],"next_cursor":"abc"}`))
	}, config.OrgType360)

	result, err := client.ListDescendants(
		context.Background(),
		api.ParsePageLocator("users/me"),
		api.ListDescendantsOptions{PageSize: 100, ShowAll: true, Cursor: "prev"},
	)
	if err != nil {
		t.Fatalf("ListDescendants returned error: %v", err)
	}

	if len(result.Items) != 1 || result.Items[0].Slug != "a" {
		t.Errorf("Items = %+v, want one item with slug a", result.Items)
	}
	if result.NextCursor != "abc" {
		t.Errorf("NextCursor = %q, want abc", result.NextCursor)
	}

	query := got.URL.Query()
	for key, want := range map[string]string{
		"page_size": "100",
		"show_all":  "true",
		"cursor":    "prev",
		"slug":      "users/me",
	} {
		if query.Get(key) != want {
			t.Errorf("query %s = %q, want %q", key, query.Get(key), want)
		}
	}
}
