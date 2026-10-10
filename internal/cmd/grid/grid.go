// Package grid provides dynamic table commands for the ywiki CLI.
package grid

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
	"github.com/cloud-yyy/ywiki/internal/output"
)

// rowIDKey is the key under which a row's ID appears in row objects. Column
// slugs cannot start with an underscore in practice, so it never collides.
const rowIDKey = "_id"

// JSON field names repeated across the field lists below.
const (
	fieldTitle    = "title"
	fieldRevision = "revision"
)

// GridFields lists the available JSON field names for table output.
var GridFields = []string{"id", fieldTitle, "createdAt", "page", fieldRevision, "columns", "rows"}

// GridRefFields lists the available JSON field names for the table listing.
var GridRefFields = []string{"id", fieldTitle, "createdAt"}

// WriteFields lists the available JSON field names for commands that change a
// table.
var WriteFields = []string{"id", fieldRevision, "rowIds"}

type columnItem struct {
	Slug          string   `json:"slug"`
	Title         string   `json:"title"`
	Type          string   `json:"type"`
	Required      bool     `json:"required,omitempty"`
	Multiple      bool     `json:"multiple,omitempty"`
	SelectOptions []string `json:"selectOptions,omitempty"`
}

// gridItem is the JSON-serializable view of a table. Rows are objects keyed by
// column slug rather than the API's positional arrays, so a consumer does not
// have to line cells up with the column list.
type gridItem struct {
	ID        string           `json:"id"`
	Title     string           `json:"title"`
	CreatedAt string           `json:"createdAt,omitempty"`
	Page      string           `json:"page,omitempty"`
	Revision  string           `json:"revision"`
	Columns   []columnItem     `json:"columns"`
	Rows      []map[string]any `json:"rows"`
}

type gridRefItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// writeItem is the result of a command that changes a table. It always carries
// the new revision, so a script can chain writes without re-reading.
type writeItem struct {
	ID       string   `json:"id"`
	Revision string   `json:"revision"`
	RowIDs   []string `json:"rowIds,omitempty"`
}

func toGridItem(g *api.Grid) gridItem {
	item := gridItem{
		ID:       g.ID.String(),
		Title:    g.Title,
		Revision: g.Revision,
		Columns:  make([]columnItem, len(g.Structure.Columns)),
		Rows:     make([]map[string]any, len(g.Rows)),
	}

	if !g.CreatedAt.IsZero() {
		item.CreatedAt = g.CreatedAt.Format(time.RFC3339)
	}
	if g.Page != nil {
		item.Page = g.Page.Slug
	}

	for i, col := range g.Structure.Columns {
		item.Columns[i] = columnItem{
			Slug:          col.Slug,
			Title:         col.Title,
			Type:          col.Type,
			Required:      col.Required,
			Multiple:      col.Multiple,
			SelectOptions: col.SelectOptions,
		}
	}

	for i, row := range g.Rows {
		obj := make(map[string]any, len(g.Structure.Columns)+1)
		obj[rowIDKey] = row.ID.String()
		for j, col := range g.Structure.Columns {
			if j < len(row.Row) {
				obj[col.Slug] = cellValue(col, row.Row[j])
			} else {
				obj[col.Slug] = nil
			}
		}
		item.Rows[i] = obj
	}

	return item
}

// cellValue restores the type a column declares. The API stores numbers as
// text and returns them as strings, which would force every consumer to
// convert them back.
func cellValue(col api.GridColumn, v any) any {
	text, ok := v.(string)
	if !ok || col.Type != api.ColumnNumber {
		return v
	}

	if _, err := strconv.ParseFloat(text, 64); err != nil {
		return v
	}

	return json.Number(text)
}

func toGridRefItem(g *api.GridRef) gridRefItem {
	item := gridRefItem{ID: g.ID.String(), Title: g.Title}
	if !g.CreatedAt.IsZero() {
		item.CreatedAt = g.CreatedAt.Format(time.RFC3339)
	}

	return item
}

// cellDisplayKeys are the keys tried, in order, to turn an object-valued cell
// (a user, a ticket) into a short label.
var cellDisplayKeys = []string{"display", "name", "login", "slug", "key", "id"}

// cellText renders a cell value as plain text for table and CSV output.
func cellText(v any) string {
	switch value := v.(type) {
	case nil:
		return ""
	case string:
		return value
	case bool:
		return strconv.FormatBool(value)
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case []any:
		parts := make([]string, len(value))
		for i, part := range value {
			parts[i] = cellText(part)
		}
		return strings.Join(parts, ", ")
	case map[string]any:
		for _, key := range cellDisplayKeys {
			if text, ok := value[key].(string); ok && text != "" {
				return text
			}
		}
	}

	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}

	return string(data)
}

// gridID normalizes a table ID argument.
func gridID(arg string) (string, error) {
	id := strings.TrimSpace(arg)
	if id == "" {
		return "", wikierrors.NewUserError(
			"missing table ID",
			"Pass the table ID shown by: ywiki grid list <page>",
		)
	}

	return id, nil
}

// loadSchema reads the table a change is about to be applied to, so that input
// can be checked and converted against its columns before anything is written.
func loadSchema(cmd *cobra.Command, client *api.Client, id string) (*api.Grid, error) {
	return client.GetGrid(cmd.Context(), id, api.GetGridOptions{})
}

// renderWrite prints the outcome of a change: the new revision, plus the IDs
// of any rows it created. Quiet mode prints those row IDs, or the revision
// when the change created none.
func renderWrite(cmd *cobra.Command, item writeItem, summary string) error {
	w := cmd.OutOrStdout()

	if output.IsJSON() {
		return cmdutil.RenderJSON(w, item)
	}

	if output.IsQuiet() {
		if len(item.RowIDs) > 0 {
			output.PrintQuiet(w, item.RowIDs...)
		} else {
			output.PrintQuiet(w, item.Revision)
		}
		return nil
	}

	_, err := fmt.Fprintf(w, "%s (table %s, revision %s)\n", summary, item.ID, item.Revision)

	return err
}

// NewCmd creates the parent "grid" command with its subcommands.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "grid",
		Aliases: []string{"table"},
		Short:   "Manage dynamic tables",
		Long: `Read and edit Yandex Wiki dynamic tables (grids).

A table is addressed by its ID, shown by "ywiki grid list <page>". Rows are
addressed by row ID and columns by slug; "ywiki grid get" prints both.

Every change prints the table's new revision. The API does not reject a change
made against an outdated revision, so concurrent edits to one table are not
detected: the last write to a cell wins.

A new table only appears in the page body once a {% wgrid id="..." %} tag
references it; "ywiki grid create --embed" adds that tag.`,
	}

	cmd.AddCommand(
		newGetCmd(),
		newListCmd(),
		newCreateCmd(),
		newUpdateCmd(),
		newDeleteCmd(),
		newCloneCmd(),
		newRowCmd(),
		newColumnCmd(),
		newCellCmd(),
	)

	return cmd
}
