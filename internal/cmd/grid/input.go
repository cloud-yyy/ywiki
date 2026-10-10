package grid

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
)

// unknownColumn reports a column slug the table does not have, listing the
// ones it does so the caller can correct the input without another read.
func unknownColumn(slug string, g *api.Grid) error {
	return wikierrors.NewUserError(
		fmt.Sprintf("unknown column %q", slug),
		"Columns: "+strings.Join(g.ColumnSlugs(), ", "),
	)
}

// looksLikeJSON reports whether text is a JSON array or object literal.
func looksLikeJSON(text string) bool {
	return strings.HasPrefix(text, "[") || strings.HasPrefix(text, "{")
}

// coerceText turns the text of one input cell into the value its column
// expects. CSV and command-line arguments only carry strings, so numbers and
// checkboxes must be converted; list-valued and object-valued cells are given
// as JSON.
func coerceText(col *api.GridColumn, text string) (any, error) {
	switch col.Type {
	case api.ColumnNumber:
		if _, err := strconv.ParseFloat(text, 64); err != nil {
			return nil, wikierrors.NewUserError(
				fmt.Sprintf("column %q is a number, got %q", col.Slug, text),
				"Pass a plain number such as 42 or 3.5",
			)
		}
		return json.Number(text), nil
	case api.ColumnCheckbox:
		checked, err := strconv.ParseBool(text)
		if err != nil {
			return nil, wikierrors.NewUserError(
				fmt.Sprintf("column %q is a checkbox, got %q", col.Slug, text),
				"Pass true or false",
			)
		}
		return checked, nil
	case api.ColumnSelect:
		// The API takes a list for every select cell, single-choice included.
		if looksLikeJSON(text) {
			return parseJSONValue(col, text)
		}
		return splitList(text), nil
	case api.ColumnStaff, api.ColumnTicket, api.ColumnTicketField:
		if !looksLikeJSON(text) {
			return nil, wikierrors.NewUserError(
				fmt.Sprintf("column %q takes a JSON value, got %q", col.Slug, text),
				`Pass JSON; staff takes users by ID, e.g. [{"uid": "123"}]`,
			)
		}
		return parseJSONValue(col, text)
	}

	return text, nil
}

// parseJSONValue parses a cell value given as JSON text.
func parseJSONValue(col *api.GridColumn, text string) (any, error) {
	var value any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		return nil, wikierrors.NewUserError(
			fmt.Sprintf("column %q: invalid JSON value: %v", col.Slug, err),
			"Pass a valid JSON array or object",
		)
	}

	return value, nil
}

// splitList splits a comma-separated list, dropping blanks.
func splitList(text string) []string {
	var items []string
	for part := range strings.SplitSeq(text, ",") {
		if part = strings.TrimSpace(part); part != "" {
			items = append(items, part)
		}
	}

	return items
}

// parseRows reads rows to insert. The text is a JSON array of objects, a
// stream of JSON objects (one per line is conventional), or CSV with a header
// row of column slugs. Objects map column slugs to cell values; an "_id" key,
// as printed by "grid get", is ignored. Every key must be a column of g.
func parseRows(text string, g *api.Grid) ([]map[string]any, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, wikierrors.NewUserError(
			"no rows to add",
			"Pass --rows or --rows-file (use - for stdin)",
		)
	}

	var (
		rows []map[string]any
		err  error
	)
	switch {
	case strings.HasPrefix(trimmed, "["):
		err = json.Unmarshal([]byte(trimmed), &rows)
	case strings.HasPrefix(trimmed, "{"):
		rows, err = decodeObjectStream(trimmed)
	default:
		rows, err = parseCSVRows(trimmed, g)
	}
	if err != nil {
		return nil, wikierrors.NewUserError(
			"invalid rows: "+err.Error(),
			"Pass a JSON array of objects, one JSON object per line, or CSV with a header row of column slugs",
		)
	}

	for _, row := range rows {
		delete(row, rowIDKey)
		for slug := range row {
			if _, ok := g.Column(slug); !ok {
				return nil, unknownColumn(slug, g)
			}
		}
	}

	if len(rows) == 0 {
		return nil, wikierrors.NewUserError("no rows to add", "The input contained no rows")
	}

	return rows, nil
}

func decodeObjectStream(text string) ([]map[string]any, error) {
	dec := json.NewDecoder(strings.NewReader(text))

	var rows []map[string]any
	for {
		var row map[string]any
		err := dec.Decode(&row)
		if errors.Is(err, io.EOF) {
			return rows, nil
		}
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
}

func parseCSVRows(text string, g *api.Grid) ([]map[string]any, error) {
	reader := csv.NewReader(strings.NewReader(text))
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, errors.New("CSV needs a header row and at least one data row")
	}

	header := records[0]
	rows := make([]map[string]any, 0, len(records)-1)
	for _, record := range records[1:] {
		row := make(map[string]any, len(header))
		for i, slug := range header {
			if i >= len(record) || record[i] == "" || slug == rowIDKey {
				continue
			}

			col, ok := g.Column(slug)
			if !ok {
				return nil, unknownColumn(slug, g)
			}
			value, err := coerceText(col, record[i])
			if err != nil {
				return nil, err
			}
			row[slug] = value
		}
		rows = append(rows, row)
	}

	return rows, nil
}

