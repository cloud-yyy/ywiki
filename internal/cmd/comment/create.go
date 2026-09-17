package comment

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	"github.com/cloud-yyy/ywiki/internal/cmd/jsonfields"
	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
	"github.com/cloud-yyy/ywiki/internal/output"
)

func newCreateCmd() *cobra.Command {
	var (
		body     string
		bodyFile string
		replyTo  int
	)

	cmd := &cobra.Command{
		Use:   "create <slug|id>",
		Short: "Add a comment to a page",
		Long: `Add a comment to a Yandex Wiki page.

Pass --reply-to to answer an existing comment instead of starting a new thread.

JSON FIELDS
  id, body, author, createdAt, threadId, parentId, resolveStatus`,
		Example: `  # Comment on a page
  ywiki comment create users/me/notes --body "Reviewed, looks good"

  # Reply to a comment
  ywiki comment create users/me/notes --body "Agreed" --reply-to 42

  # Comment with content from stdin
  echo "See the build log" | ywiki comment create 12345 --body-file -`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			content, contentSet, err := cmdutil.ReadBody(cmd, body, bodyFile)
			if err != nil {
				return err
			}
			if !contentSet || content == "" {
				return wikierrors.NewUserError(
					"empty comment",
					"Pass --body, or --body-file (use - for stdin)",
				)
			}

			in := api.CreateCommentInput{Body: content}
			if replyTo > 0 {
				in.ParentID = &replyTo
			}

			return runCreate(cmd, args[0], in)
		},
	}

	cmd.Flags().StringVar(&body, "body", "", "Comment text")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "Read comment text from a file, or - for stdin")
	cmd.Flags().IntVar(&replyTo, "reply-to", 0, "ID of the comment being replied to")
	cmd.MarkFlagsMutuallyExclusive("body", "body-file")

	jsonfields.Register("ywiki comment create", CommentFields)

	return cmd
}

func runCreate(cmd *cobra.Command, ref string, in api.CreateCommentInput) error {
	if err := cmdutil.PrepareFields(cmd, "comment create", CommentFields); err != nil {
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

	comment, err := client.CreateComment(cmd.Context(), pageID, in)
	if err != nil {
		return err
	}

	item := toCommentItem(comment)
	w := cmd.OutOrStdout()

	if output.IsJSON() {
		return cmdutil.RenderJSON(w, item)
	}

	if output.IsQuiet() {
		output.PrintQuiet(w, item.ID)
		return nil
	}

	_, err = fmt.Fprintf(w, "Added comment %s\n", item.ID)

	return err
}
