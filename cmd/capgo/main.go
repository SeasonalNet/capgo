// Command capgo reads a CAP XML message and emits machine-readable JSON.
package main

import (
	"os"

	"git.seasonalnet.org/SeasonalNet/capgo/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
