package grid

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	"github.com/cloud-yyy/ywiki/internal/cmd/jsonfields"
	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
	"github.com/cloud-yyy/ywiki/internal/output"
)

const (
	formatTable = "table"
	formatCSV   = "csv"
)

func newGetCmd() *cobra.Command {
	var (
		opts   api.GetGridOptions
		format string
	)

	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Show a table",
		Long: `Show a Yandex Wiki dynamic table: its columns, rows, and current revision.

In JSON each row is an object keyed by column slug, with the row ID under
"_id". "columns" lists every column with its type, so the value format each
cell expects is visible. Use --filter, --sort, --cols, and --rows to read only
part of a large table.

Filter syntax: [column] ~ text, [column] > 10, combined with AND / OR.

JSON FIELDS
  id, title, createdAt, page, revision, columns, rows`,
		Example: `  # Show a table
  ywiki grid get 5f0c2d1e-0000-4000-8000-000000000001

  # Export as CSV
  ywiki grid get 5f0c2d1e-... --format csv > table.csv

  # Rows as JSON, filtered and sorted
  ywiki grid get 5f0c2d1e-... --filter '[status] ~ open' --sort -created --jq '.rows'

  # Just the column definitions
  ywiki grid get 5f0c2d1e-... --json columns --jq '.columns'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != formatTable && format != formatCSV {
				return wikierrors.NewUserError(
					"invalid format: "+format,
					"Use --format table or --format csv",
				)
			}

			return runGet(cmd, args[0], opts, format)
		},
	}

	cmd.Flags().StringVar(&opts.Filter, "filter", "", "Keep rows matching an expression")
	cmd.Flags().StringVar(&opts.Sort, "sort", "", "Order rows, e.g. name,-price")
	cmd.Flags().StringSliceVar(&opts.OnlyCols, "cols", nil, "Show only these column slugs")
	cmd.Flags().StringSliceVar(&opts.OnlyRows, "rows", nil, "Show only these row IDs")
	cmd.Flags().StringVar(&opts.Revision, "revision", "", "Read an older revision")
	cmd.Flags().StringVar(&format, "format", formatTable, "Text format: table or csv")

	jsonfields.Register("ywiki grid get", GridFields)

	return cmd
}

func runGet(cmd *cobra.Command, arg string, opts api.GetGridOptions, format string) error {
	if err := cmdutil.PrepareFields(cmd, "grid get", GridFields); err != nil {
		return err
	}

	id, err := gridID(arg)
	if err != nil {
		return err
	}

	client, err := cmdutil.Client(cmd)
	if err != nil {
		return err
	}

	g, err := client.GetGrid(cmd.Context(), id, opts)
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	item := toGridItem(g)

	if output.IsJSON() {
		return cmdutil.RenderJSON(w, item)
	}

	if output.IsQuiet() {
		ids := make([]string, len(item.Rows))
		for i, row := range item.Rows {
			ids[i] = cellText(row[rowIDKey])
		}
		output.PrintQuiet(w, ids...)
		return nil
	}

	if format == formatCSV {
		return writeCSV(w, item)
	}

	return writeTable(w, item)
}

func writeTable(w io.Writer, item gridItem) error {
	if len(item.Columns) == 0 {
		_, err := fmt.Fprintf(w, "%s has no columns\n", item.Title)
		return err
	}

	tbl := output.NewTable(w)

	header := make([]any, 0, len(item.Columns)+1)
	header = append(header, "ID")
	for _, col := range item.Columns {
		header = append(header, col.Slug)
	}
	tbl.AddHeader(header...)

	for _, row := range item.Rows {
		cells := make([]any, 0, len(item.Columns)+1)
		cells = append(cells, cellText(row[rowIDKey]))
		for _, col := range item.Columns {
			cells = append(cells, cellText(row[col.Slug]))
		}
		tbl.AddRow(cells...)
	}
	tbl.Render()

	if len(item.Rows) == 0 {
		_, _ = fmt.Fprintln(w, "No rows")
	}
	_, err := fmt.Fprintf(w, "\n%s  revision %s\n", strings.TrimSpace(item.Title), item.Revision)

	return err
}

// writeCSV prints the table with the row ID as the first column. The header
// uses column slugs, so the output can be fed straight back to "grid row add".
func writeCSV(w io.Writer, item gridItem) error {
	out := csv.NewWriter(w)

	header := make([]string, 0, len(item.Columns)+1)
	header = append(header, rowIDKey)
	for _, col := range item.Columns {
		header = append(header, col.Slug)
	}
	if err := out.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV: %w", err)
	}

	for _, row := range item.Rows {
		record := make([]string, 0, len(header))
		record = append(record, cellText(row[rowIDKey]))
		for _, col := range item.Columns {
			record = append(record, cellText(row[col.Slug]))
		}
		if err := out.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV: %w", err)
		}
	}

	out.Flush()

	return out.Error()
}
