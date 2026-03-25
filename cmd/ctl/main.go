package main

import (
	"fmt"
	"os"

	"github.com/micwin/mono-repo/vaultline/pkg/cli"
)

func main() {
	if err := cli.Run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "vaultlinectl:", err)
		os.Exit(1)
	}
}
