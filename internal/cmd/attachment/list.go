package attachment

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	"github.com/cloud-yyy/ywiki/internal/cmd/jsonfields"
	"github.com/cloud-yyy/ywiki/internal/output"
)

func toItem(a *api.Attachment) attachmentItem {
	item := attachmentItem{
		ID:       strconv.Itoa(a.ID),
		Name:     a.Name,
		Size:     a.Size,
		Mimetype: a.Mimetype,
	}

	if a.User != nil {
		item.Author = a.User.DisplayName
		if item.Author == "" {
			item.Author = a.User.Username
		}
	}
	if !a.CreatedAt.IsZero() {
		item.CreatedAt = a.CreatedAt.Format(time.RFC3339)
	}

	return item
}

func newListCmd() *cobra.Command {
	var (
		limit  int
		cursor string
	)

	cmd := &cobra.Command{
		Use:   "list <slug|id>",
		Short: "List files attached to a page",
		Long: `List the files attached to a Yandex Wiki page.

JSON FIELDS
  id, name, size, mimetype, author, createdAt`,
		Example: `  # List attachments
  ywiki attachment list users/me/notes

  # List attachment names only
  ywiki attachment list users/me/notes --json name --jq '.[].name'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, args[0], api.ListAttachmentsOptions{
				PageSize: limit,
				Cursor:   cursor,
			})
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 0, "Results per page")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Continue a previous listing")

	jsonfields.Register("ywiki attachment list", AttachmentFields)

	return cmd
}

func runList(cmd *cobra.Command, ref string, opts api.ListAttachmentsOptions) error {
	if err := cmdutil.PrepareFields(cmd, "attachment list", AttachmentFields); err != nil {
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

	result, err := client.ListAttachments(cmd.Context(), pageID, opts)
	if err != nil {
		return err
	}

	items := make([]attachmentItem, len(result.Items))
	for i := range result.Items {
		items[i] = toItem(&result.Items[i])
	}

	w := cmd.OutOrStdout()

	if output.IsJSON() {
		return cmdutil.RenderList(w, items, result.NextCursor)
	}

	if output.IsQuiet() {
		names := make([]string, len(items))
		for i, item := range items {
			names[i] = item.Name
		}
		output.PrintQuiet(w, names...)
		return nil
	}

	if len(items) == 0 {
		_, err := fmt.Fprintln(w, "No attachments found")
		return err
	}

	tbl := output.NewTable(w)
	tbl.AddHeader("ID", "NAME", "SIZE", "TYPE")
	for _, item := range items {
		tbl.AddRow(item.ID, item.Name, item.Size, item.Mimetype)
	}
	tbl.Render()

	return nil
}
