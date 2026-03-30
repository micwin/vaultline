package daemoncfg

import (
	"path/filepath"
	"testing"
)

func TestAddBindRejectsLoopback(t *testing.T) {
	manager, err := New(filepath.Join(t.TempDir(), "daemon.json"))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if err := manager.AddBind("127.0.0.1:9999"); err == nil {
		t.Fatalf("expected loopback bind rejection")
	}
}

func TestAddAllowNormalizesHostRule(t *testing.T) {
	manager, err := New(filepath.Join(t.TempDir(), "daemon.json"))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if err := manager.AddBind("0.0.0.0:9999"); err != nil {
		t.Fatalf("add bind: %v", err)
	}
	if err := manager.AddAllow("0.0.0.0:9999", "192.168.3.1"); err != nil {
		t.Fatalf("add allow: %v", err)
	}
	allows, err := manager.ListAllows("0.0.0.0:9999")
	if err != nil {
		t.Fatalf("list allows: %v", err)
	}
	if len(allows) != 1 || allows[0] != "192.168.3.1/32" {
		t.Fatalf("unexpected allows: %#v", allows)
	}
}
