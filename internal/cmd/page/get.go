package page

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	"github.com/cloud-yyy/ywiki/internal/cmd/jsonfields"
	"github.com/cloud-yyy/ywiki/internal/output"
)

func newGetCmd() *cobra.Command {
	var (
		contentOnly     bool
		raiseOnRedirect bool
	)

	cmd := &cobra.Command{
		Use:   "get <slug|id>",
		Short: "Show a page",
		Long: `Show a Yandex Wiki page by slug or ID.

By default the page metadata and body are printed. Use --content to print only
the raw page body, which is what you want when piping a page into a file or
another tool.

JSON FIELDS
  id, slug, title, type, content, createdAt, modifiedAt, author, url`,
		Example: `  # Show a page by slug
  ywiki page get users/me/notes

  # Print only the page body
  ywiki page get users/me/notes --content > notes.md

  # Show a page by ID as JSON
  ywiki page get 12345 --json id,title,modifiedAt`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGet(cmd, args[0], contentOnly, raiseOnRedirect)
		},
	}

	cmd.Flags().BoolVar(&contentOnly, "content", false, "Print only the raw page body")
	cmd.Flags().BoolVar(&raiseOnRedirect, "raise-on-redirect", false,
		"Fail instead of following a page redirect")

	jsonfields.Register("ywiki page get", PageFields)

	return cmd
}

func runGet(cmd *cobra.Command, ref string, contentOnly, raiseOnRedirect bool) error {
	if err := cmdutil.PrepareFields(cmd, "page get", PageFields); err != nil {
		return err
	}

	client, err := cmdutil.Client(cmd)
	if err != nil {
		return err
	}

	page, err := client.GetPage(cmd.Context(), api.ParsePageLocator(ref), api.GetPageOptions{
		RaiseOnRedirect: raiseOnRedirect,
	})
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	item := toPageItem(page)

	if output.IsJSON() {
		return cmdutil.RenderJSON(w, item)
	}

	if contentOnly {
		_, err := fmt.Fprintln(w, page.Content)
		return err
	}

	if output.IsQuiet() {
		output.PrintQuiet(w, page.Slug)
		return nil
	}

	_, _ = fmt.Fprintf(w, "%s\n", item.Title)
	_, _ = fmt.Fprintf(w, "slug: %s\n", item.Slug)
	_, _ = fmt.Fprintf(w, "id: %d\n", item.ID)
	if item.Author != "" {
		_, _ = fmt.Fprintf(w, "author: %s\n", item.Author)
	}
	if item.ModifiedAt != "" {
		_, _ = fmt.Fprintf(w, "modified: %s\n", item.ModifiedAt)
	}
	_, _ = fmt.Fprintf(w, "url: %s\n", item.URL)

	if page.Content != "" {
		_, _ = fmt.Fprintf(w, "\n%s\n", page.Content)
	}

	return nil
}
