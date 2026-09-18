package page

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	"github.com/cloud-yyy/ywiki/internal/cmd/jsonfields"
	"github.com/cloud-yyy/ywiki/internal/output"
)

// defaultPageSize matches the API default for descendant listings.
const defaultPageSize = 50

func newListCmd() *cobra.Command {
	var (
		pageSize    int
		cursor      string
		includeSelf bool
		recursive   bool
		actuality   string
	)

	cmd := &cobra.Command{
		Use:   "list <slug|id>",
		Short: "List subpages of a page",
		Long: `List the subpages of a Yandex Wiki page.

By default only direct children are listed. Use --recursive for the whole
subtree. Results are cursor-paginated; the next cursor is reported in JSON
output and can be passed back with --cursor.

JSON FIELDS
  id, slug, title`,
		Example: `  # List direct subpages
  ywiki page list users/me

  # List the whole subtree as JSON
  ywiki page list users/me --recursive --json id,slug

  # Continue a listing
  ywiki page list users/me --cursor <next-cursor>`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, args[0], api.ListDescendantsOptions{
				PageSize:    pageSize,
				Cursor:      cursor,
				IncludeSelf: includeSelf,
				ShowAll:     recursive,
				Actuality:   actuality,
			})
		},
	}

	cmd.Flags().IntVar(&pageSize, "limit", defaultPageSize, "Results per page (1-100)")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Continue a previous listing")
	cmd.Flags().BoolVar(&includeSelf, "include-self", false, "Include the page itself")
	cmd.Flags().BoolVar(&recursive, "recursive", false, "List the whole subtree, not just direct children")
	cmd.Flags().StringVar(&actuality, "actuality", "", "Filter by actuality: actual or obsolete")

	jsonfields.Register("ywiki page list", PageRefFields)

	return cmd
}

func runList(cmd *cobra.Command, ref string, opts api.ListDescendantsOptions) error {
	if err := cmdutil.PrepareFields(cmd, "page list", PageRefFields); err != nil {
		return err
	}

	client, err := cmdutil.Client(cmd)
	if err != nil {
		return err
	}

	result, err := client.ListDescendants(cmd.Context(), api.ParsePageLocator(ref), opts)
	if err != nil {
		return err
	}

	items := make([]pageRefItem, len(result.Items))
	for i, p := range result.Items {
		items[i] = pageRefItem{ID: p.ID, Slug: p.Slug, Title: p.Title}
	}

	w := cmd.OutOrStdout()

	if output.IsJSON() {
		return cmdutil.RenderList(w, items, result.NextCursor)
	}

	if output.IsQuiet() {
		slugs := make([]string, len(items))
		for i, item := range items {
			slugs[i] = item.Slug
		}
		output.PrintQuiet(w, slugs...)
		return nil
	}

	if len(items) == 0 {
		_, err := fmt.Fprintln(w, "No subpages found")
		return err
	}

	tbl := output.NewTable(w)
	tbl.AddHeader("ID", "SLUG", "TITLE")
	for _, item := range items {
		tbl.AddRow(item.ID, item.Slug, item.Title)
	}
	tbl.Render()

	if result.NextCursor != "" {
		_, _ = fmt.Fprintf(w, "\nMore results available. Continue with: --cursor %s\n", result.NextCursor)
	}

	return nil
}
