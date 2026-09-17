package page

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	"github.com/cloud-yyy/ywiki/internal/cmd/jsonfields"
	"github.com/cloud-yyy/ywiki/internal/output"
)

func newCreateCmd() *cobra.Command {
	var (
		title      string
		body       string
		bodyFile   string
		accessType string
		staffRole  string
		silent     bool
	)

	cmd := &cobra.Command{
		Use:   "create <slug>",
		Short: "Create a page",
		Long: `Create a Yandex Wiki page at the given slug.

Page content comes from --body, or from --body-file (pass - to read stdin).

JSON FIELDS
  id, slug, title, type, content, createdAt, modifiedAt, author, url`,
		Example: `  # Create a page with inline content
  ywiki page create users/me/notes --title Notes --body "First line"

  # Create a page from a file
  ywiki page create users/me/notes --title Notes --body-file notes.md

  # Create a page from stdin
  cat notes.md | ywiki page create users/me/notes --title Notes --body-file -`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			content, _, err := cmdutil.ReadBody(cmd, body, bodyFile)
			if err != nil {
				return err
			}

			in := api.CreatePageInput{
				Slug:    args[0],
				Title:   title,
				Content: content,
			}
			if accessType != "" {
				in.AccessPolicy = &api.AccessPolicyUpdate{
					AccessType:   accessType,
					AllStaffRole: staffRole,
				}
			}

			return runCreate(cmd, in, silent)
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "Page title (required)")
	cmd.Flags().StringVar(&body, "body", "", "Page content")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "Read page content from a file, or - for stdin")
	cmd.Flags().StringVar(&accessType, "access", "",
		"Access type: inherited, all_staff, or custom")
	cmd.Flags().StringVar(&staffRole, "staff-role", "",
		"Role for all_staff access: reader, editor, or extra_editor")
	cmd.Flags().BoolVar(&silent, "silent", false, "Do not notify subscribers")
	_ = cmd.MarkFlagRequired("title")
	cmd.MarkFlagsMutuallyExclusive("body", "body-file")

	jsonfields.Register("ywiki page create", PageFields)

	return cmd
}

func runCreate(cmd *cobra.Command, in api.CreatePageInput, silent bool) error {
	if err := cmdutil.PrepareFields(cmd, "page create", PageFields); err != nil {
		return err
	}

	client, err := cmdutil.Client(cmd)
	if err != nil {
		return err
	}

	page, err := client.CreatePage(cmd.Context(), in, silent)
	if err != nil {
		return err
	}

	return renderPageResult(cmd.OutOrStdout(), page, "Created")
}

// renderPageResult prints a mutated page in the active output mode.
func renderPageResult(w io.Writer, page *api.Page, action string) error {
	item := toPageItem(page)

	if output.IsJSON() {
		return cmdutil.RenderJSON(w, item)
	}

	if output.IsQuiet() {
		output.PrintQuiet(w, item.Slug)
		return nil
	}

	_, err := fmt.Fprintf(w, "%s %s\n%s\n", action, item.Slug, item.URL)

	return err
}
