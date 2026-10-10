package grid

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	"github.com/cloud-yyy/ywiki/internal/cmd/jsonfields"
	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
	"github.com/cloud-yyy/ywiki/internal/output"
)

// CreateFields lists the available JSON field names for table creation.
var CreateFields = []string{"id", fieldTitle, fieldRevision, "embedded"}

type createItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Revision string `json:"revision"`
	Embedded bool   `json:"embedded"`
}

// embedTag is the page markup that displays a table.
func embedTag(id string) string {
	return fmt.Sprintf("{%% wgrid id=%q %%}", id)
}

func newCreateCmd() *cobra.Command {
	var (
		title string
		embed bool
		cols  columnFlags
	)

	cmd := &cobra.Command{
		Use:   "create <slug|id>",
		Short: "Create a table on a page",
		Long: `Create a dynamic table on a Yandex Wiki page.

Define the columns with --column slug:type[:title], repeated per column. The
title defaults to the slug. Column types: string, number, date, select, staff,
checkbox, ticket, ticket_field. For options the short form cannot express
(select options, widths, colors), pass full definitions with --columns-file.

A new table is not shown in the page until a {% wgrid id="..." %} tag
references it. Pass --embed to append that tag to the end of the page.

JSON FIELDS
  id, title, revision, embedded`,
		Example: `  # Create a table with two columns and show it on the page
  ywiki grid create users/me/notes --title Tasks \
    --column name:string:Name --column done:checkbox:Done --embed

  # Columns from a file
  ywiki grid create users/me/notes --title Tasks --columns-file columns.json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(title) == "" {
				return wikierrors.NewUserError("missing table title", "Pass the name with --title")
			}

			columns, err := cols.columns(cmd)
			if err != nil {
				return err
			}

			return runCreate(cmd, args[0], strings.TrimSpace(title), columns, embed)
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "Table title (required)")
	cmd.Flags().BoolVar(&embed, "embed", false, "Append the table to the page body")
	cols.register(cmd)

	jsonfields.Register("ywiki grid create", CreateFields)

	return cmd
}

func runCreate(cmd *cobra.Command, ref, title string, columns []api.GridColumn, embed bool) error {
	if err := cmdutil.PrepareFields(cmd, "grid create", CreateFields); err != nil {
		return err
	}

	client, err := cmdutil.Client(cmd)
	if err != nil {
		return err
	}

	locator := api.ParsePageLocator(ref)

	g, err := client.CreateGrid(cmd.Context(), title, locator)
	if err != nil {
		return err
	}

	id := g.ID.String()
	item := createItem{ID: id, Title: g.Title, Revision: g.Revision}

	if len(columns) > 0 {
		item.Revision, err = client.AddGridColumns(cmd.Context(), id, columns, nil)
		if err != nil {
			return createdThen(err, id, "add the columns with: ywiki grid column add "+id+" ...")
		}
	}

	if embed {
		pageID, resolveErr := client.ResolvePageID(cmd.Context(), locator)
		if resolveErr != nil {
			return createdThen(resolveErr, id, "add "+embedTag(id)+" to the page yourself")
		}

		_, appendErr := client.AppendContent(cmd.Context(), pageID, api.AppendContentInput{
			Content: embedTag(id),
			Body:    &api.AppendPosition{Location: "bottom"},
		}, false)
		if appendErr != nil {
			return createdThen(appendErr, id, "add "+embedTag(id)+" to the page yourself")
		}
		item.Embedded = true
	}

	w := cmd.OutOrStdout()

	if output.IsJSON() {
		return cmdutil.RenderJSON(w, item)
	}

	if output.IsQuiet() {
		output.PrintQuiet(w, item.ID)
		return nil
	}

	_, err = fmt.Fprintf(w, "Created table %s\n", item.ID)

	return err
}

// createdThen reports a failure that happened after the table was already
// created, so the caller knows the table exists and how to finish the job
// instead of creating a duplicate.
func createdThen(err error, id, next string) error {
	exitErr, ok := errors.AsType[*wikierrors.ExitError](err)
	if !ok {
		return fmt.Errorf("table %s was created, but a later step failed: %w", id, err)
	}

	wrapped := *exitErr
	wrapped.Message = fmt.Sprintf("table %s was created, but a later step failed: %s", id, exitErr.Message)
	wrapped.Suggestion = "To finish, " + next

	return &wrapped
}
