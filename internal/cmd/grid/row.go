package grid

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	"github.com/cloud-yyy/ywiki/internal/cmd/jsonfields"
	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
)

func newRowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "row",
		Short: "Add, remove, and move table rows",
	}

	cmd.AddCommand(newRowAddCmd(), newRowRemoveCmd(), newRowMoveCmd())

	return cmd
}

// placement is where a new or moved row goes: after a row, or at a position.
type placement struct {
	after    string
	position int
}

func (p *placement) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&p.after, "after", "", "Place the row after this row ID")
	cmd.Flags().IntVar(&p.position, "position", 0, "Place the row at this zero-based position")
	cmd.MarkFlagsMutuallyExclusive("after", "position")
}

// positionPtr returns the --position value, or nil when the flag was not set:
// position 0 is a valid place, so zero cannot stand for "unset".
func (p *placement) positionPtr(cmd *cobra.Command) *int {
	if !cmd.Flags().Changed("position") {
		return nil
	}

	return &p.position
}

func newRowAddCmd() *cobra.Command {
	var (
		rows     string
		rowsFile string
		where    placement
	)

	cmd := &cobra.Command{
		Use:   "add <id>",
		Short: "Add rows to a table",
		Long: `Add rows to a Yandex Wiki dynamic table.

Rows come from --rows (inline) or --rows-file (a file, or - for stdin), as one
of:
  - a JSON array of objects:    [{"name": "a", "qty": 2}, {"name": "b"}]
  - JSON objects, one per line
  - CSV with a header row of column slugs

Each object maps column slugs to cell values; the "_id" key printed by
"ywiki grid get" is ignored, so its output can be fed back in. Unknown columns
are rejected and the valid ones listed. In CSV, values are converted to the
column's type (numbers, checkboxes, select lists); give staff and ticket values as
JSON.

Rows go at the end unless --after or --position says otherwise. The output
carries the new row IDs.

JSON FIELDS
  id, revision, rowIds`,
		Example: `  # Add one row
  ywiki grid row add 5f0c2d1e-... --rows '[{"name": "Write docs", "done": false}]'

  # Add rows from CSV on stdin
  printf 'name,qty\nbolt,10\nnut,20\n' | ywiki grid row add 5f0c2d1e-... --rows-file -

  # Insert at the top
  ywiki grid row add 5f0c2d1e-... --rows-file rows.json --position 0

  # Print the new row IDs only
  ywiki grid row add 5f0c2d1e-... --rows-file rows.json --quiet`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			text, ok, err := cmdutil.ReadInput(cmd, "rows", rows, "rows-file", rowsFile)
			if err != nil {
				return err
			}
			if !ok {
				return wikierrors.NewUserError(
					"no rows to add",
					"Pass --rows, or --rows-file (use - for stdin)",
				)
			}

			return runRowAdd(cmd, args[0], text, where)
		},
	}

	cmd.Flags().StringVar(&rows, "rows", "", "Rows to add, as JSON or CSV")
	cmd.Flags().StringVar(&rowsFile, "rows-file", "", "Read rows from a file, or - for stdin")
	cmd.MarkFlagsMutuallyExclusive("rows", "rows-file")
	where.register(cmd)

	jsonfields.Register("ywiki grid row add", WriteFields)

	return cmd
}

func runRowAdd(cmd *cobra.Command, arg, text string, where placement) error {
	if err := cmdutil.PrepareFields(cmd, "grid row add", WriteFields); err != nil {
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

	rows, err := parseRows(text, g)
	if err != nil {
		return err
	}

	result, err := client.AddGridRows(cmd.Context(), id, api.AddGridRowsInput{
		Rows:       rows,
		Position:   where.positionPtr(cmd),
		AfterRowID: where.after,
	})
	if err != nil {
		return err
	}

	rowIDs := make([]string, len(result.Results))
	for i, row := range result.Results {
		rowIDs[i] = row.ID.String()
	}

	summary := "Added " + strconv.Itoa(len(rows)) + " row(s)"

	return renderWrite(cmd, writeItem{ID: id, Revision: result.Revision, RowIDs: rowIDs}, summary)
}

func newRowRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <id> <row-id>...",
		Short: "Remove rows from a table",
		Long: `Remove rows from a Yandex Wiki dynamic table by row ID.

Row IDs are shown by "ywiki grid get <id> --quiet".

JSON FIELDS
  id, revision, rowIds`,
		Example: `  # Remove two rows
  ywiki grid row remove 5f0c2d1e-... 3 4`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRowRemove(cmd, args[0], args[1:])
		},
	}

	jsonfields.Register("ywiki grid row remove", WriteFields)

	return cmd
}

func runRowRemove(cmd *cobra.Command, arg string, rowIDs []string) error {
	if err := cmdutil.PrepareFields(cmd, "grid row remove", WriteFields); err != nil {
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

	newRevision, err := client.RemoveGridRows(cmd.Context(), id, rowIDs)
	if err != nil {
		return err
	}

	return renderWrite(cmd,
		writeItem{ID: id, Revision: newRevision},
		"Removed "+strconv.Itoa(len(rowIDs))+" row(s)")
}

func newRowMoveCmd() *cobra.Command {
	var where placement

	cmd := &cobra.Command{
		Use:   "move <id> <row-id>",
		Short: "Move a row within a table",
		Long: `Move a row of a Yandex Wiki dynamic table, after another row (--after) or to
a zero-based position (--position).

JSON FIELDS
  id, revision, rowIds`,
		Example: `  # Move row 7 to the top
  ywiki grid row move 5f0c2d1e-... 7 --position 0

  # Move row 7 below row 2
  ywiki grid row move 5f0c2d1e-... 7 --after 2`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if where.after == "" && !cmd.Flags().Changed("position") {
				return wikierrors.NewUserError(
					"no destination for the row",
					"Pass --after <row-id> or --position <n>",
				)
			}

			return runRowMove(cmd, args[0], args[1], where)
		},
	}

	where.register(cmd)

	jsonfields.Register("ywiki grid row move", WriteFields)

	return cmd
}

func runRowMove(cmd *cobra.Command, arg, rowID string, where placement) error {
	if err := cmdutil.PrepareFields(cmd, "grid row move", WriteFields); err != nil {
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

	newRevision, err := client.MoveGridRow(cmd.Context(), id, api.MoveGridRowInput{
		RowID:      rowID,
		AfterRowID: where.after,
		Position:   where.positionPtr(cmd),
	})
	if err != nil {
		return err
	}

	return renderWrite(cmd, writeItem{ID: id, Revision: newRevision}, "Moved row "+rowID)
}
