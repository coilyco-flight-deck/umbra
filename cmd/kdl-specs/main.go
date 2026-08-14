// Command kdl-specs is the compatibility entrypoint for consumers that have
// not migrated their Go invocation path to cmd/specgen.
package main

import (
	"context"
	"os"

	"forgejo.coilysiren.me/coilyco-flight-deck/umbra/internal/specgencli"
)

func main() {
	if code := specgencli.Run(context.Background(), os.Args, os.Stdout, os.Stderr); code != 0 {
		os.Exit(code)
	}
}
