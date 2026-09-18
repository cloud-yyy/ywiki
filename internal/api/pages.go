package api

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// PageLocator addresses a page either by numeric ID or by slug. The Wiki API
// exposes both: some endpoints take an ID path segment, while lookups by slug
// go through a query parameter. Commands accept whichever the user typed.
type PageLocator struct {
	ID   int
	Slug string
}

// ParsePageLocator interprets a user-supplied page reference. An all-digit
// value is treated as an ID; anything else is a slug. A leading slash is
// trimmed so both "users/me/page" and "/users/me/page" work.
func ParsePageLocator(value string) PageLocator {
	trimmed := strings.TrimSpace(value)
	if id, err := strconv.Atoi(trimmed); err == nil && id > 0 {
		return PageLocator{ID: id}
	}

	return PageLocator{Slug: strings.TrimPrefix(trimmed, "/")}
}

// IsID reports whether the locator addresses a page by numeric ID.
func (l PageLocator) IsID() bool { return l.ID > 0 }

// String renders the locator as the user supplied it, for messages.
func (l PageLocator) String() string {
	if l.IsID() {
		return strconv.Itoa(l.ID)
	}
	return l.Slug
}

// GetPageOptions tunes a page read.
type GetPageOptions struct {
	// Fields selects optional response blocks (comma-separated API field names).
	Fields string

	// RaiseOnRedirect makes the API fail instead of following a page redirect.
	RaiseOnRedirect bool
}

// GetPage fetches a page by ID or slug.
func (c *Client) GetPage(ctx context.Context, loc PageLocator, opts GetPageOptions) (*Page, error) {
	query := url.Values{}
	if opts.Fields != "" {
		query.Set("fields", opts.Fields)
	}
	if opts.RaiseOnRedirect {
		query.Set("raise_on_redirect", "true")
	}

	path := "/pages"
	if loc.IsID() {
		path = "/pages/" + strconv.Itoa(loc.ID)
	} else {
		query.Set("slug", loc.Slug)
	}

	var page Page
	if err := c.do(ctx, request{method: http.MethodGet, path: path, query: query}, &page); err != nil {
		return nil, err
	}

	return &page, nil
}

// CreatePageInput is the body of a page creation request.
type CreatePageInput struct {
	Slug         string              `json:"slug"`
	Title        string              `json:"title"`
	Content      string              `json:"content,omitempty"`
	AccessPolicy *AccessPolicyUpdate `json:"access_policy,omitempty"`
}

// AccessPolicyUpdate sets the access policy on create or update.
type AccessPolicyUpdate struct {
	AccessType   string `json:"access_type"`
	AllStaffRole string `json:"all_staff_role,omitempty"`
}

// CreatePage creates a page. When silent is true, subscribers are not notified.
func (c *Client) CreatePage(ctx context.Context, in CreatePageInput, silent bool) (*Page, error) {
	query := url.Values{}
	if silent {
		query.Set("is_silent", "true")
	}

	var page Page
	err := c.do(ctx, request{
		method: http.MethodPost,
		path:   "/pages",
		query:  query,
		body:   in,
	}, &page)
	if err != nil {
		return nil, err
	}

	return &page, nil
}

// UpdatePageInput is the body of a page update request. Fields are pointers so
// an omitted field is left untouched rather than cleared.
type UpdatePageInput struct {
	Title        *string             `json:"title,omitempty"`
	Content      *string             `json:"content,omitempty"`
	AccessPolicy *AccessPolicyUpdate `json:"access_policy,omitempty"`
}

// UpdatePageOptions tunes a page update.
type UpdatePageOptions struct {
	// AllowMerge resolves concurrent edits with a 3-way merge instead of
	// failing with a conflict.
	AllowMerge bool

	// Silent suppresses the notification sent to page subscribers.
	Silent bool
}

// UpdatePage updates a page by ID.
func (c *Client) UpdatePage(
	ctx context.Context, id int, in UpdatePageInput, opts UpdatePageOptions,
) (*Page, error) {
	query := url.Values{}
	if opts.AllowMerge {
		query.Set("allow_merge", "true")
	}
	if opts.Silent {
		query.Set("is_silent", "true")
	}

	var page Page
	err := c.do(ctx, request{
		method: http.MethodPost,
		path:   "/pages/" + strconv.Itoa(id),
		query:  query,
		body:   in,
	}, &page)
	if err != nil {
		return nil, err
	}

	return &page, nil
}

