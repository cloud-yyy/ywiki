// Package attachment provides page attachment commands for the ywiki CLI.
package attachment

import (
	"github.com/spf13/cobra"
)

// AttachmentFields lists the available JSON field names for attachment output.
var AttachmentFields = []string{"id", "name", "size", "mimetype", "author", "createdAt"}

type attachmentItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Size      string `json:"size,omitempty"`
	Mimetype  string `json:"mimetype,omitempty"`
	Author    string `json:"author,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// NewCmd creates the parent "attachment" command with its subcommands.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "attachment",
		Aliases: []string{"attach"},
		Short:   "Manage page attachments",
		Long:    "List, upload, download, and delete files attached to Yandex Wiki pages.",
	}

	cmd.AddCommand(
		newListCmd(),
		newUploadCmd(),
		newDownloadCmd(),
		newDeleteCmd(),
	)

	return cmd
}
