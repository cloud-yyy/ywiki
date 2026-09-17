package api

import (
	"context"
	"net/http"
)

// SearchInput is the body of a search request.
type SearchInput struct {
	Query     string `json:"query"`
	Cursor    int    `json:"cursor,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	OrderBy   string `json:"order_by,omitempty"`
	Highlight bool   `json:"highlight,omitempty"`
}

// SearchPage is one page of search hits. Search paginates by an integer
// cursor (a page number, 1-500) rather than the opaque string cursor the
// list endpoints use.
type SearchPage struct {
	Items      []SearchResult
	NextCursor string
}

// Search runs a full-text search over Wiki pages.
func (c *Client) Search(ctx context.Context, in SearchInput) (*SearchPage, error) {
	var list cursorList[SearchResult]
	if err := c.do(ctx, request{
		method: http.MethodPost,
		path:   "/search",
		body:   in,
	}, &list); err != nil {
		return nil, err
	}

	return &SearchPage{Items: list.Results, NextCursor: list.NextCursor}, nil
}
