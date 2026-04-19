package cli

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/micwin/vaultline/internal/daemoncfg"
	"github.com/micwin/vaultline/pkg/api"
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
	if store != stores.DefaultStoreName || key != "db.password" {
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
	if !strings.Contains(out.String(), "vaultline store init <name> [path]") {
		t.Fatalf("unexpected help output: %s", out.String())
	}
	if !strings.Contains(out.String(), "--prompt-passphrase") {
		t.Fatalf("expected prompt-passphrase in help output: %s", out.String())
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

func TestDefaultNamedStorePath(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/vaultline-data")
	if got := defaultNamedStorePath("bitwarden"); got != "/tmp/vaultline-data/vaultline/stores/bitwarden" {
		t.Fatalf("unexpected default store path: %q", got)
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

func TestMaskSecret(t *testing.T) {
	if got := maskSecret("supersecret"); got != "s*********t" {
		t.Fatalf("unexpected masked secret: %q", got)
	}
	if got := maskSecret("ab"); got != "ab" {
		t.Fatalf("two-character secret should stay same length: %q", got)
	}
	if got := maskSecret("x"); got != "x" {
		t.Fatalf("single-character secret should stay same length: %q", got)
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
	items := buildQualifiedKeyCompletions("project-a", "project-a:", []string{"app.db.password", "app.api.key", "root"})
	expected := []string{"project-a:app.", "project-a:root"}
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

func TestBuildQualifiedKeyCompletionsOnlyShowsNextLevel(t *testing.T) {
	items := buildQualifiedKeyCompletions("project-a", "project-a:app.", []string{"app.db.password", "app.api.key", "app.api.secret", "root"})
	expected := []string{"project-a:app.api.", "project-a:app.db."}
	if len(items) != len(expected) {
		t.Fatalf("unexpected completion count: %#v", items)
	}
	for index, want := range expected {
		if items[index] != want {
			t.Fatalf("unexpected completion at %d: got %q want %q", index, items[index], want)
		}
	}

	deeper := buildQualifiedKeyCompletions("project-a", "project-a:app.api.", []string{"app.db.password", "app.api.key", "app.api.secret"})
	deepExpected := []string{"project-a:app.api.key", "project-a:app.api.secret"}
	if len(deeper) != len(deepExpected) {
		t.Fatalf("unexpected deeper completion count: %#v", deeper)
	}
	for index, want := range deepExpected {
		if deeper[index] != want {
			t.Fatalf("unexpected deeper completion at %d: got %q want %q", index, deeper[index], want)
		}
	}
}

func TestBitwardenEntries(t *testing.T) {
	item := bitwardenItem{Name: "GitHub Prod", Notes: "note text"}
	item.Login.Username = "micwin"
	item.Login.Password = "secret"
	item.Login.Totp = "totp-secret"
	item.Login.URIs = []struct {
		URI string `json:"uri"`
	}{{URI: "https://github.com"}, {URI: "https://api.github.com"}}
	item.Fields = []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}{{Name: "API Token", Value: "abc"}}
	entries := bitwardenEntries(item, "nogroup", "bitwarden", "project-a")
	keys := make(map[string]bool, len(entries))
	for _, entry := range entries {
		keys[entry.Key] = true
	}
	if entries[0].OriginalKey == entries[0].Key {
		t.Fatalf("expected at least one normalized key mapping")
	}
	expected := []string{
		"project-a:bitwarden.nogroup.githubprod.username",
		"project-a:bitwarden.nogroup.githubprod.password",
		"project-a:bitwarden.nogroup.githubprod.totp",
		"project-a:bitwarden.nogroup.githubprod.note",
		"project-a:bitwarden.nogroup.githubprod.uri",
		"project-a:bitwarden.nogroup.githubprod.uri.b",
		"project-a:bitwarden.nogroup.githubprod.field.apitoken",
	}
	for _, key := range expected {
		if !keys[key] {
			t.Fatalf("missing import key %q", key)
		}
	}
}

func TestImportRejectsInvalidGeneratedKeys(t *testing.T) {
	item := bitwardenItem{Name: "foo?bar"}
	item.Login.Password = "secret"
	entries := bitwardenEntries(item, "nogroup", "test", "bitwarden")
	if len(entries) == 0 {
		t.Fatalf("expected entries for item")
	}
	storeName, keyName, err := parseQualifiedKey(entries[0].Key)
	if err != nil {
		t.Fatalf("parse qualified key: %v", err)
	}
	if storeName != "bitwarden" {
		t.Fatalf("unexpected store name: %q", storeName)
	}
	if importKeyPattern.MatchString(keyName) {
		t.Fatalf("expected generated key to violate vaultline naming rules: %q", keyName)
	}
}

func TestFolderNameForItemFallsBackToNogroup(t *testing.T) {
	item := bitwardenItem{FolderID: "missing"}
	if got := folderNameForItem(item, map[string]string{}); got != "nogroup" {
		t.Fatalf("unexpected fallback group: %q", got)
	}
}

func TestNormalizeImportComponent(t *testing.T) {
	if got := normalizeImportComponent("GitHub Prod"); got != "githubprod" {
		t.Fatalf("unexpected normalized component: %q", got)
	}
	if got := normalizeImportComponent("RK 11"); got != "rk11" {
		t.Fatalf("unexpected normalized component with digits: %q", got)
	}
	if got := normalizeImportComponent("Admin / Root"); got != "admin.root" {
		t.Fatalf("unexpected slash normalization: %q", got)
	}
	if got := normalizeImportComponent("A-B:C/D"); got != "a-b.c.d" {
		t.Fatalf("unexpected punctuation normalization: %q", got)
	}
	if got := normalizeImportComponent("Hello_World!(Prod)"); got != "hello-world.prod" {
		t.Fatalf("unexpected underscore/bang/paren normalization: %q", got)
	}
	if got := normalizeImportComponent("Alpha,Beta"); got != "alpha.beta" {
		t.Fatalf("unexpected comma normalization: %q", got)
	}
}

func TestAlphabeticOrdinal(t *testing.T) {
	if got := alphabeticOrdinal(1); got != "a" {
		t.Fatalf("unexpected ordinal for 1: %q", got)
	}
	if got := alphabeticOrdinal(2); got != "b" {
		t.Fatalf("unexpected ordinal for 2: %q", got)
	}
}

func TestLoadBitwardenItems(t *testing.T) {
	orig := execCommand
	defer func() { execCommand = orig }()
	execCommand = func(name string, args ...string) *exec.Cmd {
		if len(args) > 1 && args[0] == "list" && args[1] == "folders" {
			return exec.Command("sh", "-c", "printf '[]'")
		}
		jsonPayload := `[{
  "id": "1",
  "name": "GitHub",
  "login": {"username": "micwin", "password": "secret"}
}]`
		return exec.Command("sh", "-c", fmt.Sprintf("cat <<'EOF'\n%s\nEOF", jsonPayload))
	}
	items, err := loadBitwardenItems()
	if err != nil {
		t.Fatalf("load items: %v", err)
	}
	if len(items) != 1 || items[0].Name != "GitHub" {
		t.Fatalf("unexpected items: %#v", items)
	}
}

func TestChooseImportAction(t *testing.T) {
	if got := chooseImportAction(false, true, false); got != importActionAdd {
		t.Fatalf("expected add action, got %v", got)
	}
	if got := chooseImportAction(true, true, false); got != importActionSkipExisting {
		t.Fatalf("expected skip-existing action, got %v", got)
	}
	if got := chooseImportAction(true, false, true); got != importActionUpdate {
		t.Fatalf("expected update action, got %v", got)
	}
	if got := chooseImportAction(false, false, false); got != importActionSkipMissing {
		t.Fatalf("expected skip-missing action, got %v", got)
	}
}

func TestSecretDeletePrefixDryRun(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/stores/default/secrets":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"keys":[{"name":"app.demo.one"},{"name":"app.demo.two"},{"name":"other.key"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	var out bytes.Buffer
	if err := secretDeletePrefix(server.URL, []string{"default:app.demo.", "--dry-run"}, &out); err != nil {
		t.Fatalf("delete-prefix dry-run: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "default:app.demo.one") || !strings.Contains(output, "default:app.demo.two") {
		t.Fatalf("unexpected dry-run output: %s", output)
	}
	if strings.Contains(output, "other.key") {
		t.Fatalf("delete-prefix matched unrelated key: %s", output)
	}
}

func TestStoreSealAcceptsKeepKeysAfterStoreName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/stores/project-a/seal" {
			http.NotFound(w, r)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if !strings.Contains(string(body), `"keep_keys":true`) {
			t.Fatalf("expected keep_keys=true payload, got %s", body)
		}
		_, _ = w.Write([]byte(`{"sealed":true,"store":"project-a","keep_keys":true}`))
	}))
	defer server.Close()
	var out bytes.Buffer
	if err := runStore(server.URL, []string{"seal", "project-a", "--keep-keys"}, &out); err != nil {
		t.Fatalf("run store seal: %v", err)
	}
}

func TestParseGlobPattern(t *testing.T) {
	storePattern, keyPattern := parseGlobPattern("bitw*:*.zf.*test*")
	if storePattern != "bitw*" || keyPattern != "*.zf.*test*" {
		t.Fatalf("unexpected parsed glob pattern: %q %q", storePattern, keyPattern)
	}
	storePattern, keyPattern = parseGlobPattern("*password*")
	if storePattern != "*" || keyPattern != "*password*" {
		t.Fatalf("unexpected implicit-store glob pattern: %q %q", storePattern, keyPattern)
	}
}

func TestSecretGlobMatchesAcrossStores(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/stores/default/secrets":
			_, _ = w.Write([]byte(`{"keys":[{"name":"app.demo.one"}]}`))
		case "/api/v1/stores/project-a/secrets":
			_, _ = w.Write([]byte(`{"keys":[{"name":"team.zf.test-user"},{"name":"team.other"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	origStores := listConfiguredStoresFn
	defer func() { listConfiguredStoresFn = origStores }()
	listConfiguredStoresFn = func(includeLocal bool) []string { return []string{"default", "project-a"} }
	var out bytes.Buffer
	if err := secretGlob(server.URL, []string{"project-*:*.zf.*test*"}, "text", &out); err != nil {
		t.Fatalf("secret glob: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "project-a:team.zf.test-user") {
		t.Fatalf("expected match in output: %s", output)
	}
	if strings.Contains(output, "team.other") || strings.Contains(output, "default:app.demo.one") {
		t.Fatalf("unexpected non-matching entries in output: %s", output)
	}
}

func TestSecretTransferCopyMoveAndForce(t *testing.T) {
	secrets := map[string]string{
		"source:alpha": base64.StdEncoding.EncodeToString([]byte("from-source")),
		"target:alpha": base64.StdEncoding.EncodeToString([]byte("existing-target")),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/stores/") || !strings.Contains(r.URL.Path, "/secrets/") {
			http.NotFound(w, r)
			return
		}
		rest := strings.TrimPrefix(r.URL.Path, "/api/v1/stores/")
		parts := strings.SplitN(rest, "/secrets/", 2)
		if len(parts) != 2 {
			http.NotFound(w, r)
			return
		}
		storeName, err := url.PathUnescape(parts[0])
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		keyName, err := url.PathUnescape(parts[1])
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		qualified := storeName + ":" + keyName
		switch r.Method {
		case http.MethodGet:
			value, ok := secrets[qualified]
			if !ok {
				http.Error(w, `{"error":"SECRET_NOT_FOUND"}`, http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(api.SecretResponse{Value: value, Version: "v1"})
		case http.MethodPut:
			var payload api.SecretRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			secrets[qualified] = payload.Value
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			if _, ok := secrets[qualified]; !ok {
				http.Error(w, `{"error":"SECRET_NOT_FOUND"}`, http.StatusNotFound)
				return
			}
			delete(secrets, qualified)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var out bytes.Buffer
	err := secretTransfer(server.URL, []string{"source:alpha", "target:alpha"}, false, &out)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected collision error without --force, got %v", err)
	}
	if got := secrets["target:alpha"]; got != base64.StdEncoding.EncodeToString([]byte("existing-target")) {
		t.Fatalf("target changed on failed copy")
	}

	out.Reset()
	if err := secretTransfer(server.URL, []string{"source:alpha", "target:alpha", "--force"}, false, &out); err != nil {
		t.Fatalf("forced copy failed: %v", err)
	}
	if got := secrets["target:alpha"]; got != base64.StdEncoding.EncodeToString([]byte("from-source")) {
		t.Fatalf("forced copy did not overwrite target")
	}
	if _, ok := secrets["source:alpha"]; !ok {
		t.Fatalf("copy should keep source secret")
	}

	out.Reset()
	if err := secretTransfer(server.URL, []string{"source:alpha", "target:beta"}, true, &out); err != nil {
		t.Fatalf("move failed: %v", err)
	}
	if _, ok := secrets["source:alpha"]; ok {
		t.Fatalf("move should remove source secret")
	}
	if got := secrets["target:beta"]; got != base64.StdEncoding.EncodeToString([]byte("from-source")) {
		t.Fatalf("move destination mismatch")
	}
}

func TestCompleteSecretWordsIncludesCopyMove(t *testing.T) {
	subs := completeWords([]string{"secret"}, "c")
	if len(subs) == 0 || subs[0] != "copy" {
		t.Fatalf("expected copy suggestion, got %#v", subs)
	}
	flags := completeWords([]string{"secret", "copy"}, "--f")
	foundForce := false
	for _, item := range flags {
		if item == "--force" {
			foundForce = true
			break
		}
	}
	if !foundForce {
		t.Fatalf("expected --force completion, got %#v", flags)
	}
}

func TestZipDirectoryRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	if err := os.MkdirAll(filepath.Join(src, "secrets"), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, ".master_salt"), []byte("salt"), 0o600); err != nil {
		t.Fatalf("write salt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "secrets", "demo.vlx"), []byte("payload"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	zipPath := filepath.Join(tmp, "store.zip")
	if err := zipDirectory(src, zipPath); err != nil {
		t.Fatalf("zip directory: %v", err)
	}
	if _, err := zip.OpenReader(zipPath); err != nil {
		t.Fatalf("open zip: %v", err)
	}
	dst := filepath.Join(tmp, "dst")
	if err := unzipToDirectory(zipPath, dst); err != nil {
		t.Fatalf("unzip directory: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dst, "secrets", "demo.vlx"))
	if err != nil {
		t.Fatalf("read unzipped secret: %v", err)
	}
	if string(data) != "payload" {
		t.Fatalf("unexpected unzipped payload: %q", data)
	}
}

func TestDefaultExportZipName(t *testing.T) {
	name := defaultExportZipName("default")
	if !strings.HasPrefix(name, "default-") || !strings.HasSuffix(name, ".zip") {
		t.Fatalf("unexpected export zip name: %q", name)
	}
}
