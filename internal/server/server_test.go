package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/micwin/vaultline/pkg/stores"
)

func TestHealthDegradedWhenExternalStoreUnavailable(t *testing.T) {
	tmp := t.TempDir()
	manager, err := stores.NewManager(filepath.Join(tmp, "stores.json"), filepath.Join(tmp, "local"))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	// register a broken external store through the persisted config
	cfg, err := stores.LoadConfig(filepath.Join(tmp, "stores.json"), filepath.Join(tmp, "local"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Stores["project-a"] = stores.Entry{Path: filepath.Join(tmp, "missing")}
	if err := stores.SaveConfig(filepath.Join(tmp, "stores.json"), cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	manager, err = stores.NewManager(filepath.Join(tmp, "stores.json"), filepath.Join(tmp, "local"))
	if err != nil {
		t.Fatalf("reload manager: %v", err)
	}

	srv := New(manager, "test")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rr.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["status"] != "degraded" {
		t.Fatalf("expected degraded status, got %v", payload["status"])
	}
}
