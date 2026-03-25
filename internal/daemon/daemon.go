package daemon

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/micwin/vaultline/internal/server"
	"github.com/micwin/vaultline/pkg/storage"
)

// Run starts the vaultline daemon on the provided addr/storeDir and blocks until shutdown.
func Run(addr, storeDir, sealFile, version string) error {
	store, err := storage.New(storeDir)
	if err != nil {
		return err
	}

	switch {
	case sealFile != "":
		passphrase, created, err := ensureSealFile(sealFile)
		if err != nil {
			return err
		}
		if err := store.Unseal(passphrase); err != nil {
			return err
		}
		if created {
			log.Printf("created seal file at %s", sealFile)
		} else {
			log.Printf("auto-unsealed via seal file %s", sealFile)
		}
	case os.Getenv("VAULTLINE_PASSPHRASE") != "":
		pass := os.Getenv("VAULTLINE_PASSPHRASE")
		if err := store.Unseal(pass); err != nil {
			return err
		}
		log.Println("vaultline auto-unsealed via VAULTLINE_PASSPHRASE")
	}

	apiServer := server.New(store, version)
	httpServer := &http.Server{Addr: addr, Handler: apiServer.Handler()}

	go func() {
		log.Printf("vaultline listening on %s (store: %s)", addr, storeDir)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server: %v", err)
		}
	}()

	waitForSignal()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		return err
	}
	log.Println("vaultline stopped")
	return nil
}

func waitForSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
}

func ensureSealFile(sealPath string) (string, bool, error) {
	dir := filepath.Dir(sealPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", false, err
	}
	data, err := os.ReadFile(sealPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			passphrase, err := generatePassphrase()
			if err != nil {
				return "", false, err
			}
			if err := os.WriteFile(sealPath, []byte(passphrase+"\n"), 0o600); err != nil {
				return "", false, err
			}
			return passphrase, true, nil
		}
		return "", false, err
	}
	return strings.TrimSpace(string(data)), false, nil
}

func generatePassphrase() (string, error) {
	buf := make([]byte, 48)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}
