package grid

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	"github.com/cloud-yyy/ywiki/internal/cmd/jsonfields"
	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
)

func newColumnCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "column",
		Short: "Add, remove, and move table columns",
	}

	cmd.AddCommand(newColumnAddCmd(), newColumnRemoveCmd(), newColumnMoveCmd())

	return cmd
}

func newColumnAddCmd() *cobra.Command {
	var (
		cols     columnFlags
		position int
	)

	cmd := &cobra.Command{
		Use:   "add <id>",
		Short: "Add columns to a table",
		Long: `Add columns to a Yandex Wiki dynamic table.

Define columns with --column slug:type[:title], repeated per column, or with
full definitions in --columns-file. Types: string, number, date, select, staff,
checkbox, ticket, ticket_field. Columns go at the end unless --position gives a
zero-based index.

JSON FIELDS
  id, revision, rowIds`,
		Example: `  # Add a number column
  ywiki grid column add 5f0c2d1e-... --column price:number:Price

  # Add a select column with options
  echo '[{"slug":"status","title":"Status","type":"select","select_options":["open","done"]}]' |
    ywiki grid column add 5f0c2d1e-... --columns-file -`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			columns, err := cols.columns(cmd)
			if err != nil {
				return err
			}
			if len(columns) == 0 {
				return wikierrors.NewUserError(
					"no columns to add",
					"Pass --column slug:type[:title], or --columns-file",
				)
			}

			return runColumnAdd(cmd, args[0], columns, position)
		},
	}

	cols.register(cmd)
	cmd.Flags().IntVar(&position, "position", 0, "Insert at this zero-based position (default: the end)")

	jsonfields.Register("ywiki grid column add", WriteFields)

	return cmd
}

func runColumnAdd(cmd *cobra.Command, arg string, columns []api.GridColumn, position int) error {
	if err := cmdutil.PrepareFields(cmd, "grid column add", WriteFields); err != nil {
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

	var at *int
	if cmd.Flags().Changed("position") {
		at = &position
	}

	newRevision, err := client.AddGridColumns(cmd.Context(), id, columns, at)
	if err != nil {
		return err
	}

	return renderWrite(cmd,
		writeItem{ID: id, Revision: newRevision},
		"Added "+strconv.Itoa(len(columns))+" column(s)")
}

func newColumnRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <id> <slug>...",
		Short: "Remove columns from a table",
		Long: `Remove columns from a Yandex Wiki dynamic table by slug. The cell values in
those columns are deleted with them.

JSON FIELDS
  id, revision, rowIds`,
		Example: `  # Remove a column
  ywiki grid column remove 5f0c2d1e-... price`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runColumnRemove(cmd, args[0], args[1:])
		},
	}

	jsonfields.Register("ywiki grid column remove", WriteFields)

	return cmd
}

func runColumnRemove(cmd *cobra.Command, arg string, slugs []string) error {
	if err := cmdutil.PrepareFields(cmd, "grid column remove", WriteFields); err != nil {
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

	newRevision, err := client.RemoveGridColumns(cmd.Context(), id, slugs)
	if err != nil {
		return err
	}

	return renderWrite(cmd,
		writeItem{ID: id, Revision: newRevision},
		"Removed "+strconv.Itoa(len(slugs))+" column(s)")
}

func newColumnMoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "move <id> <slug> <position>",
		Short: "Move a column within a table",
		Long: `Move a column of a Yandex Wiki dynamic table to a zero-based position.

JSON FIELDS
  id, revision, rowIds`,
		Example: `  # Make "name" the first column
  ywiki grid column move 5f0c2d1e-... name 0`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			position, err := strconv.Atoi(args[2])
			if err != nil || position < 0 {
				return wikierrors.NewUserError(
					"invalid position: "+args[2],
					"Pass a zero-based index such as 0",
				)
			}

			return runColumnMove(cmd, args[0], args[1], position)
		},
	}

	jsonfields.Register("ywiki grid column move", WriteFields)

	return cmd
}

func runColumnMove(cmd *cobra.Command, arg, slug string, position int) error {
	if err := cmdutil.PrepareFields(cmd, "grid column move", WriteFields); err != nil {
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

	newRevision, err := client.MoveGridColumn(cmd.Context(), id, slug, position)
	if err != nil {
		return err
	}

	return renderWrite(cmd, writeItem{ID: id, Revision: newRevision}, "Moved column "+slug)
}
