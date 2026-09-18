package api

import "time"

// User is the subset of the Wiki user schema the CLI renders.
type User struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	IsDismissed bool   `json:"is_dismissed"`
	Affiliation string `json:"affiliation"`
}

// PageAttributes holds page metadata returned alongside the page itself.
type PageAttributes struct {
	CreatedAt       time.Time `json:"created_at"`
	ModifiedAt      time.Time `json:"modified_at"`
	Lang            string    `json:"lang"`
	IsReadonly      bool      `json:"is_readonly"`
	CommentsCount   int       `json:"comments_count"`
	CommentsEnabled bool      `json:"comments_enabled"`
	Keywords        []string  `json:"keywords"`
	IsDraft         bool      `json:"is_draft"`
}

// Breadcrumb is one ancestor entry in a page's path.
type Breadcrumb struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	Slug       string `json:"slug"`
	PageExists bool   `json:"page_exists"`
}

// AccessPolicy describes how access to a page is granted.
type AccessPolicy struct {
	AccessType          string `json:"access_type"`
	InheritedAccessType string `json:"inherited_access_type"`
	AllStaffRole        string `json:"all_staff_role"`
	HasExternal         bool   `json:"has_external"`
}

// PageOwner identifies the owning user or group of a page.
type PageOwner struct {
	User *User `json:"user"`
}

// Page is a Wiki page as returned by the pages endpoints.
type Page struct {
	ID           int             `json:"id"`
	Slug         string          `json:"slug"`
	Title        string          `json:"title"`
	PageType     string          `json:"page_type"`
	Content      string          `json:"content"`
	Attributes   *PageAttributes `json:"attributes"`
	Breadcrumbs  []Breadcrumb    `json:"breadcrumbs"`
	AccessPolicy *AccessPolicy   `json:"access_policy"`
	Owner        *PageOwner      `json:"owner"`
}

// PageRef is the trimmed page representation returned by list endpoints,
// which carry only identity fields.
type PageRef struct {
	ID    int    `json:"id"`
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

// Comment is a single comment on a page.
type Comment struct {
	ID            int         `json:"id"`
	Body          string      `json:"body"`
	InlineText    string      `json:"inline_text"`
	ParentID      *int        `json:"parent_id"`
	ThreadID      *int        `json:"thread_id"`
	Author        *User       `json:"author"`
	CreatedAt     time.Time   `json:"created_at"`
	IsDeleted     bool        `json:"is_deleted"`
	ResolveStatus string      `json:"resolve_status"`
	ThreadInfo    *ThreadInfo `json:"thread_info"`
}

// ThreadInfo summarizes a comment thread.
type ThreadInfo struct {
	TotalPosts int `json:"total_posts"`
}

// Attachment is a file attached to a page. Size is a string in the API
// response, so it is kept as one rather than guessing at a numeric type.
type Attachment struct {
	ID             int       `json:"id"`
	Name           string    `json:"name"`
	Size           string    `json:"size"`
	Mimetype       string    `json:"mimetype"`
	Description    string    `json:"description"`
	IsDownloadable bool      `json:"is_downloadable"`
	DownloadURL    string    `json:"download_url"`
	CheckStatus    string    `json:"check_status"`
	CreatedAt      time.Time `json:"created_at"`
	User           *User     `json:"user"`
}

// SearchResult is a single hit from the search endpoint.
type SearchResult struct {
	URL        string    `json:"url"`
	Slug       string    `json:"slug"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Type       string    `json:"type"`
	ModifiedAt time.Time `json:"modified_at"`
}

// UploadSession tracks a multipart file upload.
type UploadSession struct {
	SessionID string `json:"session_id"`
	FileName  string `json:"file_name"`
	FileSize  int64  `json:"file_size"`
	Status    string `json:"status"`
}

// cursorList is the shared envelope for cursor-paginated list endpoints.
type cursorList[T any] struct {
	Results    []T    `json:"results"`
	NextCursor string `json:"next_cursor"`
	PrevCursor string `json:"prev_cursor"`
}

// ListResult carries one page of results plus the cursor for the next page.
type ListResult[T any] struct {
	Items      []T
	NextCursor string
}
