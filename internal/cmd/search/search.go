// Package search provides the wiki search command for the ywiki CLI.
package search

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	"github.com/cloud-yyy/ywiki/internal/cmd/jsonfields"
	"github.com/cloud-yyy/ywiki/internal/output"
)

const (
	// defaultLimit matches the API default page size for search.
	defaultLimit = 10

	// snippetReservedWidth is the space taken by the title and slug columns
	// plus padding in table output.
	snippetReservedWidth = 60

	// snippetMinWidth keeps the snippet column readable on narrow terminals.
	snippetMinWidth = 20

	// titleColumnWidth caps the title and slug columns in table output.
	titleColumnWidth = 30
)

// SearchFields lists the available JSON field names for search output.
var SearchFields = []string{"slug", "title", "snippet", "type", "modifiedAt", "url"}

type searchItem struct {
	Slug       string `json:"slug"`
	Title      string `json:"title"`
	Snippet    string `json:"snippet,omitempty"`
	Type       string `json:"type,omitempty"`
	ModifiedAt string `json:"modifiedAt,omitempty"`
	URL        string `json:"url,omitempty"`
}

// NewCmd creates the "search" command.
func NewCmd() *cobra.Command {
	var (
		limit     int
		cursor    int
		orderBy   string
		highlight bool
	)

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search wiki pages",
		Long: `Full-text search over the pages you can read in Yandex Wiki.

Results are paginated by page number: --cursor 2 is the second page of results.

JSON FIELDS
  slug, title, snippet, type, modifiedAt, url`,
		Example: `  # Search for pages
  ywiki search "release checklist"

  # Get the top 50 hits as JSON
  ywiki search onboarding --limit 50 --json slug,title

  # List matching slugs only
  ywiki search onboarding --quiet`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, api.SearchInput{
				Query:     args[0],
				Limit:     limit,
				Cursor:    cursor,
				OrderBy:   orderBy,
				Highlight: highlight,
			})
		},
	}

	cmd.Flags().IntVar(&limit, "limit", defaultLimit, "Results per page (1-50)")
	cmd.Flags().IntVar(&cursor, "cursor", 0, "Result page number (1-500)")
	cmd.Flags().StringVar(&orderBy, "order-by", "", "Sort order: relevancy or updated_at")
	cmd.Flags().BoolVar(&highlight, "highlight", false, "Mark query matches in snippets")

	jsonfields.Register("ywiki search", SearchFields)

	return cmd
}

func run(cmd *cobra.Command, in api.SearchInput) error {
	if err := cmdutil.PrepareFields(cmd, "search", SearchFields); err != nil {
		return err
	}

	client, err := cmdutil.Client(cmd)
	if err != nil {
		return err
	}

	result, err := client.Search(cmd.Context(), in)
	if err != nil {
		return err
	}

	items := make([]searchItem, len(result.Items))
	for i, r := range result.Items {
		item := searchItem{
			Slug:    r.Slug,
			Title:   r.Title,
			Snippet: r.Content,
			Type:    r.Type,
			URL:     r.URL,
		}
		if !r.ModifiedAt.IsZero() {
			item.ModifiedAt = r.ModifiedAt.Format(time.RFC3339)
		}
		items[i] = item
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
		_, err := fmt.Fprintln(w, "No results found")
		return err
	}

	tbl := output.NewTable(w)
	tbl.AddHeader("TITLE", "SLUG", "SNIPPET")
	snippetWidth := max(output.TerminalWidth()-snippetReservedWidth, snippetMinWidth)
	for _, item := range items {
		tbl.AddRow(
			output.TruncateDisplay(item.Title, titleColumnWidth),
			output.TruncateDisplay(item.Slug, titleColumnWidth),
			output.TruncateDisplay(item.Snippet, snippetWidth),
		)
	}
	tbl.Render()

	return nil
}
