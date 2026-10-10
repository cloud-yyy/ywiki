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
	return ReadInput(cmd, "body", body, "body-file", bodyFile)
}

// ReadInput resolves text from an inline flag or from a file flag, where a
// file of "-" reads stdin. valueFlag and fileFlag are the flag names, used to
// tell which one the user set and to word errors. The bool result is false
// when neither was given.
func ReadInput(cmd *cobra.Command, valueFlag, value, fileFlag, file string) (string, bool, error) {
	valueSet := cmd.Flags().Changed(valueFlag)
	fileSet := cmd.Flags().Changed(fileFlag)

	switch {
	case valueSet && fileSet:
		return "", false, wikierrors.NewUserError(
			fmt.Sprintf("--%s and --%s are mutually exclusive", valueFlag, fileFlag),
			"Pass the content with one of them, not both",
		)
	case valueSet:
		return value, true, nil
	case !fileSet:
		return "", false, nil
	}

	if file == "-" {
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", false, fmt.Errorf("failed to read content from stdin: %w", err)
		}
		return strings.TrimRight(string(data), "\n"), true, nil
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return "", false, wikierrors.NewUserError(
			fmt.Sprintf("failed to read %s: %v", file, err),
			"Check the path, or pass - to read from stdin",
		)
	}

	return strings.TrimRight(string(data), "\n"), true, nil
}
