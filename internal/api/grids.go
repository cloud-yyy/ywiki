package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// FlexID is an identifier the API sends as either a JSON string or a number.
// Grid and row IDs are documented both ways, so it is kept as a string.
type FlexID string

// UnmarshalJSON accepts a JSON string or number.
func (id *FlexID) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if string(data) == "null" {
		*id = ""
		return nil
	}

	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("invalid identifier: %w", err)
		}
		*id = FlexID(s)
		return nil
	}

	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("invalid identifier: %w", err)
	}
	*id = FlexID(n.String())

	return nil
}

// String returns the identifier as text.
func (id FlexID) String() string { return string(id) }

// GridColumn is a column definition. The same shape describes existing columns
// in a read and new columns in an add request, so optional fields are omitted
// when empty. Required is the exception: the API rejects a new column that
// lacks it, so it is always sent, false included.
type GridColumn struct {
	ID            string          `json:"id,omitempty"`
	Slug          string          `json:"slug"`
	Title         string          `json:"title"`
	Type          string          `json:"type"`
	Required      bool            `json:"required"`
	Width         int             `json:"width,omitempty"`
	WidthUnits    string          `json:"width_units,omitempty"`
	Pinned        string          `json:"pinned,omitempty"`
	Color         string          `json:"color,omitempty"`
	Format        string          `json:"format,omitempty"`
	Multiple      bool            `json:"multiple,omitempty"`
	SelectOptions []string        `json:"select_options,omitempty"`
	MarkRows      bool            `json:"mark_rows,omitempty"`
	TicketField   json.RawMessage `json:"ticket_field,omitempty"`
	Description   string          `json:"description,omitempty"`
}

// Column types the API accepts.
const (
	ColumnString      = "string"
	ColumnNumber      = "number"
	ColumnDate        = "date"
	ColumnSelect      = "select"
	ColumnStaff       = "staff"
	ColumnCheckbox    = "checkbox"
	ColumnTicket      = "ticket"
	ColumnTicketField = "ticket_field"
)

// ColumnTypes lists every column type, in the order the docs give them.
var ColumnTypes = []string{
	ColumnString, ColumnNumber, ColumnDate, ColumnSelect,
	ColumnStaff, ColumnCheckbox, ColumnTicket, ColumnTicketField,
}

// GridRow is one table row. Cells are positional: Row[i] belongs to the i-th
// column of the grid's structure.
type GridRow struct {
	ID     FlexID `json:"id"`
	Row    []any  `json:"row"`
	Pinned bool   `json:"pinned"`
	Color  string `json:"color"`
}

// GridSort is one default-sort entry, a single-key object mapping a column
// slug to "asc" or "desc".
type GridSort map[string]string

// GridStructure describes a grid's columns and default ordering.
type GridStructure struct {
	Columns     []GridColumn `json:"columns"`
	DefaultSort []GridSort   `json:"default_sort"`
}

// Grid is a dynamic table.
type Grid struct {
	ID        FlexID        `json:"id"`
	Title     string        `json:"title"`
	CreatedAt time.Time     `json:"created_at"`
	Page      *PageRef      `json:"page"`
	Revision  string        `json:"revision"`
	Structure GridStructure `json:"structure"`
	Rows      []GridRow     `json:"rows"`
}

// Column returns the column with the given slug.
func (g *Grid) Column(slug string) (*GridColumn, bool) {
	for i := range g.Structure.Columns {
		if g.Structure.Columns[i].Slug == slug {
			return &g.Structure.Columns[i], true
		}
	}

	return nil, false
}

// ColumnSlugs lists the column slugs in display order.
func (g *Grid) ColumnSlugs() []string {
	slugs := make([]string, len(g.Structure.Columns))
	for i := range g.Structure.Columns {
		slugs[i] = g.Structure.Columns[i].Slug
	}

	return slugs
}

