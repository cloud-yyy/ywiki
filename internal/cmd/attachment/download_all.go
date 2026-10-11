package attachment

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
	"github.com/cloud-yyy/ywiki/internal/output"
)

// DownloadFields lists the result fields for bulk attachment downloads.
var DownloadFields = []string{"id", "name", "path"}

const (
	downloadFileMode = 0o600
	downloadDirMode  = 0o700
)

type downloadItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

func runDownloadAll(cmd *cobra.Command, ref, dir string) error {
	if err := cmdutil.PrepareFields(cmd, "attachment download --all", DownloadFields); err != nil {
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
	if dir == "" {
		dir = "."
	}
	if mkdirErr := os.MkdirAll(dir, downloadDirMode); mkdirErr != nil {
		return wikierrors.NewUserError(
			"failed to create download directory: "+mkdirErr.Error(),
			"Check the destination path and permissions",
		)
	}
	items, err := downloadAllPages(cmd, client, pageID, dir)
	if err != nil {
		return err
	}
	if output.IsJSON() {
		return cmdutil.RenderList(cmd.OutOrStdout(), items, "")
	}
	if output.IsQuiet() {
		for _, item := range items {
			output.PrintQuiet(cmd.OutOrStdout(), item.Path)
		}
		return nil
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Downloaded %d attachment(s) to %s\n", len(items), dir)
	return err
}

func downloadAllPages(cmd *cobra.Command, client *api.Client, pageID int, dir string) ([]downloadItem, error) {
	items := make([]downloadItem, 0)
	seen := make(map[string]bool)
	opts := api.ListAttachmentsOptions{}
	for {
		result, err := client.ListAttachments(cmd.Context(), pageID, opts)
		if err != nil {
			return nil, err
		}
		for _, file := range result.Items {
			// Treat both slash styles as separators, keeping remote names inside dir.
			name := filepath.Base(strings.ReplaceAll(file.Name, `\`, "/"))
			path := filepath.Join(dir, strconv.Itoa(file.ID)+"-"+name)
			if err := saveAttachment(cmd.Context(), client, pageID, file.ID, path); err != nil {
				return nil, fmt.Errorf("attachment %d (%s): %w", file.ID, file.Name, err)
			}
			items = append(items, downloadItem{ID: strconv.Itoa(file.ID), Name: file.Name, Path: path})
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Saved %s\n", path)
		}
		if result.NextCursor == "" {
			break
		}
		if seen[result.NextCursor] {
			return nil, wikierrors.NewUserError(
				"attachment listing returned a repeated cursor",
				"Completed downloads remain in "+dir,
			)
		}
		seen[result.NextCursor] = true
		opts.Cursor = result.NextCursor
	}

	return items, nil
}

// saveAttachment creates exclusively so existing files and symlinks stay intact.
// A failed read or write removes only the incomplete file created by this call.
func saveAttachment(ctx context.Context, client *api.Client, pageID, fileID int, path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, downloadFileMode)
	if err != nil {
		return wikierrors.NewUserError(
			"failed to create "+path+": "+err.Error(),
			"Use a writable directory without existing download files",
		)
	}
	complete := false
	defer func() {
		_ = file.Close()
		if !complete {
			_ = os.Remove(path)
		}
	}()
	body, err := client.DownloadAttachment(ctx, pageID, fileID)
	if err != nil {
		return err
	}
	defer func() { _ = body.Close() }()
	if _, err := io.Copy(file, body); err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close downloaded file: %w", err)
	}
	complete = true
	return nil
}
