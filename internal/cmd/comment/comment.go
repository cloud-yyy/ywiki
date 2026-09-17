// Package comment provides page comment commands for the ywiki CLI.
package comment

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
)

// CommentFields lists the available JSON field names for comment output.
var CommentFields = []string{
	"id", "body", "author", "createdAt", "threadId", "parentId", "resolveStatus",
}

type commentItem struct {
	ID            string `json:"id"`
	Body          string `json:"body"`
	Author        string `json:"author,omitempty"`
	CreatedAt     string `json:"createdAt,omitempty"`
	ThreadID      string `json:"threadId,omitempty"`
	ParentID      string `json:"parentId,omitempty"`
	ResolveStatus string `json:"resolveStatus,omitempty"`
}

func toCommentItem(c *api.Comment) commentItem {
	item := commentItem{
		ID:            itoa(c.ID),
		Body:          c.Body,
		ResolveStatus: c.ResolveStatus,
	}

	if c.Author != nil {
		item.Author = c.Author.DisplayName
		if item.Author == "" {
			item.Author = c.Author.Username
		}
	}
	if !c.CreatedAt.IsZero() {
		item.CreatedAt = c.CreatedAt.Format(time.RFC3339)
	}
	if c.ThreadID != nil {
		item.ThreadID = itoa(*c.ThreadID)
	}
	if c.ParentID != nil {
		item.ParentID = itoa(*c.ParentID)
	}

	return item
}

// NewCmd creates the parent "comment" command with its subcommands.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment",
		Short: "Manage page comments",
		Long:  "List, add, and delete comments on Yandex Wiki pages.",
	}

	cmd.AddCommand(
		newListCmd(),
		newCreateCmd(),
		newDeleteCmd(),
	)

	return cmd
}
