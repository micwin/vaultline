package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/micwin/vaultline/internal/daemon"
	"github.com/micwin/vaultline/pkg/cli"
	"github.com/micwin/vaultline/pkg/version"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "daemon" && shouldStartDaemon(os.Args[2:]) {
		fmt.Printf("vaultline %s\n", version.Version)
		daemonFlags := flag.NewFlagSet("vaultline daemon", flag.ExitOnError)
		addr := daemonFlags.String("addr", "127.0.0.1:8428", "listen address")
		storeDir := daemonFlags.String("store-dir", "", "store directory")
		sealFile := daemonFlags.String("seal-file", "", "path to seal file for auto-unseal")
		configFile := daemonFlags.String("config-file", "", "store registry config file")
		daemonConfigFile := daemonFlags.String("daemon-config-file", "", "daemon bind/allow config file")
		_ = daemonFlags.Parse(os.Args[2:])
		storePath := *storeDir
		if storePath == "" {
			storePath = resolveDefaultStore()
		}
		cfgPath := *configFile
		if cfgPath == "" {
			cfgPath = resolveDefaultStoreConfig()
		}
		daemonCfgPath := *daemonConfigFile
		if daemonCfgPath == "" {
			daemonCfgPath = resolveDefaultDaemonConfig()
		}
		if err := daemon.Run(*addr, storePath, *sealFile, cfgPath, daemonCfgPath, version.Version); err != nil {
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

func shouldStartDaemon(args []string) bool {
	if len(args) == 0 {
		return true
	}
	first := args[0]
	return strings.HasPrefix(first, "-")
}

func resolveDefaultStore() string {
	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
		return filepath.Join(xdgData, "vaultline", "store")
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "share", "vaultline", "store")
}

func resolveDefaultStoreConfig() string {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "vaultline", "stores.json")
	}
	return filepath.Join(os.Getenv("HOME"), ".config", "vaultline", "stores.json")
}

func resolveDefaultDaemonConfig() string {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "vaultline", "daemon.json")
	}
	return filepath.Join(os.Getenv("HOME"), ".config", "vaultline", "daemon.json")
}
