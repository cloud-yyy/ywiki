package page

import (
	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	"github.com/cloud-yyy/ywiki/internal/cmd/jsonfields"
	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
)

func newEditCmd() *cobra.Command {
	var (
		title      string
		body       string
		bodyFile   string
		allowMerge bool
		silent     bool
	)

	cmd := &cobra.Command{
		Use:   "edit <slug|id>",
		Short: "Update a page",
		Long: `Update the title or content of a Yandex Wiki page.

Only the fields you pass are changed. Replacing content overwrites the whole
body; use "ywiki page append" to add to it instead.

If someone else edited the page since you read it, the update fails with a
conflict. Pass --allow-merge to let the API merge both edits.

JSON FIELDS
  id, slug, title, type, content, createdAt, modifiedAt, author, url`,
		Example: `  # Rename a page
  ywiki page edit users/me/notes --title "Release notes"

  # Replace content from a file
  ywiki page edit users/me/notes --body-file notes.md

  # Replace content from stdin, merging concurrent edits
  cat notes.md | ywiki page edit 12345 --body-file - --allow-merge`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			content, contentSet, err := cmdutil.ReadBody(cmd, body, bodyFile)
			if err != nil {
				return err
			}

			in := api.UpdatePageInput{}
			if cmd.Flags().Changed("title") {
				in.Title = &title
			}
			if contentSet {
				in.Content = &content
			}
			if in.Title == nil && in.Content == nil {
				return wikierrors.NewUserError(
					"nothing to update",
					"Pass --title, --body, or --body-file",
				)
			}

			return runEdit(cmd, args[0], in, api.UpdatePageOptions{
				AllowMerge: allowMerge,
				Silent:     silent,
			})
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "New page title")
	cmd.Flags().StringVar(&body, "body", "", "New page content")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "Read new content from a file, or - for stdin")
	cmd.Flags().BoolVar(&allowMerge, "allow-merge", false, "Merge concurrent edits instead of failing")
	cmd.Flags().BoolVar(&silent, "silent", false, "Do not notify subscribers")
	cmd.MarkFlagsMutuallyExclusive("body", "body-file")

	jsonfields.Register("ywiki page edit", PageFields)

	return cmd
}

func runEdit(
	cmd *cobra.Command, ref string, in api.UpdatePageInput, opts api.UpdatePageOptions,
) error {
	if err := cmdutil.PrepareFields(cmd, "page edit", PageFields); err != nil {
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

	page, err := client.UpdatePage(cmd.Context(), id, in, opts)
	if err != nil {
		return err
	}

	return renderPageResult(cmd.OutOrStdout(), page, "Updated")
}
