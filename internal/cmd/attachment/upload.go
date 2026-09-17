package attachment

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	"github.com/cloud-yyy/ywiki/internal/cmd/jsonfields"
	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
	"github.com/cloud-yyy/ywiki/internal/output"
)

func newUploadCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "upload <slug|id> <file>",
		Short: "Attach a file to a page",
		Long: `Attach a local file to a Yandex Wiki page.

The file is uploaded in parts and then attached, which is what the API
requires; the command handles the whole sequence.

JSON FIELDS
  id, name, size, mimetype, author, createdAt`,
		Example: `  # Attach a file
  ywiki attachment upload users/me/notes report.pdf

  # Attach a file under a different name
  ywiki attachment upload users/me/notes report.pdf --name q1-report.pdf`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpload(cmd, args[0], args[1], name)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name to store the file under")

	jsonfields.Register("ywiki attachment upload", AttachmentFields)

	return cmd
}

func runUpload(cmd *cobra.Command, ref, path, name string) error {
	if err := cmdutil.PrepareFields(cmd, "attachment upload", AttachmentFields); err != nil {
		return err
	}

	file, err := os.Open(path)
	if err != nil {
		return wikierrors.NewUserError(
			fmt.Sprintf("failed to open %s: %v", path, err),
			"Check the file path",
		)
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat %s: %w", path, err)
	}
	if info.IsDir() {
		return wikierrors.NewUserError(
			path+" is a directory",
			"Pass a single file to upload",
		)
	}

	if name == "" {
		name = filepath.Base(path)
	}

	client, err := cmdutil.Client(cmd)
	if err != nil {
		return err
	}

	pageID, err := client.ResolvePageID(cmd.Context(), api.ParsePageLocator(ref))
	if err != nil {
		return err
	}

	attachments, err := client.UploadAttachment(cmd.Context(), pageID, name, info.Size(), file)
	if err != nil {
		return err
	}

	items := make([]attachmentItem, len(attachments))
	for i := range attachments {
		items[i] = toItem(&attachments[i])
	}

	w := cmd.OutOrStdout()

	if output.IsJSON() {
		return cmdutil.RenderList(w, items, "")
	}

	if output.IsQuiet() {
		for _, item := range items {
			output.PrintQuiet(w, item.ID)
		}
		return nil
	}

	_, err = fmt.Fprintf(w, "Attached %s\n", name)

	return err
}
