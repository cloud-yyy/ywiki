// Package page provides page management commands for the ywiki CLI.
package page

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
)

// fieldSlug is the JSON field name for a page slug, used in several field lists.
const fieldSlug = "slug"

// PageFields lists the available JSON field names for single-page output.
var PageFields = []string{
	"id", fieldSlug, "title", "type", "content", "createdAt", "modifiedAt", "author", "url",
}

// PageRefFields lists the available JSON field names for page list output.
// List endpoints return identity fields only.
var PageRefFields = []string{"id", fieldSlug, "title"}

// pageItem is the JSON-serializable view of a page.
type pageItem struct {
	ID         int    `json:"id"`
	Slug       string `json:"slug"`
	Title      string `json:"title"`
	Type       string `json:"type"`
	Content    string `json:"content,omitempty"`
	CreatedAt  string `json:"createdAt,omitempty"`
	ModifiedAt string `json:"modifiedAt,omitempty"`
	Author     string `json:"author,omitempty"`
	URL        string `json:"url,omitempty"`
}

// pageRefItem is the JSON-serializable view of a page list entry.
type pageRefItem struct {
	ID    int    `json:"id"`
	Slug  string `json:"slug"`
	Title string `json:"title,omitempty"`
}

func toPageItem(p *api.Page) pageItem {
	item := pageItem{
		ID:    p.ID,
		Slug:  p.Slug,
		Title: p.Title,
		Type:  p.PageType,
		URL:   pageURL(p.Slug),
	}

	if p.Content != "" {
		item.Content = p.Content
	}
	if p.Attributes != nil {
		if !p.Attributes.CreatedAt.IsZero() {
			item.CreatedAt = p.Attributes.CreatedAt.Format(time.RFC3339)
		}
		if !p.Attributes.ModifiedAt.IsZero() {
			item.ModifiedAt = p.Attributes.ModifiedAt.Format(time.RFC3339)
		}
	}
	if p.Owner != nil && p.Owner.User != nil {
		item.Author = p.Owner.User.DisplayName
		if item.Author == "" {
			item.Author = p.Owner.User.Username
		}
	}

	return item
}

// pageURL builds the browser URL for a slug. The API returns slugs, not links,
// but a clickable address is what a human reading table output wants.
func pageURL(slug string) string {
	if slug == "" {
		return ""
	}

	return "https://wiki.yandex.ru/" + slug
}

// NewCmd creates the parent "page" command with its subcommands.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "page",
		Short: "Manage wiki pages",
		Long: `Read, create, update, and delete Yandex Wiki pages.

Every command accepts a page either as a slug (users/me/notes) or as a
numeric page ID.`,
	}

	cmd.AddCommand(
		newGetCmd(),
		newListCmd(),
		newCreateCmd(),
		newEditCmd(),
		newAppendCmd(),
		newDeleteCmd(),
		newCloneCmd(),
	)

	return cmd
}
