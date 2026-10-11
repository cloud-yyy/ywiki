package attachment

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cloud-yyy/ywiki/internal/api"
	"github.com/cloud-yyy/ywiki/internal/cmd/cmdutil"
	"github.com/cloud-yyy/ywiki/internal/cmd/jsonfields"
	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
)

func newDownloadCmd() *cobra.Command {
	var out string
	var all bool

	cmd := &cobra.Command{
		Use:   "download <slug|id> [<file-id>]",
		Short: "Download one or all attached files",
		Long: `Download a file attached to a Yandex Wiki page.

The file is written to stdout unless --out names a destination, so it can be
piped straight into another tool.

With --all, download every attachment across all listing pages to the directory
named by --out (default: the current directory). Files are named <id>-<name>
to distinguish duplicate names. Existing files are never overwritten. On an
error the command stops; completed files remain, and the incomplete file is
removed. JSON output lists id, name, and path; --quiet prints saved paths.`,
		Example: `  # Save an attachment to a file
  ywiki attachment download users/me/notes 987 --out report.pdf

  # Pipe an attachment to another tool
  ywiki attachment download users/me/notes 987 | wc -c

  # Download every attachment into a directory
  ywiki attachment download users/me/notes --all --out attachments`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				if len(args) != 1 {
					return wikierrors.NewUserError("--all cannot be combined with a file ID", "Pass only the page")
				}
				return runDownloadAll(cmd, args[0], out)
			}
			if len(args) != 2 {
				return wikierrors.NewUserError(
					"missing file ID",
					"Pass a file ID, or --all to download every attachment",
				)
			}
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

	cmd.Flags().StringVarP(&out, "out", "o", "", "Destination file, or directory with --all")
	cmd.Flags().BoolVar(&all, "all", false, "Download every attachment to a directory")
	jsonfields.Register("ywiki attachment download", DownloadFields)

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
