package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseQualifiedKey(t *testing.T) {
	store, key, err := parseQualifiedKey("project-a:db.password")
	if err != nil {
		t.Fatalf("parse qualified key: %v", err)
	}
	if store != "project-a" || key != "db.password" {
		t.Fatalf("unexpected parse result: %q %q", store, key)
	}

	store, key, err = parseQualifiedKey("db.password")
	if err != nil {
		t.Fatalf("parse default key: %v", err)
	}
	if store != "local" || key != "db.password" {
		t.Fatalf("unexpected default parse result: %q %q", store, key)
	}
}

func TestParseStoreSelector(t *testing.T) {
	store, err := parseStoreSelector("project-a:")
	if err != nil {
		t.Fatalf("parse store selector: %v", err)
	}
	if store != "project-a" {
		t.Fatalf("unexpected store selector: %q", store)
	}
}

func TestStoreInitHelpShowsSubcommandUsage(t *testing.T) {
	var out bytes.Buffer
	if err := Run([]string{"store", "init", "--help"}, &out); err != nil {
		t.Fatalf("run store init help: %v", err)
	}
	if !strings.Contains(out.String(), "vaultline store init <name> <path>") {
		t.Fatalf("unexpected help output: %s", out.String())
	}
}

func TestSecretSetHelpShowsSubcommandUsage(t *testing.T) {
	var out bytes.Buffer
	if err := Run([]string{"secret", "set", "--help"}, &out); err != nil {
		t.Fatalf("run secret set help: %v", err)
	}
	if !strings.Contains(out.String(), "vaultline secret set <store:key>") {
		t.Fatalf("unexpected help output: %s", out.String())
	}
}

func TestConfirmSecretMatch(t *testing.T) {
	value, err := confirmSecretMatch([]byte("same"), []byte("same"))
	if err != nil {
		t.Fatalf("matching secrets should succeed: %v", err)
	}
	if string(value) != "same" {
		t.Fatalf("unexpected confirmed value: %q", value)
	}
}

func TestConfirmSecretMatchMismatch(t *testing.T) {
	if _, err := confirmSecretMatch([]byte("one"), []byte("two")); err == nil {
		t.Fatalf("mismatched secrets should fail")
	}
}

func TestReadSecretInputTwiceRequiresInteractiveStdin(t *testing.T) {
	if _, err := readSecretInput("", "", true, true); err == nil || !strings.Contains(err.Error(), "interactive") {
		t.Fatalf("expected interactive stdin error, got %v", err)
	}
}
