package grid

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	"github.com/cloud-yyy/ywiki/internal/cmd/jsonfields"
	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
)

func newUpdateCmd() *cobra.Command {
	var (
		title string
		sort  string
	)

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Rename a table or set its default sort",
		Long: `Change a table's title or default row order.

--sort takes comma-separated column slugs, descending when prefixed with a
minus sign.

JSON FIELDS
  id, revision, rowIds`,
		Example: `  # Rename a table
  ywiki grid update 5f0c2d1e-... --title "Q1 tasks"

  # Sort by status, then newest first
  ywiki grid update 5f0c2d1e-... --sort status,-created`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			in := api.UpdateGridInput{Title: strings.TrimSpace(title)}
			if cmd.Flags().Changed("sort") {
				in.DefaultSort = parseSort(sort)
			}
			if in.Title == "" && len(in.DefaultSort) == 0 {
				return wikierrors.NewUserError(
					"nothing to update",
					"Pass --title and/or --sort",
				)
			}

			return runUpdate(cmd, args[0], in)
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "New title")
	cmd.Flags().StringVar(&sort, "sort", "", "Default sort, e.g. status,-created")

	jsonfields.Register("ywiki grid update", WriteFields)

	return cmd
}

// parseSort turns "a,-b" into [{"a": "asc"}, {"b": "desc"}].
func parseSort(spec string) []api.GridSort {
	var sorts []api.GridSort
	for _, key := range splitList(spec) {
		direction := "asc"
		if rest, desc := strings.CutPrefix(key, "-"); desc {
			key, direction = rest, "desc"
		}
		sorts = append(sorts, api.GridSort{key: direction})
	}

	return sorts
}

func runUpdate(cmd *cobra.Command, arg string, in api.UpdateGridInput) error {
	if err := cmdutil.PrepareFields(cmd, "grid update", WriteFields); err != nil {
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

	newRevision, err := client.UpdateGrid(cmd.Context(), id, in)
	if err != nil {
		return err
	}

	return renderWrite(cmd, writeItem{ID: id, Revision: newRevision}, "Updated")
}
