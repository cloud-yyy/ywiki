package attachment

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
)

func newDownloadCmd() *cobra.Command {
	var out string

	cmd := &cobra.Command{
		Use:   "download <slug|id> <file-id>",
		Short: "Download an attached file",
		Long: `Download a file attached to a Yandex Wiki page.

The file is written to stdout unless --out names a destination, so it can be
piped straight into another tool.`,
		Example: `  # Save an attachment to a file
  ywiki attachment download users/me/notes 987 --out report.pdf

  # Pipe an attachment to another tool
  ywiki attachment download users/me/notes 987 | wc -c`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			fileID, err := strconv.Atoi(args[1])
			if err != nil || fileID <= 0 {
				return wikierrors.NewUserError(
					"invalid file ID: "+args[1],
					"Pass the numeric file ID shown by: ywiki attachment list <page>",
				)
			}

			return runDownload(cmd, args[0], fileID, out)
		},
	}

	cmd.Flags().StringVarP(&out, "out", "o", "", "Write the file here instead of stdout")

	return cmd
}

func runDownload(cmd *cobra.Command, ref string, fileID int, out string) error {
	client, err := cmdutil.Client(cmd)
	if err != nil {
		return err
	}

	pageID, err := client.ResolvePageID(cmd.Context(), api.ParsePageLocator(ref))
	if err != nil {
		return err
	}

	body, err := client.DownloadAttachment(cmd.Context(), pageID, fileID)
	if err != nil {
		return err
	}
	defer func() { _ = body.Close() }()

	dst := cmd.OutOrStdout()
	if out != "" {
		file, createErr := os.Create(out)
		if createErr != nil {
			return wikierrors.NewUserError(
				fmt.Sprintf("failed to create %s: %v", out, createErr),
				"Check the destination path and permissions",
			)
		}
		defer func() { _ = file.Close() }()
		dst = file
	}

	if _, copyErr := io.Copy(dst, body); copyErr != nil {
		return fmt.Errorf("failed to download file: %w", copyErr)
	}

	if out != "" {
		_, err = fmt.Fprintf(cmd.ErrOrStderr(), "Saved %s\n", out)
	}

	return err
}
