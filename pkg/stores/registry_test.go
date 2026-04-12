package stores

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManagerAddInitAndInfo(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "stores.json")
	localPath := filepath.Join(tmp, "local")
	manager, err := NewManager(cfgPath, localPath)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if err := manager.Add("project-a", filepath.Join(tmp, "project-a"), true); err != nil {
		t.Fatalf("add init store: %v", err)
	}
	info, err := manager.Info("project-a")
	if err != nil {
		t.Fatalf("store info: %v", err)
	}
	if !info.Available {
		t.Fatalf("expected store to be available")
	}
	if !info.Sealed {
		t.Fatalf("new store should start sealed")
	}
}

func TestManagerInfoForMissingExternalStoreDoesNotFail(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "stores.json")
	localPath := filepath.Join(tmp, "local")
	manager, err := NewManager(cfgPath, localPath)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	manager.cfg.Stores["project-a"] = Entry{Path: filepath.Join(tmp, "missing-store")}
	if err := SaveConfig(cfgPath, manager.cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	info, err := manager.Info("project-a")
	if err != nil {
		t.Fatalf("store info: %v", err)
	}
	if info.Available {
		t.Fatalf("expected missing external store to be unavailable")
	}
	if info.Error == "" {
		t.Fatalf("expected unavailable store to carry an error message")
	}
}

func TestManagerListIncludesBrokenExternalStore(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "stores.json")
	localPath := filepath.Join(tmp, "local")
	manager, err := NewManager(cfgPath, localPath)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	brokenPath := filepath.Join(tmp, "broken")
	if err := os.MkdirAll(brokenPath, 0o700); err != nil {
		t.Fatalf("mkdir broken store: %v", err)
	}
	if err := os.WriteFile(filepath.Join(brokenPath, ".master_salt"), []byte("not-base64"), 0o600); err != nil {
		t.Fatalf("write broken salt: %v", err)
	}
	manager.cfg.Stores["broken"] = Entry{Path: brokenPath}
	if err := SaveConfig(cfgPath, manager.cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	infos, err := manager.List()
	if err != nil {
		t.Fatalf("list stores: %v", err)
	}
	if len(infos) < 2 {
		t.Fatalf("expected default + broken stores, got %d", len(infos))
	}
	for _, info := range infos {
		if info.Name == "broken" {
			if info.Available {
				t.Fatalf("broken store unexpectedly available")
			}
			if info.Error == "" {
				t.Fatalf("broken store missing error message")
			}
			return
		}
	}
	t.Fatalf("broken store missing from info list")
}

func TestManagerUnsealStoresPassphraseAndSealDropsItByDefault(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "stores.json")
	localPath := filepath.Join(tmp, "local")
	manager, err := NewManager(cfgPath, localPath)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if err := manager.Unseal(DefaultStoreName, "topsecret", true); err != nil {
		t.Fatalf("unseal local: %v", err)
	}
	info, err := manager.Info(DefaultStoreName)
	if err != nil {
		t.Fatalf("local info: %v", err)
	}
	if !info.HasKey {
		t.Fatalf("expected remembered passphrase after unseal")
	}
	if err := manager.Seal(DefaultStoreName, false); err != nil {
		t.Fatalf("seal local: %v", err)
	}
	info, err = manager.Info(DefaultStoreName)
	if err != nil {
		t.Fatalf("local info after seal: %v", err)
	}
	if info.HasKey {
		t.Fatalf("expected passphrase to be dropped on seal without keep-keys")
	}
}

func TestManagerSealKeepsPassphraseWhenRequested(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "stores.json")
	localPath := filepath.Join(tmp, "local")
	manager, err := NewManager(cfgPath, localPath)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if err := manager.Unseal(DefaultStoreName, "keepme", true); err != nil {
		t.Fatalf("unseal local: %v", err)
	}
	if err := manager.Seal(DefaultStoreName, true); err != nil {
		t.Fatalf("seal local keep-keys: %v", err)
	}
	info, err := manager.Info(DefaultStoreName)
	if err != nil {
		t.Fatalf("local info after keep-keys: %v", err)
	}
	if !info.HasKey {
		t.Fatalf("expected passphrase to remain when keep-keys is true")
	}
	if err := manager.Unseal(DefaultStoreName, "", true); err != nil {
		t.Fatalf("unseal local using remembered passphrase: %v", err)
	}
}

func TestManagerInitStyleUnsealLeavesStoreUnsealedAndRemembered(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "stores.json")
	localPath := filepath.Join(tmp, "local")
	manager, err := NewManager(cfgPath, localPath)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	storePath := filepath.Join(tmp, "project-a")
	if err := manager.Add("project-a", storePath, true); err != nil {
		t.Fatalf("init store: %v", err)
	}
	if err := manager.Unseal("project-a", "generated-pass", true); err != nil {
		t.Fatalf("unseal init store: %v", err)
	}
	info, err := manager.Info("project-a")
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if info.Sealed {
		t.Fatalf("expected initialized store to be unsealed")
	}
	if !info.HasKey {
		t.Fatalf("expected initialized store to remember its passphrase")
	}
}

func TestManagerTransientUnsealDoesNotPersistPassphrase(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "stores.json")
	localPath := filepath.Join(tmp, "local")
	manager, err := NewManager(cfgPath, localPath)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if err := manager.Unseal(DefaultStoreName, "temporary", false); err != nil {
		t.Fatalf("transient unseal: %v", err)
	}
	info, err := manager.Info(DefaultStoreName)
	if err != nil {
		t.Fatalf("default info: %v", err)
	}
	if info.HasKey {
		t.Fatalf("expected transient unseal to avoid persisting the passphrase")
	}
}

func TestManagerRemoveStore(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "stores.json")
	localPath := filepath.Join(tmp, "local")
	manager, err := NewManager(cfgPath, localPath)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	storePath := filepath.Join(tmp, "project-a")
	if err := manager.Add("project-a", storePath, true); err != nil {
		t.Fatalf("add store: %v", err)
	}
	if err := manager.Remove("project-a"); err != nil {
		t.Fatalf("remove store: %v", err)
	}
	if _, err := manager.Info("project-a"); err == nil {
		t.Fatalf("expected removed store lookup to fail")
	}
}
