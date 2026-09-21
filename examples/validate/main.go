// Command validate is a compatibility wrapper around the capgo CLI.
package main

import (
	"os"

	"git.seasonalnet.org/SeasonalNet/capgo/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
