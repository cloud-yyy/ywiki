package cmdutil

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
)

// ReadBody resolves page or comment content from --body or --body-file.
// A body file of "-" reads stdin, so content can be piped in. It returns
// (content, provided): provided is false when neither flag was given, letting
// update commands distinguish "leave unchanged" from "set to empty".
func ReadBody(cmd *cobra.Command, body, bodyFile string) (string, bool, error) {
	bodySet := cmd.Flags().Changed("body")
	fileSet := cmd.Flags().Changed("body-file")

	switch {
	case bodySet && fileSet:
		return "", false, wikierrors.NewUserError(
			"--body and --body-file are mutually exclusive",
			"Pass the content with one of them, not both",
		)
	case bodySet:
		return body, true, nil
	case !fileSet:
		return "", false, nil
	}

	if bodyFile == "-" {
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", false, fmt.Errorf("failed to read content from stdin: %w", err)
		}
		return strings.TrimRight(string(data), "\n"), true, nil
	}

	data, err := os.ReadFile(bodyFile)
	if err != nil {
		return "", false, wikierrors.NewUserError(
			fmt.Sprintf("failed to read %s: %v", bodyFile, err),
			"Check the path, or pass - to read from stdin",
		)
	}

	return strings.TrimRight(string(data), "\n"), true, nil
}
