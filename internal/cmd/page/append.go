package page

import (
	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	"github.com/cloud-yyy/ywiki/internal/cmd/jsonfields"
	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
)

func newAppendCmd() *cobra.Command {
	var (
		body     string
		bodyFile string
		location string
		silent   bool
	)

	cmd := &cobra.Command{
		Use:   "append <slug|id>",
		Short: "Add content to a page",
		Long: `Append content to a Yandex Wiki page without rewriting its body.

This is the safe way to add to a page from a script: it cannot clobber edits
made by someone else between a read and a write.

JSON FIELDS
  id, slug, title, type, content, createdAt, modifiedAt, author, url`,
		Example: `  # Append a line
  ywiki page append users/me/log --body "Deployed v1.2.0"

  # Prepend a changelog entry from stdin
  git log -1 --oneline | ywiki page append users/me/log --body-file - --location top`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			content, contentSet, err := cmdutil.ReadBody(cmd, body, bodyFile)
			if err != nil {
				return err
			}
			if !contentSet {
				return wikierrors.NewUserError(
					"no content to append",
					"Pass --body, or --body-file (use - for stdin)",
				)
			}
			if location != "top" && location != "bottom" {
				return wikierrors.NewUserError(
					"invalid location: "+location,
					"Use --location top or --location bottom",
				)
			}

			return runAppend(cmd, args[0], api.AppendContentInput{
				Content: content,
				Body:    &api.AppendPosition{Location: location},
			}, silent)
		},
	}

	cmd.Flags().StringVar(&body, "body", "", "Content to append")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "Read content from a file, or - for stdin")
	cmd.Flags().StringVar(&location, "location", "bottom", "Where to add content: top or bottom")
	cmd.Flags().BoolVar(&silent, "silent", false, "Do not notify subscribers")
	cmd.MarkFlagsMutuallyExclusive("body", "body-file")

	jsonfields.Register("ywiki page append", PageFields)

	return cmd
}

func runAppend(cmd *cobra.Command, ref string, in api.AppendContentInput, silent bool) error {
	if err := cmdutil.PrepareFields(cmd, "page append", PageFields); err != nil {
		return err
	}

	client, err := cmdutil.Client(cmd)
	if err != nil {
		return err
	}

	id, err := client.ResolvePageID(cmd.Context(), api.ParsePageLocator(ref))
	if err != nil {
		return err
	}

	page, err := client.AppendContent(cmd.Context(), id, in, silent)
	if err != nil {
		return err
	}

	return renderPageResult(cmd.OutOrStdout(), page, "Updated")
}
