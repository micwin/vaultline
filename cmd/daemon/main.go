package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/micwin/vaultline/internal/daemon"
	"github.com/micwin/vaultline/pkg/version"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8428", "listen address (host:port)")
	storeDir := flag.String("store-dir", "./store", "directory for encrypted secrets")
	sealFile := flag.String("seal-file", "", "path to seal file for auto-unseal")
	configFile := flag.String("config-file", "", "path to store registry config")
	daemonConfigFile := flag.String("daemon-config-file", "", "path to daemon bind/allow config")
	flag.Parse()

	cfgPath := *configFile
	if cfgPath == "" {
		if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
			cfgPath = filepath.Join(xdgConfig, "vaultline", "stores.json")
		} else {
			cfgPath = filepath.Join(os.Getenv("HOME"), ".config", "vaultline", "stores.json")
		}
	}

	daemonCfgPath := *daemonConfigFile
	if daemonCfgPath == "" {
		if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
			daemonCfgPath = filepath.Join(xdgConfig, "vaultline", "daemon.json")
		} else {
			daemonCfgPath = filepath.Join(os.Getenv("HOME"), ".config", "vaultline", "daemon.json")
		}
	}
	if err := daemon.Run(*addr, *storeDir, *sealFile, cfgPath, daemonCfgPath, version.Version); err != nil {
		log.Fatalf("vaultlined: %v", err)
	}
}
