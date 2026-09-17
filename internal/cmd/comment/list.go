package comment

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	"github.com/cloud-yyy/ywiki/internal/cmd/jsonfields"
	"github.com/cloud-yyy/ywiki/internal/output"
)

const (
	// commentReservedWidth is the space taken by the ID, author, and date
	// columns plus padding in table output.
	commentReservedWidth = 44

	// commentMinBodyWidth keeps the body column readable on narrow terminals.
	commentMinBodyWidth = 10
)

func itoa(n int) string { return strconv.Itoa(n) }

func newListCmd() *cobra.Command {
	var (
		limit    int
		cursor   string
		threadID int
	)

	cmd := &cobra.Command{
		Use:   "list <slug|id>",
		Short: "List comments on a page",
		Long: `List the comments on a Yandex Wiki page.

Pass --thread to list the replies in one thread instead of the top-level
comments.

JSON FIELDS
  id, body, author, createdAt, threadId, parentId, resolveStatus`,
		Example: `  # List comments on a page
  ywiki comment list users/me/notes

  # List comments as JSON
  ywiki comment list users/me/notes --json id,author,body

  # List the replies in a thread
  ywiki comment list users/me/notes --thread 42`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, args[0], threadID, api.ListCommentsOptions{
				PageSize: limit,
				Cursor:   cursor,
			})
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 0, "Results per page")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Continue a previous listing")
	cmd.Flags().IntVar(&threadID, "thread", 0, "List replies in this comment thread")

	jsonfields.Register("ywiki comment list", CommentFields)

	return cmd
}

func runList(cmd *cobra.Command, ref string, threadID int, opts api.ListCommentsOptions) error {
	if err := cmdutil.PrepareFields(cmd, "comment list", CommentFields); err != nil {
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

	var result *api.ListResult[api.Comment]
	if threadID > 0 {
		result, err = client.ListThread(cmd.Context(), pageID, threadID, opts)
	} else {
		result, err = client.ListComments(cmd.Context(), pageID, opts)
	}
	if err != nil {
		return err
	}

	items := make([]commentItem, len(result.Items))
	for i := range result.Items {
		items[i] = toCommentItem(&result.Items[i])
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
		_, err := fmt.Fprintln(w, "No comments found")
		return err
	}

	tbl := output.NewTable(w)
	tbl.AddHeader("ID", "AUTHOR", "DATE", "BODY")
	bodyWidth := max(output.TerminalWidth()-commentReservedWidth, commentMinBodyWidth)
	for i, item := range items {
		date := "-"
		if !result.Items[i].CreatedAt.IsZero() {
			date = output.TimeAgo(result.Items[i].CreatedAt)
		}
		tbl.AddRow(item.ID, item.Author, date, output.TruncateDisplay(item.Body, bodyWidth))
	}
	tbl.Render()

	if result.NextCursor != "" {
		_, _ = fmt.Fprintf(w, "\nMore results available. Continue with: --cursor %s\n", result.NextCursor)
	}

	return nil
}
