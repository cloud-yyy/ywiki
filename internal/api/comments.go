package api

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// ListCommentsOptions tunes a comment listing.
type ListCommentsOptions struct {
	// PageSize is the number of results per page.
	PageSize int

	// Cursor continues a previous listing.
	Cursor string
}

// ListComments lists the comments on a page.
func (c *Client) ListComments(
	ctx context.Context, pageID int, opts ListCommentsOptions,
) (*ListResult[Comment], error) {
	query := url.Values{}
	if opts.PageSize > 0 {
		query.Set("page_size", strconv.Itoa(opts.PageSize))
	}
	if opts.Cursor != "" {
		query.Set("cursor", opts.Cursor)
	}

	var list cursorList[Comment]
	if err := c.do(ctx, request{
		method: http.MethodGet,
		path:   "/pages/" + strconv.Itoa(pageID) + "/comments",
		query:  query,
	}, &list); err != nil {
		return nil, err
	}

	return &ListResult[Comment]{Items: list.Results, NextCursor: list.NextCursor}, nil
}

// CreateCommentInput is the body of a comment creation request.
type CreateCommentInput struct {
	Body       string `json:"body"`
	InlineText string `json:"inline_text,omitempty"`
	ParentID   *int   `json:"parent_id,omitempty"`
	ThreadID   *int   `json:"thread_id,omitempty"`
}

// CreateComment adds a comment to a page.
func (c *Client) CreateComment(
	ctx context.Context, pageID int, in CreateCommentInput,
) (*Comment, error) {
	var comment Comment
	if err := c.do(ctx, request{
		method: http.MethodPost,
		path:   "/pages/" + strconv.Itoa(pageID) + "/comments",
		body:   in,
	}, &comment); err != nil {
		return nil, err
	}

	return &comment, nil
}

// ListThread lists the replies in a comment thread.
func (c *Client) ListThread(
	ctx context.Context, pageID, commentID int, opts ListCommentsOptions,
) (*ListResult[Comment], error) {
	query := url.Values{}
	if opts.PageSize > 0 {
		query.Set("page_size", strconv.Itoa(opts.PageSize))
	}
	if opts.Cursor != "" {
		query.Set("cursor", opts.Cursor)
	}

	var list cursorList[Comment]
	if err := c.do(ctx, request{
		method: http.MethodGet,
		path: "/pages/" + strconv.Itoa(pageID) +
			"/comments/" + strconv.Itoa(commentID) + "/thread",
		query: query,
	}, &list); err != nil {
		return nil, err
	}

	return &ListResult[Comment]{Items: list.Results, NextCursor: list.NextCursor}, nil
}

// DeleteComment removes a comment from a page.
func (c *Client) DeleteComment(ctx context.Context, pageID, commentID int) error {
	return c.do(ctx, request{
		method: http.MethodDelete,
		path: "/pages/" + strconv.Itoa(pageID) +
			"/comments/" + strconv.Itoa(commentID),
	}, nil)
}
