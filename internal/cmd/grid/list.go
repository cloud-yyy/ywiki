package grid

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	"github.com/cloud-yyy/ywiki/internal/cmd/jsonfields"
	"github.com/cloud-yyy/ywiki/internal/output"
)

func newListCmd() *cobra.Command {
	var (
		limit  int
		cursor string
	)

	cmd := &cobra.Command{
		Use:   "list <slug|id>",
		Short: "List tables on a page",
		Long: `List the dynamic tables attached to a Yandex Wiki page.

JSON FIELDS
  id, title, createdAt`,
		Example: `  # List the tables on a page
  ywiki grid list users/me/notes

  # Print table IDs only
  ywiki grid list users/me/notes --quiet`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, args[0], api.ListGridsOptions{PageSize: limit, Cursor: cursor})
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 0, "Results per page")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Continue a previous listing")

	jsonfields.Register("ywiki grid list", GridRefFields)

	return cmd
}

func runList(cmd *cobra.Command, ref string, opts api.ListGridsOptions) error {
	if err := cmdutil.PrepareFields(cmd, "grid list", GridRefFields); err != nil {
		return err
	}

	client, err := cmdutil.Client(cmd)
	if err != nil {
		return err
	}

	pageID, err := client.ResolvePageID(cmd.Context(), api.ParsePageLocator(ref))
	if err != nil {
		return err
	}

	result, err := client.ListGrids(cmd.Context(), pageID, opts)
	if err != nil {
		return err
	}

	items := make([]gridRefItem, len(result.Items))
	for i := range result.Items {
		items[i] = toGridRefItem(&result.Items[i])
	}

	w := cmd.OutOrStdout()

	if output.IsJSON() {
		return cmdutil.RenderList(w, items, result.NextCursor)
	}

	if output.IsQuiet() {
		ids := make([]string, len(items))
		for i, item := range items {
			ids[i] = item.ID
		}
		output.PrintQuiet(w, ids...)
		return nil
	}

	if len(items) == 0 {
		_, err := fmt.Fprintln(w, "No tables found")
		return err
	}

	tbl := output.NewTable(w)
	tbl.AddHeader("ID", "TITLE", "CREATED")
	for i, item := range items {
		created := "-"
		if !result.Items[i].CreatedAt.IsZero() {
			created = output.TimeAgo(result.Items[i].CreatedAt)
		}
		tbl.AddRow(item.ID, item.Title, created)
	}
	tbl.Render()

	if result.NextCursor != "" {
		_, _ = fmt.Fprintf(w, "\nMore results available. Continue with: --cursor %s\n", result.NextCursor)
	}

	return nil
}
