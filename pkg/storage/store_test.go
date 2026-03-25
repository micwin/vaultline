package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/micwin/mono-repo/vaultline/pkg/storage"
)

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.New(dir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if !store.Sealed() {
		t.Fatalf("new store should be sealed")
	}
	if err := store.Unseal("passphrase"); err != nil {
		t.Fatalf("unseal: %v", err)
	}
	version, err := store.Put("app.api-key", []byte("top-secret"))
	if err != nil {
		t.Fatalf("put secret: %v", err)
	}
	secret, err := store.Get("app.api-key")
	if err != nil {
		t.Fatalf("get secret: %v", err)
	}
	if string(secret.Data) != "top-secret" {
		t.Fatalf("unexpected secret payload: %s", string(secret.Data))
	}
	if secret.Version != version {
		t.Fatalf("expected version %s got %s", version, secret.Version)
	}
}

func TestStoreDelete(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.New(dir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if err := store.Unseal("passphrase"); err != nil {
		t.Fatalf("unseal: %v", err)
	}
	if _, err := store.Put("ops.token", []byte("value")); err != nil {
		t.Fatalf("put secret: %v", err)
	}
	if err := store.Delete("ops.token"); err != nil {
		t.Fatalf("delete secret: %v", err)
	}
	if _, err := store.Get("ops.token"); err == nil {
		t.Fatalf("expected error after deletion")
	}
}

func TestStorePersistsSalt(t *testing.T) {
	dir := t.TempDir()
	store1, err := storage.New(dir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if err := store1.Unseal("passphrase"); err != nil {
		t.Fatalf("unseal: %v", err)
	}
	if _, err := store1.Put("ops.token", []byte("value")); err != nil {
		t.Fatalf("put: %v", err)
	}
	store1.Seal()

	store2, err := storage.New(dir)
	if err != nil {
		t.Fatalf("recreate store: %v", err)
	}
	if err := store2.Unseal("passphrase"); err != nil {
		t.Fatalf("unseal second: %v", err)
	}
	secret, err := store2.Get("ops.token")
	if err != nil {
		t.Fatalf("get secret: %v", err)
	}
	if string(secret.Data) != "value" {
		t.Fatalf("unexpected payload after reopening: %s", string(secret.Data))
	}
	if _, err := os.Stat(filepath.Join(dir, "secrets", "ops.token.vlx")); err != nil {
		t.Fatalf("secret file missing: %v", err)
	}
}
