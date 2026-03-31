package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/micwin/vaultline/internal/daemoncfg"
	"github.com/micwin/vaultline/pkg/stores"
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

func TestDaemonBindHelpShowsSubcommandUsage(t *testing.T) {
	var out bytes.Buffer
	if err := Run([]string{"daemon", "bind", "--help"}, &out); err != nil {
		t.Fatalf("run daemon bind help: %v", err)
	}
	if !strings.Contains(out.String(), "vaultline daemon bind <addr>") {
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

func TestCompleteStoreCommandsSuggestConfiguredStores(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	manager, err := stores.NewManager(resolveStoreConfigPath(), resolveLocalStorePath())
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if err := manager.Add("project-a", filepath.Join(tmp, "project-a"), true); err != nil {
		t.Fatalf("add store: %v", err)
	}
	suggestions := completeWords([]string{"store", "show"}, "pr")
	if len(suggestions) != 1 || suggestions[0] != "project-a" {
		t.Fatalf("unexpected store suggestions: %#v", suggestions)
	}
	secretSuggestions := completeWords([]string{"secret", "set"}, "pro")
	if len(secretSuggestions) != 1 || secretSuggestions[0] != "project-a:" {
		t.Fatalf("unexpected secret suggestions: %#v", secretSuggestions)
	}
}

func TestCompleteDaemonCommandsSuggestBindsAndAllows(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	mgr, err := daemoncfg.New(resolveDaemonConfigPath())
	if err != nil {
		t.Fatalf("new daemon cfg: %v", err)
	}
	if err := mgr.AddBind("0.0.0.0:8384"); err != nil {
		t.Fatalf("add bind: %v", err)
	}
	if err := mgr.AddAllow("0.0.0.0:8384", "192.168.3.1"); err != nil {
		t.Fatalf("add allow: %v", err)
	}
	bindSuggestions := completeWords([]string{"daemon", "unbind"}, "0.0")
	if len(bindSuggestions) != 1 || bindSuggestions[0] != "0.0.0.0:8384" {
		t.Fatalf("unexpected bind suggestions: %#v", bindSuggestions)
	}
	allowSuggestions := completeWords([]string{"daemon", "unallow", "0.0.0.0:8384"}, "192")
	if len(allowSuggestions) != 1 || allowSuggestions[0] != "192.168.3.1/32" {
		t.Fatalf("unexpected allow suggestions: %#v", allowSuggestions)
	}
}

func TestCompletionScriptsMentionHiddenCompleteCommand(t *testing.T) {
	if !strings.Contains(bashCompletionScript(), "__complete") {
		t.Fatalf("bash completion script missing hidden completion hook")
	}
	if !strings.Contains(zshCompletionScript(), "__complete") {
		t.Fatalf("zsh completion script missing hidden completion hook")
	}
}

func TestCompletionScriptsSuppressSpaceForHierarchies(t *testing.T) {
	if !strings.Contains(bashCompletionScript(), "compopt -o nospace") {
		t.Fatalf("bash completion script should suppress spaces for hierarchical completions")
	}
	if !strings.Contains(zshCompletionScript(), "compadd -Q -S ''") {
		t.Fatalf("zsh completion script should suppress spaces for hierarchical completions")
	}
}

func TestBuildQualifiedKeyCompletionsBuildsSegmentPrefixes(t *testing.T) {
	items := buildQualifiedKeyCompletions("project-a", []string{"app.db.password", "app.api.key", "root"})
	expected := []string{
		"project-a:app.",
		"project-a:app.api.",
		"project-a:app.api.key",
		"project-a:app.db.",
		"project-a:app.db.password",
		"project-a:root",
	}
	for _, want := range expected {
		found := false
		for _, got := range items {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing completion %q in %#v", want, items)
		}
	}
}