// AppendContentInput is the body of an append-content request.
type AppendContentInput struct {
	Content string          `json:"content"`
	Body    *AppendPosition `json:"body,omitempty"`
}

// AppendPosition selects where appended content lands: "top" or "bottom".
type AppendPosition struct {
	Location string `json:"location"`
}

// AppendContent appends content to a page without rewriting the whole body.
func (c *Client) AppendContent(
	ctx context.Context, id int, in AppendContentInput, silent bool,
) (*Page, error) {
	query := url.Values{}
	if silent {
		query.Set("is_silent", "true")
	}

	var page Page
	err := c.do(ctx, request{
		method: http.MethodPost,
		path:   "/pages/" + strconv.Itoa(id) + "/append-content",
		query:  query,
		body:   in,
	}, &page)
	if err != nil {
		return nil, err
	}

	return &page, nil
}

// DeletePage deletes a page by ID.
func (c *Client) DeletePage(ctx context.Context, id int) error {
	return c.do(ctx, request{
		method: http.MethodDelete,
		path:   "/pages/" + strconv.Itoa(id),
	}, nil)
}

// ClonePageInput is the body of a page clone request.
type ClonePageInput struct {
	// Target is the slug the copy is created at.
	Target string `json:"target"`

	// Title overrides the copy's title; empty keeps the source title.
	Title string `json:"title,omitempty"`

	// SubscribeMe subscribes the caller to the new page.
	SubscribeMe bool `json:"subscribe_me,omitempty"`
}

// CloneResponse identifies the asynchronous operation started by a clone.
type CloneResponse struct {
	Operation struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	} `json:"operation"`
	DryRun    bool   `json:"dry_run"`
	StatusURL string `json:"status_url"`
}

// ClonePage copies a page to a new slug. Cloning is asynchronous: the response
// identifies an operation whose status is polled via GetCloneOperation.
func (c *Client) ClonePage(ctx context.Context, id int, in ClonePageInput) (*CloneResponse, error) {
	var resp CloneResponse
	err := c.do(ctx, request{
		method: http.MethodPost,
		path:   "/pages/" + strconv.Itoa(id) + "/clone",
		body:   in,
	}, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// ListDescendantsOptions tunes a subpage listing.
type ListDescendantsOptions struct {
	// PageSize is the number of results per page (1-100, API default 50).
	PageSize int

	// Cursor continues a previous listing.
	Cursor string

	// IncludeSelf includes the page itself in the results.
	IncludeSelf bool

	// ShowAll returns the whole subtree instead of direct children only.
	ShowAll bool

	// Actuality filters by "actual" or "obsolete".
	Actuality string
}

// ListDescendants lists the subpages of a page.
func (c *Client) ListDescendants(
	ctx context.Context, loc PageLocator, opts ListDescendantsOptions,
) (*ListResult[PageRef], error) {
	query := url.Values{}
	if opts.PageSize > 0 {
		query.Set("page_size", strconv.Itoa(opts.PageSize))
	}
	if opts.Cursor != "" {
		query.Set("cursor", opts.Cursor)
	}
	if opts.IncludeSelf {
		query.Set("include_self", "true")
	}
	if opts.ShowAll {
		query.Set("show_all", "true")
	}
	if opts.Actuality != "" {
		query.Set("actuality", opts.Actuality)
	}

	path := "/pages/descendants"
	if loc.IsID() {
		path = "/pages/" + strconv.Itoa(loc.ID) + "/descendants"
	} else {
		query.Set("slug", loc.Slug)
	}

	var list cursorList[PageRef]
	if err := c.do(ctx, request{method: http.MethodGet, path: path, query: query}, &list); err != nil {
		return nil, err
	}

	return &ListResult[PageRef]{Items: list.Results, NextCursor: list.NextCursor}, nil
}

// ResolvePageID returns the numeric ID for a locator, looking it up by slug
// when needed. Endpoints that only accept an ID path segment use it so the
// user can still pass a slug.
func (c *Client) ResolvePageID(ctx context.Context, loc PageLocator) (int, error) {
	if loc.IsID() {
		return loc.ID, nil
	}

	page, err := c.GetPage(ctx, loc, GetPageOptions{})
	if err != nil {
		return 0, err
	}

	return page.ID, nil
}
