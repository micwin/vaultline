package cli

import "testing"

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
