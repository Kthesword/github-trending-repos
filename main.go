// Command trending-repos lists trending public GitHub repositories.
//
// See README.md for installation and usage instructions.
package main

import (
	"os"

	"github.com/kthesword/gh-trending-repos/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
