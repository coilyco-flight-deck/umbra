// Command gittree demonstrates the clean+synced gate.
package main

import (
	"context"
	"fmt"
	"os"

	"forgejo.coilysiren.me/coilyco-flight-deck/umbra/examples/treebuilders"
)

func main() {
	if err := treebuilders.Gittree().Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
