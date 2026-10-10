package grid

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	"github.com/cloud-yyy/ywiki/internal/cmd/jsonfields"
	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
)

func newCellCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cell",
		Short: "Change table cells",
	}

	cmd.AddCommand(newCellSetCmd())

	return cmd
}

func newCellSetCmd() *cobra.Command {
	var (
		clearCell bool
		cells     string
		cellsFile string
	)

	cmd := &cobra.Command{
		Use:   "set <id> [<row-id> <column> <value>]",
		Short: "Set one cell, or a batch of cells",
		Long: `Set cell values in a Yandex Wiki dynamic table.

For one cell, pass the row ID, the column slug, and the value. The value is
converted to the column's type: numbers and checkboxes (true or false) are
parsed, select columns take one option or a comma-separated list, and staff and
ticket values are given as JSON, e.g. [{"uid": "123"}]. Use --clear to empty a
cell.

For several cells in one revision, pass --cells or --cells-file (- for stdin)
with a JSON array of {"row_id", "column", "value"} objects. Values there keep
their JSON types and are sent as given.

Row IDs and column slugs are shown by "ywiki grid get <id>".

JSON FIELDS
  id, revision, rowIds`,
		Example: `  # Set a text cell
  ywiki grid cell set 5f0c2d1e-... 3 name "Write docs"

  # Tick a checkbox, empty another cell
  ywiki grid cell set 5f0c2d1e-... 3 done true
  ywiki grid cell set 5f0c2d1e-... 3 note --clear

  # Several cells at once
  echo '[{"row_id": 3, "column": "qty", "value": 5},
         {"row_id": 4, "column": "qty", "value": 7}]' |
    ywiki grid cell set 5f0c2d1e-... --cells-file -`,
		Args: cobra.RangeArgs(1, 4),
		RunE: func(cmd *cobra.Command, args []string) error {
			text, batch, err := cmdutil.ReadInput(cmd, "cells", cells, "cells-file", cellsFile)
			if err != nil {
				return err
			}

			if batch {
				if len(args) != 1 || clearCell {
					return wikierrors.NewUserError(
						"--cells cannot be combined with a row, column, or value",
						"Pass either <row-id> <column> <value>, or a batch of cells",
					)
				}
				return runCellSet(cmd, args[0], func(g *api.Grid) ([]api.GridCell, error) {
					return parseCells(text, g)
				})
			}

			return runCellSet(cmd, args[0], func(g *api.Grid) ([]api.GridCell, error) {
				return singleCell(args[1:], clearCell, g)
			})
		},
	}

	cmd.Flags().BoolVar(&clearCell, "clear", false, "Empty the cell instead of setting a value")
	cmd.Flags().StringVar(&cells, "cells", "", "Batch of cells as a JSON array")
	cmd.Flags().StringVar(&cellsFile, "cells-file", "", "Read the batch from a file, or - for stdin")
	cmd.MarkFlagsMutuallyExclusive("cells", "cells-file")
	jsonfields.Register("ywiki grid cell set", WriteFields)

	return cmd
}

// singleCell builds the one change described by <row-id> <column> [<value>].
func singleCell(args []string, clearCell bool, g *api.Grid) ([]api.GridCell, error) {
	wantArgs := 3
	if clearCell {
		wantArgs = 2
	}
	if len(args) != wantArgs {
		return nil, wikierrors.NewUserError(
			"expected <row-id> <column> <value>, or <row-id> <column> with --clear",
			"Pass a --cells batch to change several cells at once",
		)
	}

	rowID, slug := args[0], args[1]

	col, ok := g.Column(slug)
	if !ok {
		return nil, unknownColumn(slug, g)
	}

	if clearCell {
		return []api.GridCell{{RowID: rowID, ColumnSlug: slug, Value: nil}}, nil
	}

	value, err := coerceText(col, args[2])
	if err != nil {
		return nil, err
	}

	return []api.GridCell{{RowID: rowID, ColumnSlug: slug, Value: value}}, nil
}

func runCellSet(
	cmd *cobra.Command, arg string, build func(*api.Grid) ([]api.GridCell, error),
) error {
	if err := cmdutil.PrepareFields(cmd, "grid cell set", WriteFields); err != nil {
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

	g, err := loadSchema(cmd, client, id)
	if err != nil {
		return err
	}

	changes, err := build(g)
	if err != nil {
		return err
	}

	newRevision, err := client.UpdateGridCells(cmd.Context(), id, changes)
	if err != nil {
		return err
	}

	return renderWrite(cmd,
		writeItem{ID: id, Revision: newRevision},
		fmt.Sprintf("Set %d cell(s)", len(changes)))
}