// cellInput is one cell change in a batch file.
type cellInput struct {
	RowID      json.RawMessage `json:"row_id"`
	Column     string          `json:"column"`
	ColumnSlug string          `json:"column_slug"`
	Value      any             `json:"value"`
}

// parseCells reads a batch of cell changes: a JSON array of
// {"row_id": ..., "column": "slug", "value": ...} objects. Values are sent as
// given, so numbers and lists keep their JSON types.
func parseCells(text string, g *api.Grid) ([]api.GridCell, error) {
	var inputs []cellInput
	if err := json.Unmarshal([]byte(text), &inputs); err != nil {
		return nil, wikierrors.NewUserError(
			"invalid cells: "+err.Error(),
			`Pass a JSON array like [{"row_id": "3", "column": "name", "value": "x"}]`,
		)
	}
	if len(inputs) == 0 {
		return nil, wikierrors.NewUserError("no cells to set", "The input contained no cells")
	}

	cells := make([]api.GridCell, len(inputs))
	for i, in := range inputs {
		slug := in.Column
		if slug == "" {
			slug = in.ColumnSlug
		}
		if _, ok := g.Column(slug); !ok {
			return nil, unknownColumn(slug, g)
		}

		rowID := strings.Trim(string(bytes.TrimSpace(in.RowID)), `"`)
		if rowID == "" || rowID == "null" {
			return nil, wikierrors.NewUserError(
				fmt.Sprintf("cell %d has no row_id", i+1),
				"Row IDs are shown by: ywiki grid get <id> --quiet",
			)
		}

		cells[i] = api.GridCell{RowID: rowID, ColumnSlug: slug, Value: in.Value}
	}

	return cells, nil
}

// columnSpecParts is the most parts a column spec has: slug, type, title. The
// title is last so it may itself contain colons.
const columnSpecParts = 3

// parseColumnSpec parses "slug:type[:title]" into a column definition.
func parseColumnSpec(spec string) (api.GridColumn, error) {
	parts := strings.SplitN(spec, ":", columnSpecParts)
	if len(parts) < 2 || parts[0] == "" {
		return api.GridColumn{}, wikierrors.NewUserError(
			fmt.Sprintf("invalid column %q", spec),
			"Use slug:type or slug:type:title, e.g. --column price:number:Price",
		)
	}

	col := api.GridColumn{Slug: parts[0], Type: parts[1], Title: parts[0]}
	if !slices.Contains(api.ColumnTypes, col.Type) {
		return api.GridColumn{}, wikierrors.NewUserError(
			fmt.Sprintf("unknown column type %q in %q", col.Type, spec),
			"Types: "+strings.Join(api.ColumnTypes, ", "),
		)
	}
	if len(parts) == columnSpecParts && parts[2] != "" {
		col.Title = parts[2]
	}

	return col, nil
}

// columnFlags carries the flags that define new columns.
type columnFlags struct {
	specs []string
	file  string
}

func (f *columnFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringArrayVar(&f.specs, "column", nil,
		"Column as slug:type[:title], repeatable (types: "+strings.Join(api.ColumnTypes, ", ")+")")
	cmd.Flags().StringVar(&f.file, "columns-file", "",
		"Read full column definitions (JSON array) from a file, or - for stdin")
}

// columns resolves the columns the user asked for, from --column specs and
// --columns-file together. It returns an empty list when neither was given.
func (f *columnFlags) columns(cmd *cobra.Command) ([]api.GridColumn, error) {
	columns := make([]api.GridColumn, 0, len(f.specs))
	for _, spec := range f.specs {
		col, err := parseColumnSpec(spec)
		if err != nil {
			return nil, err
		}
		columns = append(columns, col)
	}

	text, ok, err := cmdutil.ReadInput(cmd, "", "", "columns-file", f.file)
	if err != nil {
		return nil, err
	}
	if ok {
		var fromFile []api.GridColumn
		if err := json.Unmarshal([]byte(text), &fromFile); err != nil {
			return nil, wikierrors.NewUserError(
				"invalid columns file: "+err.Error(),
				`Pass a JSON array like [{"slug": "name", "title": "Name", "type": "string"}]`,
			)
		}
		columns = append(columns, fromFile...)
	}

	return columns, nil
}
