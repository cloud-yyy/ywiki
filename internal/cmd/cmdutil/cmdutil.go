// Package cmdutil holds the plumbing every ywiki command repeats: resolving
// credentials into a client, preparing --json field selection, and rendering
// a result in the active output mode.
package cmdutil

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/config"
	"github.com/cloud-yyy/ywiki/internal/output"
)

// NewClient builds an API client from resolved credentials. Tests replace it
// to point the command tree at a stub server.
var NewClient = func(auth *config.ResolvedAuth) *api.Client {
	return api.NewClient(auth)
}

// Client resolves credentials from the root persistent flags, environment, and
// config file, then returns a ready API client.
func Client(cmd *cobra.Command) (*api.Client, error) {
	tokenFlag, _ := cmd.Root().PersistentFlags().GetString("token")
	orgIDFlag, _ := cmd.Root().PersistentFlags().GetString("org-id")
	orgTypeFlag, _ := cmd.Root().PersistentFlags().GetString("org-type")

	auth, err := config.ResolveAuth(tokenFlag, orgIDFlag, orgTypeFlag)
	if err != nil {
		return nil, err
	}

	return NewClient(auth), nil
}

// PrepareFields validates and normalizes the --json field selection for a
// command, and prints the available-fields hint when --json was given with no
// field names. A bare --jq is left alone: the filter then runs against the
// full response, including the pagination envelope on list commands, which is
// the only way an expression can reach the next cursor.
func PrepareFields(cmd *cobra.Command, commandName string, allowed []string) error {
	if output.WantsFieldHint(cmd.Flags().Changed("json")) {
		return output.PrintFieldHint(cmd.ErrOrStderr(), commandName, allowed)
	}

	if output.HasFieldSelection() {
		if err := output.ValidateFields(output.JSONFields, allowed); err != nil {
			return err
		}
		output.JSONFields = output.NormalizeFields(output.JSONFields, allowed)
	}

	return nil
}

// RenderJSON writes a single item in JSON mode, applying field selection and
// any --jq filter.
func RenderJSON(w io.Writer, item any) error {
	if !output.HasFieldSelection() {
		if output.JQFilter != "" {
			return output.ApplyJQ(w, item, output.JQFilter)
		}
		return output.PrintJSON(w, item)
	}

	filtered := output.FilterFields(item, output.JSONFields)
	if output.JQFilter != "" {
		return output.ApplyJQ(w, filtered, output.JQFilter)
	}

	return output.PrintJSON(w, filtered)
}

// RenderList writes a slice of items in JSON mode. Unfiltered output is
// wrapped in the pagination envelope so callers can follow the cursor, while
// --json field selection keeps returning a plain array, which is what scripts
// index into.
func RenderList[T any](w io.Writer, items []T, nextCursor string) error {
	if output.HasFieldSelection() {
		filtered := make([]map[string]any, len(items))
		for i, item := range items {
			filtered[i] = output.FilterFields(item, output.JSONFields)
		}
		if output.JQFilter != "" {
			return output.ApplyJQ(w, filtered, output.JQFilter)
		}
		return output.PrintJSON(w, filtered)
	}

	result := output.PaginatedResult{
		Items: items,
		Pagination: output.PaginationMeta{
			Cursor:  nextCursor,
			HasMore: nextCursor != "",
		},
	}

	if output.JQFilter != "" {
		return output.ApplyJQ(w, result, output.JQFilter)
	}

	return output.PrintJSON(w, result)
}
