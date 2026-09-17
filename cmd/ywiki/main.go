// Command ywiki is a command-line client for Yandex Wiki.
package main

import (
	"os"

	"github.com/cloud-yyy/ywiki/internal/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
