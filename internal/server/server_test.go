package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/micwin/vaultline/internal/daemoncfg"
	"github.com/micwin/vaultline/pkg/stores"
)

type stubNetwork struct{}

func (stubNetwork) AddBind(string) error                { return nil }
func (stubNetwork) RemoveBind(string) error             { return nil }
func (stubNetwork) AddAllow(string, string) error       { return nil }
func (stubNetwork) RemoveAllow(string, string) error    { return nil }
func (stubNetwork) ListAllows(string) ([]string, error) { return nil, nil }
func (stubNetwork) ListBinds() []daemoncfg.Bind {
	return []daemoncfg.Bind{{Addr: "0.0.0.0:8384", Allows: []string{"192.168.3.0/24"}}}
}

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

	srv := New(manager, stubNetwork{}, "test")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.RemoteAddr = "127.0.0.1:12345"
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
	binds, ok := payload["binds"].([]any)
	if !ok || len(binds) != 1 {
		t.Fatalf("expected one bind entry, got %v", payload["binds"])
	}
}

func TestHealthOmitsBindsForRemoteClients(t *testing.T) {
	tmp := t.TempDir()
	manager, err := stores.NewManager(filepath.Join(tmp, "stores.json"), filepath.Join(tmp, "local"))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	srv := New(manager, stubNetwork{}, "test")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.RemoteAddr = "192.168.3.10:54321"
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rr.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := payload["binds"]; ok {
		t.Fatalf("expected binds to be omitted for remote clients")
	}
}

func TestDaemonBindListForbiddenForRemoteClients(t *testing.T) {
	tmp := t.TempDir()
	manager, err := stores.NewManager(filepath.Join(tmp, "stores.json"), filepath.Join(tmp, "local"))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	srv := New(manager, stubNetwork{}, "test")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/daemon/binds", nil)
	req.RemoteAddr = "192.168.3.10:54321"
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden status, got %d", rr.Code)
	}
}

func TestStoreSecretErrorsMentionStoreName(t *testing.T) {
	tmp := t.TempDir()
	manager, err := stores.NewManager(filepath.Join(tmp, "stores.json"), filepath.Join(tmp, "local"))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if err := manager.Add("project-a", filepath.Join(tmp, "project-a"), true); err != nil {
		t.Fatalf("add store: %v", err)
	}
	srv := New(manager, stubNetwork{}, "test")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stores/project-a/secrets/demo", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected sealed response, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "project-a") {
		t.Fatalf("expected store name in error, got %s", rr.Body.String())
	}
}
