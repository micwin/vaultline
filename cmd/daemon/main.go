package main

import (
	"flag"
	"log"

	"github.com/micwin/vaultline/internal/daemon"
	"github.com/micwin/vaultline/pkg/version"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8428", "listen address (host:port)")
	storeDir := flag.String("store-dir", "./store", "directory for encrypted secrets")
	sealFile := flag.String("seal-file", "", "path to seal file for auto-unseal")
	flag.Parse()

	if err := daemon.Run(*addr, *storeDir, *sealFile, version.Version); err != nil {
		log.Fatalf("vaultlined: %v", err)
	}
}
