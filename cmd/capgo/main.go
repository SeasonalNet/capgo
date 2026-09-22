// Command capgo validates CAP XML or encodes a JSON CAP message as XML.
package main

import (
	"os"

	"git.seasonalnet.org/SeasonalNet/capgo/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