// GridRef is the trimmed grid entry returned by the page grid listing.
type GridRef struct {
	ID        FlexID    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

func gridPath(id string, suffix ...string) string {
	return "/grids/" + url.PathEscape(id) + strings.Join(suffix, "")
}

// GetGridOptions tunes a grid read.
type GetGridOptions struct {
	// Filter keeps matching rows, e.g. "[price] > 10 AND [name] ~ wiki".
	Filter string

	// Sort orders rows, e.g. "name, -price".
	Sort string

	// OnlyCols keeps only these column slugs.
	OnlyCols []string

	// OnlyRows keeps only these row IDs.
	OnlyRows []string

	// Revision reads an older version of the table.
	Revision string
}

// GetGrid reads a table with its structure and rows.
func (c *Client) GetGrid(ctx context.Context, id string, opts GetGridOptions) (*Grid, error) {
	query := url.Values{}
	if opts.Filter != "" {
		query.Set("filter", opts.Filter)
	}
	if opts.Sort != "" {
		query.Set("sort", opts.Sort)
	}
	if len(opts.OnlyCols) > 0 {
		query.Set("only_cols", strings.Join(opts.OnlyCols, ","))
	}
	if len(opts.OnlyRows) > 0 {
		query.Set("only_rows", strings.Join(opts.OnlyRows, ","))
	}
	if opts.Revision != "" {
		query.Set("revision", opts.Revision)
	}

	var grid Grid
	if err := c.do(ctx, request{
		method: http.MethodGet,
		path:   gridPath(id),
		query:  query,
	}, &grid); err != nil {
		return nil, err
	}

	return &grid, nil
}

// ListGridsOptions tunes a page's table listing.
type ListGridsOptions struct {
	// PageSize is the number of results per page (1-100, API default 50).
	PageSize int

	// Cursor continues a previous listing.
	Cursor string
}

// ListGrids lists the tables attached to a page.
func (c *Client) ListGrids(
	ctx context.Context, pageID int, opts ListGridsOptions,
) (*ListResult[GridRef], error) {
	query := url.Values{}
	if opts.PageSize > 0 {
		query.Set("page_size", strconv.Itoa(opts.PageSize))
	}
	if opts.Cursor != "" {
		query.Set("cursor", opts.Cursor)
	}

	var list cursorList[GridRef]
	if err := c.do(ctx, request{
		method: http.MethodGet,
		path:   "/pages/" + strconv.Itoa(pageID) + "/grids",
		query:  query,
	}, &list); err != nil {
		return nil, err
	}

	return &ListResult[GridRef]{Items: list.Results, NextCursor: list.NextCursor}, nil
}

// pageSelector is the "page" object of a grid creation request.
type pageSelector struct {
	ID   int    `json:"id,omitempty"`
	Slug string `json:"slug,omitempty"`
}

// CreateGrid creates an empty table on a page. Columns are added afterwards
// with AddGridColumns, and the table shows up in the page body only once a
// {% wgrid %} tag referencing it is added to the content.
func (c *Client) CreateGrid(ctx context.Context, title string, page PageLocator) (*Grid, error) {
	selector := pageSelector{Slug: page.Slug}
	if page.IsID() {
		selector = pageSelector{ID: page.ID}
	}

	var grid Grid
	if err := c.do(ctx, request{
		method: http.MethodPost,
		path:   "/grids",
		body: struct {
			Title string       `json:"title"`
			Page  pageSelector `json:"page"`
		}{Title: title, Page: selector},
	}, &grid); err != nil {
		return nil, err
	}

	return &grid, nil
}

// UpdateGridInput is the body of a table update. Empty fields are left alone.
type UpdateGridInput struct {
	Title       string     `json:"title,omitempty"`
	DefaultSort []GridSort `json:"default_sort,omitempty"`
}

// gridRevision is the response of every grid write that reports no content.
type gridRevision struct {
	Revision string `json:"revision"`
}

// UpdateGrid changes a table's title or default sort and returns the new revision.
func (c *Client) UpdateGrid(ctx context.Context, id string, in UpdateGridInput) (string, error) {
	var out gridRevision
	if err := c.do(ctx, request{
		method: http.MethodPost,
		path:   gridPath(id),
		body:   in,
	}, &out); err != nil {
		return "", err
	}

	return out.Revision, nil
}

// DeleteGrid deletes a table.
func (c *Client) DeleteGrid(ctx context.Context, id string) error {
	return c.do(ctx, request{method: http.MethodDelete, path: gridPath(id)}, nil)
}

// CloneGridInput is the body of a table clone request.
type CloneGridInput struct {
	// Target is the slug of the page the copy goes to; it is created if missing.
	Target string `json:"target"`

	// Title overrides the copy's title.
	Title string `json:"title,omitempty"`

	// WithData copies the rows too; by default only the structure is copied.
	WithData bool `json:"with_data,omitempty"`
}

// CloneGrid copies a table to another page. Cloning is asynchronous, so the
// response identifies an operation to poll with GetOperationByURL.
func (c *Client) CloneGrid(ctx context.Context, id string, in CloneGridInput) (*CloneResponse, error) {
	var resp CloneResponse
	if err := c.do(ctx, request{
		method: http.MethodPost,
		path:   gridPath(id, "/clone"),
		body:   in,
	}, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// AddGridRowsInput is the body of an add-rows request. Each row maps a column
// slug to its cell value.
type AddGridRowsInput struct {
	Rows       []map[string]any `json:"rows"`
	Position   *int             `json:"position,omitempty"`
	AfterRowID string           `json:"after_row_id,omitempty"`
}

// AddGridRowsResult is the outcome of an add-rows request.
type AddGridRowsResult struct {
	Revision string    `json:"revision"`
	Results  []GridRow `json:"results"`
}

// AddGridRows inserts rows into a table.
func (c *Client) AddGridRows(
	ctx context.Context, id string, in AddGridRowsInput,
) (*AddGridRowsResult, error) {
	var out AddGridRowsResult
	if err := c.do(ctx, request{
		method: http.MethodPost,
		path:   gridPath(id, "/rows"),
		body:   in,
	}, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// RemoveGridRows deletes rows and returns the new revision.
func (c *Client) RemoveGridRows(ctx context.Context, id string, rowIDs []string) (string, error) {
	var out gridRevision
	if err := c.do(ctx, request{
		method: http.MethodDelete,
		path:   gridPath(id, "/rows"),
		body: struct {
			RowIDs []string `json:"row_ids"`
		}{RowIDs: rowIDs},
	}, &out); err != nil {
		return "", err
	}

	return out.Revision, nil
}

// MoveGridRowInput is the body of a move-row request. Exactly one of
// AfterRowID and Position places the row.
type MoveGridRowInput struct {
	RowID      string `json:"row_id"`
	AfterRowID string `json:"after_row_id,omitempty"`
	Position   *int   `json:"position,omitempty"`
}

// MoveGridRow moves a row and returns the new revision.
func (c *Client) MoveGridRow(ctx context.Context, id string, in MoveGridRowInput) (string, error) {
	var out gridRevision
	if err := c.do(ctx, request{
		method: http.MethodPost,
		path:   gridPath(id, "/rows/move"),
		body:   in,
	}, &out); err != nil {
		return "", err
	}

	return out.Revision, nil
}

// GridCell sets one cell. A nil Value clears it.
type GridCell struct {
	RowID      string
	ColumnSlug string
	Value      any
}

// MarshalJSON sends the row ID as a number when it is one: the cells endpoint
// documents row_id as an integer while every other endpoint takes a string.
func (c GridCell) MarshalJSON() ([]byte, error) {
	var rowID any = c.RowID
	if n, err := strconv.ParseInt(c.RowID, 10, 64); err == nil {
		rowID = n
	}

	return json.Marshal(struct {
		RowID      any    `json:"row_id"`
		ColumnSlug string `json:"column_slug"`
		Value      any    `json:"value"`
	}{RowID: rowID, ColumnSlug: c.ColumnSlug, Value: c.Value})
}

// UpdateGridCells sets cell values and returns the new revision.
func (c *Client) UpdateGridCells(ctx context.Context, id string, cells []GridCell) (string, error) {
	var out gridRevision
	if err := c.do(ctx, request{
		method: http.MethodPost,
		path:   gridPath(id, "/cells"),
		body: struct {
			Cells []GridCell `json:"cells"`
		}{Cells: cells},
	}, &out); err != nil {
		return "", err
	}

	return out.Revision, nil
}

// AddGridColumns appends columns, or inserts them at position, and returns the
// new revision.
func (c *Client) AddGridColumns(
	ctx context.Context, id string, columns []GridColumn, position *int,
) (string, error) {
	var out gridRevision
	if err := c.do(ctx, request{
		method: http.MethodPost,
		path:   gridPath(id, "/columns"),
		body: struct {
			Columns  []GridColumn `json:"columns"`
			Position *int         `json:"position,omitempty"`
		}{Columns: columns, Position: position},
	}, &out); err != nil {
		return "", err
	}

	return out.Revision, nil
}

// RemoveGridColumns deletes columns and returns the new revision.
func (c *Client) RemoveGridColumns(ctx context.Context, id string, slugs []string) (string, error) {
	var out gridRevision
	if err := c.do(ctx, request{
		method: http.MethodDelete,
		path:   gridPath(id, "/columns"),
		body: struct {
			ColumnSlugs []string `json:"column_slugs"`
		}{ColumnSlugs: slugs},
	}, &out); err != nil {
		return "", err
	}

	return out.Revision, nil
}

// MoveGridColumn moves a column to a zero-based position and returns the new
// revision.
func (c *Client) MoveGridColumn(ctx context.Context, id, slug string, position int) (string, error) {
	var out gridRevision
	if err := c.do(ctx, request{
		method: http.MethodPost,
		path:   gridPath(id, "/columns/move"),
		body: struct {
			ColumnSlug string `json:"column_slug"`
			Position   int    `json:"position"`
		}{ColumnSlug: slug, Position: position},
	}, &out); err != nil {
		return "", err
	}

	return out.Revision, nil
}
