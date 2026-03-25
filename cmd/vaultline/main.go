package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/micwin/mono-repo/vaultline/internal/daemon"
	"github.com/micwin/mono-repo/vaultline/pkg/cli"
	"github.com/micwin/mono-repo/vaultline/pkg/version"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "daemon" {
		fmt.Printf("vaultline %s\n", version.Version)
		daemonFlags := flag.NewFlagSet("vaultline daemon", flag.ExitOnError)
		addr := daemonFlags.String("addr", "127.0.0.1:8428", "listen address")
		storeDir := daemonFlags.String("store-dir", "", "store directory")
		sealFile := daemonFlags.String("seal-file", "", "path to seal file for auto-unseal")
		_ = daemonFlags.Parse(os.Args[2:])
		storePath := *storeDir
		if storePath == "" {
			storePath = resolveDefaultStore()
		}
		if err := daemon.Run(*addr, storePath, *sealFile, version.Version); err != nil {
			fmt.Fprintln(os.Stderr, "vaultline daemon:", err)
			os.Exit(1)
		}
		return
	}

	if err := cli.Run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "vaultline:", err)
		os.Exit(1)
	}
}

func resolveDefaultStore() string {
	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
		return filepath.Join(xdgData, "vaultline", "store")
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "share", "vaultline", "store")
}
