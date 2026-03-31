package cli

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/micwin/vaultline/internal/daemoncfg"
	"github.com/micwin/vaultline/pkg/api"
	"github.com/micwin/vaultline/pkg/stores"
	"github.com/micwin/vaultline/pkg/version"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}
var execCommand = exec.Command
var importKeyPattern = regexp.MustCompile(`^[\p{Ll}\p{Nd}@.-]+$`)
var listConfiguredStoresFn = listConfiguredStores

func usageText() string {
	return fmt.Sprintf("vaultline %s\n\n", version.Version) + `Usage:
  vaultline daemon [--addr HOST:PORT] [--store-dir DIR]
      Starts the HTTPS API server. When --store-dir is omitted the daemon
      uses $XDG_DATA_HOME/vaultline/stores/default or ~/.local/share/vaultline/stores/default.

  vaultline daemon bind <addr>
  vaultline daemon list-binds
  vaultline daemon unbind <addr>
  vaultline daemon allow <addr> <cidr-or-ip>
  vaultline daemon list-allows <addr>
  vaultline daemon unallow <addr> <cidr-or-ip>
      Manage extra remote listeners. Loopback remains implicitly available.

  vaultline completion bash|zsh
      Print shell completion for the requested shell.

  vaultline import bitwarden [flags]
      Import secrets from Bitwarden via the bw CLI.

  vaultline import zip <zip-file> <store>
      Restore a store directory from a zip archive.

  vaultline export zip <store> <zip-file>
      Archive a store directory into a zip file.

  vaultline backup zip <store> [zip-file]
  vaultline restore zip <zip-file> <store> [--overwrite]
      Archive and restore full sealed stores.

  vaultline [--addr HOST:PORT] <command> [flags]
      health                     Check daemon status
      unseal                     Prompt for passphrase and unlock the default store
      seal                       Reseal the default store
      daemon-stop                Ask the daemon to shut down
      completion                 Print shell completion helpers
      import                     Import external secrets
      backup                     Back up a full store
      restore                    Restore a full store backup
      export                     Export stores
      daemon                     Manage extra daemon binds and allow rules
      store add|init|list|show|unseal|seal|delete
                                 Manage named stores
      secret set|get|delete|list Manage secrets (keys use lowercase letters plus . and -)

Examples:
  vaultline daemon --store-dir ./store
  vaultline --addr 127.0.0.1:8428 health
  vaultline --addr 127.0.0.1:8428 secret set project-a:app.api-key --stdin
`
}

func storeUsageText() string {
	return `Usage:
  vaultline store add <name> <path>
      Register an existing named store.

  vaultline store init <name> [path]
      Create a new named store, generate an unseal key, store it in config,
      and leave the store immediately unsealed. When path is omitted, the store
      is created next to the default store under the same parent directory.

  vaultline store list
      Show all configured stores and their status.

  vaultline store show <name>
      Show one store as JSON.

  vaultline store unseal <name>
      Unseal a named store. Uses a remembered passphrase first, then prompts.

  vaultline store seal <name> [--keep-keys]
      Seal a named store. By default remembered passphrases are removed.

  vaultline store delete|remove|rm <name>
      Remove a named store from the registry (does not delete files on disk).
`
}

func daemonUsageText() string {
	return `Usage:
  vaultline daemon bind <addr>
      Add an extra listener address. Loopback stays implicit and cannot be managed here.

  vaultline daemon list-binds
      List configured extra listeners.

  vaultline daemon unbind <addr>
      Remove an extra listener and all of its allow rules.

  vaultline daemon allow <addr> <cidr-or-ip>
      Allow one remote host/network for a configured extra listener.

  vaultline daemon list-allows <addr>
      List allow rules for one extra listener.

  vaultline daemon unallow <addr> <cidr-or-ip>
      Remove one allow rule from a listener.
`
}

func completionUsageText() string {
	return `Usage:
  vaultline completion bash
  vaultline completion zsh

Print shell completion helpers for the selected shell.
`
}

func importUsageText() string {
	return `Usage:
  vaultline import bitwarden --all [--prefix PREFIX] [--store STORE] [--dry-run] [--add-missing-keys] [--overwrite-existing-keys]
  vaultline import bitwarden --item NAME [--prefix PREFIX] [--store STORE] [--dry-run] [--add-missing-keys] [--overwrite-existing-keys]
  vaultline import zip <zip-file> <store>

Reads Bitwarden items via the installed bw CLI. Requires an unlocked bw session.
`
}

func exportUsageText() string {
	return `Usage:
  vaultline export zip <store> [zip-file]

Archive a full store directory into a zip file. When the file is omitted,
Vaultline uses <store>-YYYY-MM-DD.zip in the current directory.
`
}

func backupUsageText() string {
	return `Usage:
  vaultline backup zip <store> [zip-file]

Archive a full store directory into a zip file. When the file is omitted,
Vaultline uses <store>-YYYY-MM-DD.zip in the current directory.
`
}

func restoreUsageText() string {
	return `Usage:
  vaultline restore zip <zip-file> <store> [--overwrite]

Restore a full store from a zip backup. Existing stores require --overwrite.
`
}

func importSubcommandHelp(name string) string {
	switch name {
	case "bitwarden":
		return "Usage:\n  vaultline import bitwarden --all [--prefix PREFIX] [--store STORE] [--dry-run] [--add-missing-keys] [--overwrite-existing-keys]\n  vaultline import bitwarden --item NAME [--prefix PREFIX] [--store STORE] [--dry-run] [--add-missing-keys] [--overwrite-existing-keys]\n\nImport login and secure-note items from Bitwarden via the bw CLI. No prefix is added unless --prefix is specified. Bitwarden item names must already fit Vaultline's key rules after the optional prefix and group are added."
	case "zip":
		return "Usage:\n  vaultline import zip <zip-file> <store>\n\nRestore a store directory from a zip archive into the target store path."
	default:
		return importUsageText()
	}
}

func exportSubcommandHelp(name string) string {
	switch name {
	case "zip":
		return "Usage:\n  vaultline export zip <store> [zip-file]\n\nArchive a full store directory into a zip file. If no file is given, Vaultline writes <store>-YYYY-MM-DD.zip in the current directory."
	default:
		return exportUsageText()
	}
}

func backupSubcommandHelp(name string) string {
	if name == "zip" {
		return "Usage:\n  vaultline backup zip <store> [zip-file]\n\nArchive a full store directory into a zip file."
	}
	return backupUsageText()
}

func restoreSubcommandHelp(name string) string {
	if name == "zip" {
		return "Usage:\n  vaultline restore zip <zip-file> <store> [--overwrite]\n\nRestore a full store from a zip backup. Existing stores require --overwrite."
	}
	return restoreUsageText()
}

func daemonSubcommandHelp(name string) string {
	switch name {
	case "bind":
		return "Usage:\n  vaultline daemon bind <addr>\n\nAdd an extra listener address. Loopback remains implicitly available and cannot be configured here."
	case "list-binds":
		return "Usage:\n  vaultline daemon list-binds\n\nList all configured extra listeners and whether they are blocked or allowlisted."
	case "unbind":
		return "Usage:\n  vaultline daemon unbind <addr>\n\nRemove an extra listener and all of its allow rules."
	case "allow":
		return "Usage:\n  vaultline daemon allow <addr> <cidr-or-ip>\n\nAllow one remote host or network for a configured listener."
	case "list-allows":
		return "Usage:\n  vaultline daemon list-allows <addr>\n\nList allow rules for one listener."
	case "unallow":
		return "Usage:\n  vaultline daemon unallow <addr> <cidr-or-ip>\n\nRemove one allow rule from a configured listener."
	default:
		return daemonUsageText()
	}
}

func secretUsageText() string {
	return `Usage:
  vaultline secret set <store:key> [--value VALUE|--file PATH|--stdin] [--twice]
  vaultline secret get <store:key> [--out PATH] [--output raw|json]
  vaultline secret delete <store:key>
  vaultline secret delete-prefix <store:prefix.> [--dry-run] [--yes]
  vaultline secret glob <store-glob:key-glob>
  vaultline secret list [store:]

Notes:
  - Store prefixes use the form store:key and default to default when omitted.
  - Secret names must use lowercase letters, digits, @, . or -.
  - --twice only applies to interactive --stdin input and aborts on mismatch.
`
}

func storeSubcommandHelp(name string) string {
	switch name {
	case "add":
		return "Usage:\n  vaultline store add <name> <path>\n\nRegister an existing store path in the local registry."
	case "init":
		return "Usage:\n  vaultline store init <name> [path]\n\nCreate a new store at [path], generate an unseal key, remember it in config, and leave the store unsealed. If no path is given, the store is created next to the default store."
	case "list":
		return "Usage:\n  vaultline store list\n\nList all configured stores and their status."
	case "show":
		return "Usage:\n  vaultline store show <name>\n\nShow one configured store as JSON."
	case "unseal":
		return "Usage:\n  vaultline store unseal <name>\n\nUnseal one named store. Uses a remembered passphrase first, then prompts if needed."
	case "seal":
		return "Usage:\n  vaultline store seal <name> [--keep-keys]\n\nSeal one named store. By default remembered passphrases are removed from config."
	case "delete", "remove", "rm":
		return "Usage:\n  vaultline store delete <name>\n\nRemove a named store from the registry without touching files on disk."
	default:
		return storeUsageText()
	}
}

func secretSubcommandHelp(name string) string {
	switch name {
	case "set":
		return "Usage:\n  vaultline secret set <store:key> [--value VALUE|--file PATH|--stdin] [--twice]\n\nStore or overwrite a secret in the selected store. `--twice` requires interactive `--stdin` and asks for the secret twice."
	case "get":
		return "Usage:\n  vaultline secret get <store:key> [--out PATH] [--output raw|json]\n\nFetch a secret from the selected store."
	case "delete":
		return "Usage:\n  vaultline secret delete <store:key>\n\nDelete a secret from the selected store."
	case "delete-prefix":
		return "Usage:\n  vaultline secret delete-prefix <store:prefix.> [--dry-run] [--yes]\n\nDelete all secrets whose names start with the given prefix. The prefix must end with a dot."
	case "glob":
		return "Usage:\n  vaultline secret glob <store-glob:key-glob>\n\nSearch secrets using shell-style glob patterns. If the store part is omitted, all stores are searched."
	case "list":
		return "Usage:\n  vaultline secret list [store:]\n\nList secrets from one store (default: default)."
	default:
		return secretUsageText()
	}
}

func isHelpArg(arg string) bool {
	return arg == "-h" || arg == "--help" || arg == "help"
}

// Run executes the CLI subcommands.
func Run(args []string, out io.Writer) error {
	if len(args) == 1 && isHelpArg(args[0]) {
		fmt.Fprint(out, usageText())
		return nil
	}
	fs := flag.NewFlagSet("vaultline", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:8428", "vaultline address")
	output := fs.String("output", "text", "output format (text|json|raw for secret get)")
	fs.Usage = func() {
		fmt.Fprint(out, usageText())
	}
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fs.Usage()
			return nil
		}
		return err
	}
	remaining := fs.Args()
	if len(remaining) == 0 {
		fs.Usage()
		return errors.New("missing command")
	}

	baseURL, err := buildBaseURL(*addr)
	if err != nil {
		return err
	}

	switch remaining[0] {
	case "__complete":
		return runComplete(remaining[1:], out)
	case "health":
		return runHealth(baseURL, *output, out)
	case "unseal":
		return runUnseal(baseURL, remaining[1:], out)
	case "seal":
		return runSeal(baseURL, remaining[1:], out)
	case "daemon-stop":
		return runDaemonStop(baseURL, out)
	case "daemon":
		return runDaemon(baseURL, remaining[1:], out)
	case "completion":
		return runCompletion(remaining[1:], out)
	case "import":
		return runImport(baseURL, remaining[1:], out)
	case "backup":
		return runBackup(baseURL, remaining[1:], out)
	case "restore":
		return runRestore(baseURL, remaining[1:], out)
	case "export":
		return runExport(baseURL, remaining[1:], out)
	case "store":
		return runStore(baseURL, remaining[1:], out)
	case "secret":
		return runSecret(baseURL, remaining[1:], *output, out)
	default:
		return fmt.Errorf("unknown command %q", remaining[0])
	}
}

func runHealth(baseURL, outputFmt string, out io.Writer) error {
	resp, err := httpClient.Get(baseURL + "/api/v1/health")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if outputFmt == "json" {
		fmt.Fprintln(out, string(body))
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		fmt.Fprintln(out, string(body))
		return nil
	}
	printKeyValueTable(
		[][2]string{{"DAEMON_STATUS", fmt.Sprint(payload["status"])}, {"DEFAULT_STORE", fmt.Sprint(payload["default_store"])}},
		out,
	)
	if storesValue, ok := payload["stores"].([]any); ok {
		rows := make([][]string, 0, len(storesValue))
		for _, item := range storesValue {
			if entry, ok := item.(map[string]any); ok {
				errValue := ""
				if entry["error"] != nil {
					errValue = sanitizeStoreError(fmt.Sprint(entry["error"]))
				}
				storeStatus := "unsealed"
				if available, ok := entry["available"].(bool); ok && !available {
					storeStatus = "unavailable"
				} else if sealed, ok := entry["sealed"].(bool); ok && sealed {
					storeStatus = "sealed"
				}
				rows = append(rows, []string{fmt.Sprint(entry["name"]), storeStatus, errValue})
			}
		}
		printTable([]string{"STORE", "STATUS", "ERROR"}, rows, out)
	}
	if bindsValue, ok := payload["binds"].([]any); ok {
		rows := make([][]string, 0, len(bindsValue))
		for _, item := range bindsValue {
			if entry, ok := item.(map[string]any); ok {
				rows = append(rows, []string{fmt.Sprint(entry["addr"]), fmt.Sprint(entry["allow_count"]), fmt.Sprint(entry["state"])})
			}
		}
		if len(rows) > 0 {
			fmt.Fprintln(out)
			printTable([]string{"BIND", "ALLOWS", "STATE"}, rows, out)
		}
	}
	return nil
}

func tryUnseal(endpoint, passphrase string) ([]byte, int, error) {
	payload, err := json.Marshal(api.UnsealRequest{Passphrase: passphrase})
	if err != nil {
		return nil, 0, err
	}
	resp, err := httpClient.Post(endpoint, "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return body, resp.StatusCode, nil
}

func runUnseal(baseURL string, args []string, out io.Writer) error {
	if len(args) > 0 {
		if len(args) == 1 && isHelpArg(args[0]) {
			fmt.Fprintln(out, "Usage:\n  vaultline unseal\n\nUnseal the default store. Uses a remembered passphrase first, then prompts if needed.")
			return nil
		}
		return fmt.Errorf("usage: vaultline unseal")
	}
	body, status, err := tryUnseal(baseURL+"/api/v1/unseal", "")
	if err != nil {
		return err
	}
	if status == http.StatusOK {
		fmt.Fprintln(out, "vaultline unsealed")
		return nil
	}
	if !strings.Contains(string(body), "passphrase required") {
		return fmt.Errorf("unseal failed: %s", body)
	}
	passphrase, err := readPassphrase()
	if err != nil {
		return err
	}
	body, status, err = tryUnseal(baseURL+"/api/v1/unseal", passphrase)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("unseal failed: %s", body)
	}
	fmt.Fprintln(out, "vaultline unsealed")
	return nil
}

func runSeal(baseURL string, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("seal", flag.ContinueOnError)
	keepKeys := fs.Bool("keep-keys", false, "keep remembered passphrases in store config")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprintln(out, "Usage:\n  vaultline seal [--keep-keys]\n\nSeal the default store. By default remembered passphrases are removed from config.")
			return nil
		}
		return err
	}
	payload, err := json.Marshal(api.SealRequest{KeepKeys: *keepKeys})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/seal", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("seal failed: %s", body)
	}
	fmt.Fprintln(out, "vaultline sealed")
	return nil
}

func runDaemonStop(baseURL string, out io.Writer) error {
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/shutdown", nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("shutdown failed: %s", body)
	}
	fmt.Fprintln(out, "vaultline shutdown requested")
	return nil
}

func runCompletion(args []string, out io.Writer) error {
	if len(args) != 1 || isHelpArg(args[0]) {
		fmt.Fprintln(out, completionUsageText())
		if len(args) == 1 && isHelpArg(args[0]) {
			return nil
		}
		return fmt.Errorf("usage: vaultline completion <bash|zsh>")
	}
	switch args[0] {
	case "bash":
		fmt.Fprint(out, bashCompletionScript())
		return nil
	case "zsh":
		fmt.Fprint(out, zshCompletionScript())
		return nil
	default:
		return fmt.Errorf("unsupported completion shell %q", args[0])
	}
}

type bitwardenItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Notes    string `json:"notes"`
	FolderID string `json:"folderId"`
	Login    struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Totp     string `json:"totp"`
		URIs     []struct {
			URI string `json:"uri"`
		} `json:"uris"`
	} `json:"login"`
	Fields []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"fields"`
	Type int `json:"type"`
}

type bitwardenFolder struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type importEntry struct {
	OriginalKey  string
	Key          string
	OriginalBase string
	Base         string
	Value        []byte
}

type importAction int

const (
	importActionSkipExisting importAction = iota
	importActionSkipMissing
	importActionAdd
	importActionUpdate
)

func runImport(baseURL string, args []string, out io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintln(out, importUsageText())
		return errors.New("import command requires subcommand")
	}
	if isHelpArg(args[0]) {
		fmt.Fprintln(out, importUsageText())
		return nil
	}
	if len(args) > 1 && isHelpArg(args[1]) {
		fmt.Fprintln(out, importSubcommandHelp(args[0]))
		return nil
	}
	switch args[0] {
	case "bitwarden":
		return runImportBitwarden(baseURL, args[1:], out)
	case "zip":
		return runImportZip(baseURL, args[1:], out)
	default:
		return fmt.Errorf("unknown import subcommand %q", args[0])
	}
}

func runExport(baseURL string, args []string, out io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintln(out, exportUsageText())
		return errors.New("export command requires subcommand")
	}
	if isHelpArg(args[0]) {
		fmt.Fprintln(out, exportUsageText())
		return nil
	}
	if len(args) > 1 && isHelpArg(args[1]) {
		fmt.Fprintln(out, exportSubcommandHelp(args[0]))
		return nil
	}
	switch args[0] {
	case "zip":
		return runExportZip(baseURL, args[1:], out)
	default:
		return fmt.Errorf("unknown export subcommand %q", args[0])
	}
}

func runBackup(baseURL string, args []string, out io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintln(out, backupUsageText())
		return errors.New("backup command requires subcommand")
	}
	if isHelpArg(args[0]) {
		fmt.Fprintln(out, backupUsageText())
		return nil
	}
	if len(args) > 1 && isHelpArg(args[1]) {
		fmt.Fprintln(out, backupSubcommandHelp(args[0]))
		return nil
	}
	switch args[0] {
	case "zip":
		return runExportZip(baseURL, args[1:], out)
	default:
		return fmt.Errorf("unknown backup subcommand %q", args[0])
	}
}

func runRestore(baseURL string, args []string, out io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintln(out, restoreUsageText())
		return errors.New("restore command requires subcommand")
	}
	if isHelpArg(args[0]) {
		fmt.Fprintln(out, restoreUsageText())
		return nil
	}
	if len(args) > 1 && isHelpArg(args[1]) {
		fmt.Fprintln(out, restoreSubcommandHelp(args[0]))
		return nil
	}
	switch args[0] {
	case "zip":
		return runImportZip(baseURL, args[1:], out)
	default:
		return fmt.Errorf("unknown restore subcommand %q", args[0])
	}
}

func runExportZip(baseURL string, args []string, out io.Writer) error {
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("usage: vaultline export zip <store> [zip-file]")
	}
	storeName := args[0]
	targetZip := ""
	if len(args) == 2 {
		targetZip = args[1]
	} else {
		targetZip = defaultExportZipName(storeName)
	}
	resp, err := httpClient.Get(baseURL + "/api/v1/stores/" + url.PathEscape(storeName) + "/backup.zip")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("backup failed: %s", body)
	}
	if err := os.MkdirAll(filepath.Dir(targetZip), 0o755); err != nil {
		return err
	}
	file, err := os.Create(targetZip)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := io.Copy(file, resp.Body); err != nil {
		return err
	}
	fmt.Fprintf(out, "backuped %s to %s\n", storeName, targetZip)
	fmt.Fprintln(out, "note: the unseal key is not included in the backup")
	if storeName == stores.DefaultStoreName {
		fmt.Fprintln(out, "show it with: cat ~/.config/vaultline/seal")
	} else {
		fmt.Fprintf(out, "show it with: jq -r '.stores[\"%s\"].passphrase' ~/.config/vaultline/stores.json\n", storeName)
	}
	return nil
}

func runImportZip(baseURL string, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("import zip", flag.ContinueOnError)
	overwrite := fs.Bool("overwrite", false, "overwrite the target store if it already exists")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return fmt.Errorf("usage: vaultline import zip <zip-file> <store> [--overwrite]")
	}
	data, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		return err
	}
	endpoint := baseURL + "/api/v1/stores/" + url.PathEscape(fs.Arg(1)) + "/restore.zip?overwrite=" + fmt.Sprint(*overwrite)
	resp, err := httpClient.Post(endpoint, "application/zip", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("restore failed: %s", body)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return err
	}
	if *overwrite {
		fmt.Fprintf(out, "restored %v secrets into %s (overwritten)\n", payload["imported"], fs.Arg(1))
	} else {
		fmt.Fprintf(out, "restored %v secrets into %s\n", payload["imported"], fs.Arg(1))
	}
	fmt.Fprintln(out, "store is sealed; unseal it before use")
	return nil
}

func runImportBitwarden(baseURL string, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("import bitwarden", flag.ContinueOnError)
	all := fs.Bool("all", false, "import all available Bitwarden items")
	item := fs.String("item", "", "import only items whose name or id matches this value")
	prefix := fs.String("prefix", "", "optional prefix for generated keys")
	storeName := fs.String("store", stores.DefaultStoreName, "target vaultline store")
	dryRun := fs.Bool("dry-run", false, "show what would be imported without storing anything")
	addMissing := fs.Bool("add-missing-keys", true, "add keys that do not already exist in vaultline")
	overwriteExisting := fs.Bool("overwrite-existing-keys", false, "overwrite keys that already exist in vaultline")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprintln(out, importSubcommandHelp("bitwarden"))
			return nil
		}
		return err
	}
	if !*all && strings.TrimSpace(*item) == "" {
		return fmt.Errorf("choose either --all or --item")
	}
	items, err := loadBitwardenItems()
	if err != nil {
		return err
	}
	folders, err := loadBitwardenFolders()
	if err != nil {
		return err
	}
	filter := strings.TrimSpace(*item)
	added := 0
	updated := 0
	skippedExisting := 0
	skippedMissing := 0
	invalid := 0
	shownRenames := map[string]bool{}
	for _, item := range items {
		if filter != "" && item.Name != filter && item.ID != filter {
			continue
		}
		entries := bitwardenEntries(item, folderNameForItem(item, folders), *prefix, *storeName)
		for _, entry := range entries {
			if entry.OriginalBase != entry.Base && !shownRenames[entry.OriginalBase+"->"+entry.Base] {
				fmt.Fprintf(out, "%s -> %s\n", entry.OriginalBase, entry.Base)
				shownRenames[entry.OriginalBase+"->"+entry.Base] = true
			}
			storeNameResolved, keyName, err := parseQualifiedKey(entry.Key)
			if err != nil {
				fmt.Fprintf(out, "skipping %s: %v\n", entry.Key, err)
				invalid++
				continue
			}
			if storeNameResolved == "" || keyName == "" || !importKeyPattern.MatchString(keyName) {
				fmt.Fprintf(out, "skipping %s: violates vaultline naming rules; rename the Bitwarden item, folder, or field\n", entry.Key)
				invalid++
				continue
			}
			exists, err := secretExists(baseURL, entry.Key)
			if err != nil {
				return err
			}
			action := chooseImportAction(exists, *addMissing, *overwriteExisting)
			switch action {
			case importActionSkipExisting:
				skippedExisting++
				continue
			case importActionSkipMissing:
				skippedMissing++
				continue
			case importActionAdd:
				if *dryRun {
					fmt.Fprintf(out, "would add %s\n", entry.Key)
					added++
					continue
				}
				if err := putSecret(baseURL, entry.Key, entry.Value); err != nil {
					return err
				}
				added++
			case importActionUpdate:
				if *dryRun {
					fmt.Fprintf(out, "would update %s\n", entry.Key)
					updated++
					continue
				}
				if err := putSecret(baseURL, entry.Key, entry.Value); err != nil {
					return err
				}
				updated++
			}
		}
	}
	if *dryRun {
		fmt.Fprintf(out, "dry-run complete: added=%d updated=%d skipped-existing=%d skipped-missing=%d invalid=%d\n", added, updated, skippedExisting, skippedMissing, invalid)
		return nil
	}
	fmt.Fprintf(out, "added=%d updated=%d skipped-existing=%d skipped-missing=%d invalid=%d\n", added, updated, skippedExisting, skippedMissing, invalid)
	return nil
}

func loadBitwardenItems() ([]bitwardenItem, error) {
	cmd := execCommand("bw", "list", "items")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("bitwarden import requires an unlocked bw CLI session: %w", err)
	}
	var items []bitwardenItem
	if err := json.Unmarshal(output, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func loadBitwardenFolders() (map[string]string, error) {
	cmd := execCommand("bw", "list", "folders")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("bitwarden import requires folder access via bw: %w", err)
	}
	var folders []bitwardenFolder
	if err := json.Unmarshal(output, &folders); err != nil {
		return nil, err
	}
	result := make(map[string]string, len(folders))
	for _, folder := range folders {
		result[folder.ID] = strings.TrimSpace(folder.Name)
	}
	return result, nil
}

func loadStoreManager() (*stores.Manager, error) {
	return stores.NewManager(resolveStoreConfigPath(), resolveLocalStorePath())
}

func defaultExportZipName(storeName string) string {
	return fmt.Sprintf("%s-%s.zip", storeName, time.Now().Format("2006-01-02"))
}

func chooseImportAction(exists, addMissing, overwriteExisting bool) importAction {
	if exists {
		if overwriteExisting {
			return importActionUpdate
		}
		return importActionSkipExisting
	}
	if addMissing {
		return importActionAdd
	}
	return importActionSkipMissing
}

func folderNameForItem(item bitwardenItem, folders map[string]string) string {
	if item.FolderID == "" {
		return "nogroup"
	}
	if name := normalizeImportComponent(folders[item.FolderID]); name != "" {
		return name
	}
	return "nogroup"
}

func bitwardenEntries(item bitwardenItem, groupName, prefix, storeName string) []importEntry {
	originalParts := []string{}
	if trimmedPrefix := strings.TrimSpace(prefix); trimmedPrefix != "" {
		originalParts = append(originalParts, trimmedPrefix)
	}
	originalParts = append(originalParts, strings.TrimSpace(groupName), strings.TrimSpace(item.Name))
	originalBase := strings.Join(originalParts, ".")
	parts := []string{}
	if normalizedPrefix := strings.Trim(normalizeImportComponent(prefix), "."); normalizedPrefix != "" {
		parts = append(parts, normalizedPrefix)
	}
	parts = append(parts, normalizeImportComponent(groupName), normalizeImportComponent(item.Name))
	base := strings.Join(parts, ".")
	entries := []importEntry{}
	appendEntry := func(suffix string, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		entries = append(entries, importEntry{
			OriginalKey:  storeName + ":" + originalBase + suffix,
			Key:          storeName + ":" + base + suffix,
			OriginalBase: storeName + ":" + originalBase,
			Base:         storeName + ":" + base,
			Value:        []byte(value),
		})
	}
	appendEntry(".username", item.Login.Username)
	appendEntry(".password", item.Login.Password)
	appendEntry(".totp", item.Login.Totp)
	appendEntry(".note", item.Notes)
	for index, uri := range item.Login.URIs {
		suffix := ".uri"
		if index > 0 {
			suffix = fmt.Sprintf(".uri.%s", alphabeticOrdinal(index+1))
		}
		appendEntry(suffix, uri.URI)
	}
	for _, field := range item.Fields {
		appendEntry(".field."+normalizeImportComponent(field.Name), field.Value)
	}
	return entries
}

func normalizeImportComponent(value string) string {
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, "/", ".")
	value = strings.ReplaceAll(value, ":", ".")
	value = strings.ReplaceAll(value, ",", ".")
	value = strings.ReplaceAll(value, "!", ".")
	value = strings.ReplaceAll(value, "(", ".")
	value = strings.ReplaceAll(value, ")", ".")
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.Join(strings.Fields(value), "")
	for strings.Contains(value, "..") {
		value = strings.ReplaceAll(value, "..", ".")
	}
	value = strings.Trim(value, ".")
	return value
}

func alphabeticOrdinal(index int) string {
	if index <= 0 {
		return "a"
	}
	value := ""
	for index > 0 {
		index--
		value = string(rune('a'+(index%26))) + value
		index /= 26
	}
	return value
}

func secretExists(baseURL, qualifiedKey string) (bool, error) {
	storeName, key, err := parseQualifiedKey(qualifiedKey)
	if err != nil {
		return false, err
	}
	resp, err := httpClient.Get(secretEndpoint(baseURL, storeName, key))
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	body, _ := io.ReadAll(resp.Body)
	return false, fmt.Errorf("lookup failed: %s", body)
}

func zipDirectory(sourceDir, targetZip string) error {
	if err := os.MkdirAll(filepath.Dir(targetZip), 0o755); err != nil {
		return err
	}
	file, err := os.Create(targetZip)
	if err != nil {
		return err
	}
	defer file.Close()
	archive := zip.NewWriter(file)
	defer archive.Close()
	return filepath.Walk(sourceDir, func(pathName string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(sourceDir, pathName)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)
		if info.IsDir() {
			header.Name += "/"
			_, err = archive.CreateHeader(header)
			return err
		}
		header.Method = zip.Deflate
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}
		src, err := os.Open(pathName)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(writer, src)
		return err
	})
}

func unzipToDirectory(zipPath, targetDir string) error {
	archive, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer archive.Close()
	for _, file := range archive.File {
		targetPath := filepath.Join(targetDir, filepath.Clean(file.Name))
		if !strings.HasPrefix(targetPath, filepath.Clean(targetDir)+string(os.PathSeparator)) && filepath.Clean(targetPath) != filepath.Clean(targetDir) {
			return fmt.Errorf("invalid zip path %q", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, file.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		src, err := file.Open()
		if err != nil {
			return err
		}
		dst, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
		if err != nil {
			src.Close()
			return err
		}
		_, copyErr := io.Copy(dst, src)
		closeErr := dst.Close()
		src.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func putSecret(baseURL, qualifiedKey string, value []byte) error {
	storeName, key, err := parseQualifiedKey(qualifiedKey)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(api.SecretRequest{Value: base64.StdEncoding.EncodeToString(value)})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, secretEndpoint(baseURL, storeName, key), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("put secret failed: %s", body)
	}
	return nil
}

func runComplete(args []string, out io.Writer) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: vaultline __complete <current> [words...]")
	}
	current := args[0]
	words := args[1:]
	for _, item := range completeWords(words, current) {
		fmt.Fprintln(out, item)
	}
	return nil
}

func bashCompletionScript() string {
	return `# bash completion for vaultline
_vaultline_complete() {
  local cur
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  local wordbreaks="$COMP_WORDBREAKS"
  COMP_WORDBREAKS=${COMP_WORDBREAKS//:}
  local out
  out="$(${COMP_WORDS[0]} __complete "$cur" "${COMP_WORDS[@]:1:$COMP_CWORD}" 2>/dev/null)"
  COMP_WORDBREAKS="$wordbreaks"
  if [[ -n "$out" ]]; then
    mapfile -t COMPREPLY < <(compgen -W "$out" -- "$cur")
    if [[ ${#COMPREPLY[@]} -gt 0 ]]; then
      local nospace=0
      local candidate
      for candidate in "${COMPREPLY[@]}"; do
        if [[ "$candidate" == *: || "$candidate" == *. ]]; then
          nospace=1
          break
        fi
      done
      if [[ "$nospace" -eq 1 ]]; then
        compopt -o nospace 2>/dev/null || true
      fi
    fi
  fi
}
complete -o default -F _vaultline_complete vaultline
`
}

func zshCompletionScript() string {
	return `#compdef vaultline
_vaultline_complete() {
  local cur
  cur="${words[CURRENT]}"
  local -a prev
  if (( CURRENT > 2 )); then
    prev=("${(@)words[2,CURRENT-1]}")
  else
    prev=()
  fi
  local -a suggestions
  suggestions=("${(@f)$(vaultline __complete "$cur" "${prev[@]}" 2>/dev/null)}")
  if (( ${#suggestions[@]} )); then
    local nospace=0
    local item
    for item in ${suggestions[@]}; do
      if [[ "$item" == *: || "$item" == *. ]]; then
        nospace=1
        break
      fi
    done
    if (( nospace )); then
      compadd -Q -S '' -- ${suggestions[@]}
    else
      _describe 'vaultline' suggestions
    fi
  else
    _files
  fi
}
compdef _vaultline_complete vaultline
`
}

func completeWords(words []string, current string) []string {
	if len(words) == 0 {
		return filterCompletions([]string{"health", "unseal", "seal", "daemon-stop", "daemon", "store", "secret", "import", "export", "backup", "restore", "completion", "--addr", "--output", "--help"}, current)
	}
	switch words[0] {
	case "completion":
		return filterCompletions([]string{"bash", "zsh"}, current)
	case "import":
		return completeImportWords(words[1:], current)
	case "export":
		return completeExportWords(words[1:], current)
	case "backup":
		return completeBackupWords(words[1:], current)
	case "restore":
		return completeRestoreWords(words[1:], current)
	case "store":
		return completeStoreWords(words[1:], current)
	case "daemon":
		return completeDaemonWords(words[1:], current)
	case "secret":
		return completeSecretWords(words[1:], current)
	case "seal":
		return filterCompletions([]string{"--keep-keys", "--help"}, current)
	case "unseal":
		return filterCompletions([]string{"--help"}, current)
	default:
		return nil
	}
}

func completeImportWords(words []string, current string) []string {
	if len(words) == 0 {
		return filterCompletions([]string{"bitwarden", "zip", "--help"}, current)
	}
	switch words[0] {
	case "zip":
		if len(words) == 1 {
			return nil
		}
		if len(words) == 2 {
			return filterCompletions(listConfiguredStoresFn(true), current)
		}
		return nil
	case "bitwarden":
	default:
		return nil
	}
	if len(words) == 1 {
		return filterCompletions([]string{"--all", "--item", "--prefix", "--store", "--dry-run", "--add-missing-keys", "--overwrite-existing-keys", "--help"}, current)
	}
	if len(words) > 1 && words[len(words)-1] == "--store" {
		return filterCompletions(listConfiguredStoresFn(true), current)
	}
	return filterCompletions([]string{"--all", "--item", "--prefix", "--store", "--dry-run", "--add-missing-keys", "--overwrite-existing-keys", "--help"}, current)
}

func completeExportWords(words []string, current string) []string {
	if len(words) == 0 {
		return filterCompletions([]string{"zip", "--help"}, current)
	}
	if words[0] != "zip" {
		return nil
	}
	if len(words) == 1 {
		return filterCompletions(listConfiguredStoresFn(true), current)
	}
	return nil
}

func completeBackupWords(words []string, current string) []string {
	if len(words) == 0 {
		return filterCompletions([]string{"zip", "--help"}, current)
	}
	if words[0] != "zip" {
		return nil
	}
	if len(words) == 1 {
		return filterCompletions(listConfiguredStoresFn(true), current)
	}
	return nil
}

func completeRestoreWords(words []string, current string) []string {
	if len(words) == 0 {
		return filterCompletions([]string{"zip", "--help"}, current)
	}
	if words[0] != "zip" {
		return nil
	}
	if len(words) == 1 {
		return nil
	}
	if len(words) == 2 {
		return filterCompletions(listConfiguredStoresFn(true), current)
	}
	return filterCompletions([]string{"--overwrite", "--help"}, current)
}

func completeStoreWords(words []string, current string) []string {
	storeNames := listConfiguredStoresFn(true)
	if len(words) == 0 {
		return filterCompletions([]string{"add", "init", "list", "show", "unseal", "seal", "delete", "remove", "rm", "--help"}, current)
	}
	sub := words[0]
	if len(words) == 1 {
		switch sub {
		case "show", "unseal", "seal":
			return filterCompletions(storeNames, current)
		case "delete", "remove", "rm":
			return filterCompletions(filterOut(storeNames, stores.DefaultStoreName), current)
		case "add", "init":
			return nil
		case "list":
			return filterCompletions([]string{"--help"}, current)
		}
	}
	if sub == "seal" && len(words) >= 2 {
		return filterCompletions([]string{"--keep-keys", "--help"}, current)
	}
	return nil
}

func completeDaemonWords(words []string, current string) []string {
	binds := listDaemonBindAddrs()
	if len(words) == 0 {
		return filterCompletions([]string{"bind", "list-binds", "unbind", "allow", "list-allows", "unallow", "--help"}, current)
	}
	sub := words[0]
	if len(words) == 1 {
		switch sub {
		case "unbind", "list-allows", "allow", "unallow":
			return filterCompletions(binds, current)
		case "bind":
			return nil
		case "list-binds":
			return filterCompletions([]string{"--help"}, current)
		}
	}
	if sub == "unallow" && len(words) == 2 {
		return filterCompletions(listDaemonAllows(words[1]), current)
	}
	return nil
}

func completeSecretWords(words []string, current string) []string {
	storePrefixes := listStorePrefixes()
	if len(words) == 0 {
		return filterCompletions([]string{"set", "get", "delete", "delete-prefix", "glob", "list", "--help"}, current)
	}
	sub := words[0]
	flagsForSet := []string{"--name", "--value", "--file", "--stdin", "--twice", "--help"}
	flagsForGet := []string{"--name", "--out", "--help"}
	flagsForDelete := []string{"--name", "--help"}
	flagsForDeletePrefix := []string{"--dry-run", "--yes", "--help"}
	if sub == "set" || sub == "get" || sub == "delete" || sub == "delete-prefix" {
		if strings.Contains(current, ":") || (len(words) > 1 && words[len(words)-1] == "--name") {
			return completeQualifiedSecret(current)
		}
	}
	if len(words) == 1 {
		switch sub {
		case "set", "get", "delete", "delete-prefix", "glob":
			return filterCompletions(append(storePrefixes, "--name", "--help"), current)
		case "list":
			return filterCompletions(append(storePrefixes, "--help"), current)
		}
	}
	if sub == "set" {
		return filterCompletions(append(storePrefixes, flagsForSet...), current)
	}
	if sub == "get" {
		return filterCompletions(append(storePrefixes, flagsForGet...), current)
	}
	if sub == "delete" {
		return filterCompletions(append(storePrefixes, flagsForDelete...), current)
	}
	if sub == "delete-prefix" {
		return filterCompletions(append(storePrefixes, flagsForDeletePrefix...), current)
	}
	if sub == "glob" {
		return filterCompletions(append(storePrefixes, "--help"), current)
	}
	if sub == "list" {
		return filterCompletions(append(storePrefixes, "--help"), current)
	}
	return nil
}

func completeQualifiedSecret(current string) []string {
	storeName, _, err := parseQualifiedKey(current)
	if err != nil {
		if strings.HasSuffix(current, ":") {
			store := strings.TrimSuffix(current, ":")
			if store == "" {
				return listStorePrefixes()
			}
			storeName = store
		} else {
			return nil
		}
	}
	keys := listSecretKeys(storeName)
	return filterCompletions(buildQualifiedKeyCompletions(storeName, current, keys), current)
}

func listSecretKeys(storeName string) []string {
	baseURL, err := buildBaseURL("127.0.0.1:8428")
	if err != nil {
		return nil
	}
	resp, err := httpClient.Get(secretListEndpoint(baseURL, storeName))
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var payload struct {
		Keys []listEntry `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil
	}
	keys := make([]string, 0, len(payload.Keys))
	for _, entry := range payload.Keys {
		keys = append(keys, entry.Name)
	}
	return keys
}

func buildQualifiedKeyCompletions(storeName, current string, keys []string) []string {
	_, keyCurrent, err := parseQualifiedKey(current)
	if err != nil {
		if strings.HasSuffix(current, ":") {
			keyCurrent = ""
		} else {
			keyCurrent = strings.TrimPrefix(current, storeName+":")
		}
	}
	basePrefix := ""
	partial := keyCurrent
	if strings.Contains(keyCurrent, ".") {
		lastDot := strings.LastIndex(keyCurrent, ".")
		basePrefix = keyCurrent[:lastDot+1]
		partial = keyCurrent[lastDot+1:]
	}
	if strings.HasSuffix(keyCurrent, ".") {
		basePrefix = keyCurrent
		partial = ""
	}
	seen := map[string]bool{}
	results := make([]string, 0, len(keys))
	for _, key := range keys {
		if !strings.HasPrefix(key, basePrefix) {
			continue
		}
		remainder := strings.TrimPrefix(key, basePrefix)
		if remainder == "" {
			continue
		}
		parts := strings.SplitN(remainder, ".", 2)
		next := parts[0]
		if partial != "" && !strings.HasPrefix(next, partial) {
			continue
		}
		candidate := basePrefix + next
		if len(parts) > 1 {
			candidate += "."
		}
		qualified := storeName + ":" + candidate
		if !seen[qualified] {
			seen[qualified] = true
			results = append(results, qualified)
		}
	}
	sort.Strings(results)
	return results
}

func filterCompletions(items []string, current string) []string {
	seen := map[string]bool{}
	filtered := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" || seen[item] {
			continue
		}
		if current == "" || strings.HasPrefix(item, current) {
			seen[item] = true
			filtered = append(filtered, item)
		}
	}
	sort.Strings(filtered)
	return filtered
}

func listConfiguredStores(includeLocal bool) []string {
	localStore := resolveLocalStorePath()
	cfg, err := stores.LoadConfig(resolveStoreConfigPath(), localStore)
	if err != nil {
		if includeLocal {
			return []string{stores.DefaultStoreName}
		}
		return nil
	}
	names := make([]string, 0, len(cfg.Stores))
	for name := range cfg.Stores {
		if !includeLocal && name == stores.DefaultStoreName {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func listStorePrefixes() []string {
	names := listConfiguredStoresFn(true)
	prefixes := make([]string, 0, len(names))
	for _, name := range names {
		prefixes = append(prefixes, name+":")
	}
	return prefixes
}

func listDaemonBindAddrs() []string {
	mgr, err := daemoncfg.New(resolveDaemonConfigPath())
	if err != nil {
		return nil
	}
	binds := mgr.ListBinds()
	addrs := make([]string, 0, len(binds))
	for _, bind := range binds {
		addrs = append(addrs, bind.Addr)
	}
	sort.Strings(addrs)
	return addrs
}

func listDaemonAllows(addr string) []string {
	mgr, err := daemoncfg.New(resolveDaemonConfigPath())
	if err != nil {
		return nil
	}
	allows, err := mgr.ListAllows(addr)
	if err != nil {
		return nil
	}
	sort.Strings(allows)
	return allows
}

func resolveLocalStorePath() string {
	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
		return filepath.Join(xdgData, "vaultline", "stores", stores.DefaultStoreName)
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "share", "vaultline", "stores", stores.DefaultStoreName)
}

func resolveStoreConfigPath() string {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "vaultline", "stores.json")
	}
	return filepath.Join(os.Getenv("HOME"), ".config", "vaultline", "stores.json")
}

func defaultNamedStorePath(name string) string {
	return filepath.Join(filepath.Dir(resolveLocalStorePath()), name)
}

func resolveDaemonConfigPath() string {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "vaultline", "daemon.json")
	}
	return filepath.Join(os.Getenv("HOME"), ".config", "vaultline", "daemon.json")
}

func filterOut(items []string, remove string) []string {
	filtered := make([]string, 0, len(items))
	for _, item := range items {
		if item != remove {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func runDaemon(baseURL string, args []string, out io.Writer) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		fmt.Fprintln(out, daemonUsageText())
		if len(args) == 0 {
			return errors.New("daemon command requires subcommand")
		}
		return nil
	}
	if len(args) > 1 && isHelpArg(args[1]) {
		fmt.Fprintln(out, daemonSubcommandHelp(args[0]))
		return nil
	}
	switch args[0] {
	case "bind":
		if len(args) != 2 {
			return fmt.Errorf("usage: vaultline daemon bind <addr>")
		}
		payload, err := json.Marshal(api.DaemonBindRequest{Addr: args[1]})
		if err != nil {
			return err
		}
		resp, err := httpClient.Post(baseURL+"/api/v1/daemon/binds", "application/json", bytes.NewReader(payload))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("daemon bind failed: %s", body)
		}
		fmt.Fprintf(out, "bind %s added\n", args[1])
		return nil
	case "list-binds":
		resp, err := httpClient.Get(baseURL + "/api/v1/daemon/binds")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("list-binds failed: %s", body)
		}
		var payload struct {
			Binds []struct {
				Addr   string   `json:"addr"`
				Allows []string `json:"allows"`
			} `json:"binds"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return err
		}
		rows := make([][]string, 0, len(payload.Binds))
		for _, bind := range payload.Binds {
			state := "blocked"
			if len(bind.Allows) > 0 {
				state = "allowlisted"
			}
			rows = append(rows, []string{bind.Addr, fmt.Sprint(len(bind.Allows)), state})
		}
		printTable([]string{"BIND", "ALLOWS", "STATE"}, rows, out)
		return nil
	case "unbind":
		if len(args) != 2 {
			return fmt.Errorf("usage: vaultline daemon unbind <addr>")
		}
		req, err := http.NewRequest(http.MethodDelete, baseURL+"/api/v1/daemon/binds/"+url.PathEscape(args[1]), nil)
		if err != nil {
			return err
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("daemon unbind failed: %s", body)
		}
		fmt.Fprintf(out, "bind %s removed\n", args[1])
		return nil
	case "allow":
		if len(args) != 3 {
			return fmt.Errorf("usage: vaultline daemon allow <addr> <cidr-or-ip>")
		}
		payload, err := json.Marshal(api.DaemonAllowRequest{Rule: args[2]})
		if err != nil {
			return err
		}
		resp, err := httpClient.Post(baseURL+"/api/v1/daemon/binds/"+url.PathEscape(args[1])+"/allows", "application/json", bytes.NewReader(payload))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("daemon allow failed: %s", body)
		}
		fmt.Fprintf(out, "allow %s added to %s\n", args[2], args[1])
		return nil
	case "list-allows":
		if len(args) != 2 {
			return fmt.Errorf("usage: vaultline daemon list-allows <addr>")
		}
		resp, err := httpClient.Get(baseURL + "/api/v1/daemon/binds/" + url.PathEscape(args[1]) + "/allows")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("list-allows failed: %s", body)
		}
		var payload struct {
			Addr   string   `json:"addr"`
			Allows []string `json:"allows"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return err
		}
		if len(payload.Allows) == 0 {
			fmt.Fprintf(out, "%s has no allow rules\n", payload.Addr)
			return nil
		}
		rows := make([][]string, 0, len(payload.Allows))
		for _, rule := range payload.Allows {
			rows = append(rows, []string{payload.Addr, rule})
		}
		printTable([]string{"BIND", "ALLOW"}, rows, out)
		return nil
	case "unallow":
		if len(args) != 3 {
			return fmt.Errorf("usage: vaultline daemon unallow <addr> <cidr-or-ip>")
		}
		req, err := http.NewRequest(http.MethodDelete, baseURL+"/api/v1/daemon/binds/"+url.PathEscape(args[1])+"/allows/"+url.PathEscape(args[2]), nil)
		if err != nil {
			return err
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("daemon unallow failed: %s", body)
		}
		fmt.Fprintf(out, "allow %s removed from %s\n", args[2], args[1])
		return nil
	default:
		return fmt.Errorf("unknown daemon subcommand %q", args[0])
	}
}

func runStore(baseURL string, args []string, out io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintln(out, storeUsageText())
		return errors.New("store command requires subcommand")
	}
	if isHelpArg(args[0]) {
		fmt.Fprintln(out, storeUsageText())
		return nil
	}
	if len(args) > 1 && isHelpArg(args[1]) {
		fmt.Fprintln(out, storeSubcommandHelp(args[0]))
		return nil
	}
	switch args[0] {
	case "list":
		resp, err := httpClient.Get(baseURL + "/api/v1/stores")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("store list failed: %s", body)
		}
		var payload struct {
			Stores []struct {
				Name      string `json:"name"`
				Path      string `json:"path"`
				Default   bool   `json:"default"`
				Available bool   `json:"available"`
				Sealed    bool   `json:"sealed"`
				HasKey    bool   `json:"has_key"`
				Error     string `json:"error"`
			} `json:"stores"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return err
		}
		rows := make([][]string, 0, len(payload.Stores))
		for _, store := range payload.Stores {
			errValue := sanitizeStoreError(store.Error)
			rows = append(rows, []string{store.Name, store.Path, fmt.Sprint(store.Available), fmt.Sprint(store.Sealed), fmt.Sprint(store.Default), errValue})
		}
		printTable([]string{"STORE", "PATH", "AVAILABLE", "SEALED", "DEFAULT", "ERROR"}, rows, out)
		return nil
	case "show":
		if len(args) != 2 {
			return errors.New("usage: vaultline store show <name>")
		}
		resp, err := httpClient.Get(baseURL + "/api/v1/stores/" + url.PathEscape(args[1]))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("store show failed: %s", body)
		}
		fmt.Fprintln(out, string(body))
		return nil
	case "add", "init":
		if args[0] == "add" && len(args) != 3 {
			return fmt.Errorf("usage: vaultline store add <name> <path>")
		}
		if args[0] == "init" && (len(args) < 2 || len(args) > 3) {
			return fmt.Errorf("usage: vaultline store init <name> [path]")
		}
		path := ""
		if len(args) == 3 {
			path = args[2]
		} else if args[0] == "init" {
			path = defaultNamedStorePath(args[1])
		}
		payload, err := json.Marshal(api.StoreCreateRequest{Name: args[1], Path: path, Initialize: args[0] == "init"})
		if err != nil {
			return err
		}
		resp, err := httpClient.Post(baseURL+"/api/v1/stores", "application/json", bytes.NewReader(payload))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("store %s failed: %s", args[0], body)
		}
		var created api.StoreCreateResponse
		if err := json.Unmarshal(body, &created); err != nil {
			return fmt.Errorf("unexpected store %s response: %s", args[0], body)
		}
		fmt.Fprintf(out, "store %s ready\n", args[1])
		if args[0] == "init" {
			fmt.Fprintf(out, "unseal key: %s\n", created.Passphrase)
			fmt.Fprintln(out, "stored in config and immediately unsealed")
		}
		return nil
	case "delete", "remove", "rm":
		if len(args) != 2 {
			return fmt.Errorf("usage: vaultline store %s <name>", args[0])
		}
		req, err := http.NewRequest(http.MethodDelete, baseURL+"/api/v1/stores/"+url.PathEscape(args[1]), nil)
		if err != nil {
			return err
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("store delete failed: %s", body)
		}
		fmt.Fprintf(out, "store %s removed\n", args[1])
		return nil
	case "unseal", "seal":
		storeArg, flagArgs := splitKeyArg(args[1:], map[string]bool{})
		fs := flag.NewFlagSet("store "+args[0], flag.ContinueOnError)
		keepKeys := fs.Bool("keep-keys", false, "keep remembered passphrases in store config")
		fs.SetOutput(io.Discard)
		if err := fs.Parse(flagArgs); err != nil {
			if err == flag.ErrHelp {
				fmt.Fprintln(out, storeSubcommandHelp(args[0]))
				return nil
			}
			return err
		}
		storeRaw := storeArg
		if storeRaw == "" && fs.NArg() > 0 {
			storeRaw = fs.Arg(0)
		}
		if storeRaw == "" || fs.NArg() > 1 {
			return fmt.Errorf("usage: vaultline store %s <name>", args[0])
		}
		storeName := url.PathEscape(storeRaw)
		if args[0] == "seal" {
			payload, err := json.Marshal(api.SealRequest{KeepKeys: *keepKeys})
			if err != nil {
				return err
			}
			req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/stores/"+storeName+"/seal", bytes.NewReader(payload))
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := httpClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("store seal failed: %s", body)
			}
			fmt.Fprintf(out, "store %s sealed\n", storeRaw)
			return nil
		}
		endpoint := baseURL + "/api/v1/stores/" + storeName + "/unseal"
		body, status, err := tryUnseal(endpoint, "")
		if err != nil {
			return err
		}
		if status == http.StatusOK {
			fmt.Fprintf(out, "store %s unsealed\n", storeRaw)
			return nil
		}
		if !strings.Contains(string(body), "passphrase required") {
			return fmt.Errorf("store unseal failed: %s", body)
		}
		passphrase, err := readPassphrase()
		if err != nil {
			return err
		}
		body, status, err = tryUnseal(endpoint, passphrase)
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("store unseal failed: %s", body)
		}
		fmt.Fprintf(out, "store %s unsealed\n", storeRaw)
		return nil
	default:
		return fmt.Errorf("unknown store subcommand %q", args[0])
	}
}

func runSecret(baseURL string, args []string, outputFmt string, out io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintln(out, secretUsageText())
		return errors.New("secret command requires subcommand")
	}
	if isHelpArg(args[0]) {
		fmt.Fprintln(out, secretUsageText())
		return nil
	}
	if len(args) > 1 && isHelpArg(args[1]) {
		fmt.Fprintln(out, secretSubcommandHelp(args[0]))
		return nil
	}
	switch args[0] {
	case "set":
		return secretSet(baseURL, args[1:], out)
	case "get":
		return secretGet(baseURL, args[1:], outputFmt, out)
	case "delete":
		return secretDelete(baseURL, args[1:], out)
	case "delete-prefix":
		return secretDeletePrefix(baseURL, args[1:], out)
	case "glob":
		return secretGlob(baseURL, args[1:], outputFmt, out)
	case "list":
		return secretList(baseURL, args[1:], outputFmt, out)
	default:
		return fmt.Errorf("unknown secret subcommand %q", args[0])
	}
}

func parseQualifiedKey(input string) (string, string, error) {
	parts := strings.SplitN(strings.TrimSpace(input), ":", 2)
	if len(parts) == 1 {
		if parts[0] == "" {
			return "", "", fmt.Errorf("secret key required")
		}
		return stores.DefaultStoreName, parts[0], nil
	}
	if parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("qualified keys must look like store:key")
	}
	return parts[0], parts[1], nil
}

func parseStoreSelector(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return stores.DefaultStoreName, nil
	}
	if !strings.HasSuffix(trimmed, ":") {
		return "", fmt.Errorf("store selector must look like store:")
	}
	name := strings.TrimSuffix(trimmed, ":")
	if name == "" {
		return "", fmt.Errorf("store selector must look like store:")
	}
	return name, nil
}

func secretEndpoint(baseURL, storeName, key string) string {
	return fmt.Sprintf("%s/api/v1/stores/%s/secrets/%s", baseURL, url.PathEscape(storeName), url.PathEscape(key))
}

func secretListEndpoint(baseURL, storeName string) string {
	return fmt.Sprintf("%s/api/v1/stores/%s/secrets", baseURL, url.PathEscape(storeName))
}

func secretSet(baseURL string, args []string, out io.Writer) error {
	keyArg, flagArgs := splitKeyArg(args, map[string]bool{"--value": true, "--file": true, "--name": true})
	fs := flag.NewFlagSet("secret set", flag.ContinueOnError)
	name := fs.String("name", "", "secret identifier (lowercase letters, digits, @, dot, dash)")
	value := fs.String("value", "", "literal secret value")
	filePath := fs.String("file", "", "path to file")
	useStdin := fs.Bool("stdin", false, "read secret from stdin (mask prompt when running interactively)")
	confirmTwice := fs.Bool("twice", false, "when prompting via --stdin, require the secret to be entered twice")
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	key := strings.TrimSpace(*name)
	if key == "" {
		switch {
		case keyArg != "":
			key = keyArg
		case fs.NArg() > 0:
			key = fs.Arg(0)
		default:
			return fmt.Errorf("provide a key via --name or as an argument")
		}
	}
	storeName, key, err := parseQualifiedKey(key)
	if err != nil {
		return err
	}
	data, err := readSecretInput(*value, *filePath, *useStdin, *confirmTwice)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return fmt.Errorf("provide a secret via --value, --file, or --stdin")
	}
	req := api.SecretRequest{Value: base64.StdEncoding.EncodeToString(data)}
	payload, err := json.Marshal(req)
	if err != nil {
		return err
	}
	endpoint := secretEndpoint(baseURL, storeName, key)
	httpReq, err := http.NewRequest(http.MethodPut, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("put secret failed: %s", body)
	}
	var version api.VersionResponse
	if err := json.Unmarshal(body, &version); err != nil {
		return fmt.Errorf("unexpected response: %s", body)
	}
	fmt.Fprintf(out, "secret stored in %s (version=%s)\n", storeName, version.Version)
	return nil
}

func secretGet(baseURL string, args []string, outputFmt string, out io.Writer) error {
	keyArg, flagArgs := splitKeyArg(args, map[string]bool{"--out": true, "--name": true})
	fs := flag.NewFlagSet("secret get", flag.ContinueOnError)
	name := fs.String("name", "", "secret identifier")
	outputPath := fs.String("out", "", "write secret to file (default stdout)")
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	key := strings.TrimSpace(*name)
	if key == "" {
		switch {
		case keyArg != "":
			key = keyArg
		case fs.NArg() > 0:
			key = fs.Arg(0)
		default:
			return fmt.Errorf("provide a key via --name or as an argument")
		}
	}
	storeName, key, err := parseQualifiedKey(key)
	if err != nil {
		return err
	}
	resp, err := httpClient.Get(secretEndpoint(baseURL, storeName, key))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get secret failed: %s", body)
	}
	var payload api.SecretResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return err
	}
	bytesValue, err := base64.StdEncoding.DecodeString(payload.Value)
	if err != nil {
		return err
	}
	if outputFmt == "json" && *outputPath == "" {
		fmt.Fprintln(out, string(body))
		return nil
	}
	if *outputPath == "" || *outputPath == "-" {
		out.Write(bytesValue)
		if outputFmt != "raw" {
			fmt.Fprintln(out)
		}
		return nil
	}
	return os.WriteFile(*outputPath, bytesValue, 0o600)
}

func secretDelete(baseURL string, args []string, out io.Writer) error {
	keyArg, flagArgs := splitKeyArg(args, map[string]bool{"--name": true})
	fs := flag.NewFlagSet("secret delete", flag.ContinueOnError)
	name := fs.String("name", "", "secret identifier")
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	key := strings.TrimSpace(*name)
	if key == "" {
		switch {
		case keyArg != "":
			key = keyArg
		case fs.NArg() > 0:
			key = fs.Arg(0)
		default:
			return fmt.Errorf("provide a key via --name or as an argument")
		}
	}
	storeName, key, err := parseQualifiedKey(key)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodDelete, secretEndpoint(baseURL, storeName, key), nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete failed: %s", body)
	}
	fmt.Fprintf(out, "secret removed from %s\n", storeName)
	return nil
}

func secretDeletePrefix(baseURL string, args []string, out io.Writer) error {
	prefixArg, flagArgs := splitKeyArg(args, map[string]bool{})
	fs := flag.NewFlagSet("secret delete-prefix", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "show matching keys without deleting them")
	confirm := fs.Bool("yes", false, "delete all matching keys without prompting")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	prefixInput := prefixArg
	if prefixInput == "" && fs.NArg() > 0 {
		prefixInput = fs.Arg(0)
	}
	if prefixInput == "" || fs.NArg() > 1 {
		return fmt.Errorf("usage: vaultline secret delete-prefix <store:prefix.> [--dry-run] [--yes]")
	}
	storeName, prefix, err := parseQualifiedKey(prefixInput)
	if err != nil {
		return err
	}
	if !strings.HasSuffix(prefix, ".") {
		return fmt.Errorf("prefix must end with a dot")
	}
	entries, err := listSecrets(baseURL, storeName)
	if err != nil {
		return err
	}
	matches := make([]string, 0)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name, prefix) {
			matches = append(matches, entry.Name)
		}
	}
	if len(matches) == 0 {
		return fmt.Errorf("no secrets matched %s:%s", storeName, prefix)
	}
	for _, match := range matches {
		fmt.Fprintf(out, "%s:%s\n", storeName, match)
	}
	if *dryRun {
		fmt.Fprintf(out, "dry-run complete: %d matching secrets\n", len(matches))
		return nil
	}
	if !*confirm {
		return fmt.Errorf("refusing to delete %d secrets without --yes (or use --dry-run first)", len(matches))
	}
	for _, match := range matches {
		req, err := http.NewRequest(http.MethodDelete, secretEndpoint(baseURL, storeName, match), nil)
		if err != nil {
			return err
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			return err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("delete-prefix failed for %s:%s: %s", storeName, match, body)
		}
	}
	fmt.Fprintf(out, "deleted %d secrets from %s\n", len(matches), storeName)
	return nil
}

func secretGlob(baseURL string, args []string, outputFmt string, out io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: vaultline secret glob <store-glob:key-glob>")
	}
	storePattern, keyPattern := parseGlobPattern(args[0])
	storeNames := listConfiguredStoresFn(true)
	entries := make([]listEntry, 0)
	for _, storeName := range storeNames {
		matched, err := path.Match(storePattern, storeName)
		if err != nil {
			return err
		}
		if !matched {
			continue
		}
		storeEntries, err := listSecrets(baseURL, storeName)
		if err != nil {
			// unavailable/broken stores are skipped in this convenience search
			continue
		}
		for _, entry := range storeEntries {
			qualified := storeName + ":" + entry.Name
			matched, err := path.Match(keyPattern, entry.Name)
			if err != nil {
				return err
			}
			if !matched {
				matched, err = path.Match(storePattern+":"+keyPattern, qualified)
				if err != nil {
					return err
				}
			}
			if matched {
				entry.Name = qualified
				entries = append(entries, entry)
			}
		}
	}
	if outputFmt == "json" {
		payload := map[string]any{"keys": entries}
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(data))
		return nil
	}
	printSecretTable(entries, out)
	return nil
}

func parseGlobPattern(input string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(input), ":", 2)
	if len(parts) == 1 {
		return "*", parts[0]
	}
	storePattern := parts[0]
	if storePattern == "" {
		storePattern = "*"
	}
	return storePattern, parts[1]
}

func secretList(baseURL string, args []string, outputFmt string, out io.Writer) error {
	storeName := stores.DefaultStoreName
	if len(args) > 0 {
		var err error
		storeName, err = parseStoreSelector(args[0])
		if err != nil {
			return err
		}
	}
	resp, err := httpClient.Get(secretListEndpoint(baseURL, storeName))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("list failed: %s", string(body))
	}
	if outputFmt == "json" {
		fmt.Fprintln(out, string(body))
		return nil
	}
	var payload struct {
		Keys []listEntry `json:"keys"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return err
	}
	for index := range payload.Keys {
		payload.Keys[index].Name = storeName + ":" + payload.Keys[index].Name
	}
	printSecretTable(payload.Keys, out)
	return nil
}

func listSecrets(baseURL, storeName string) ([]listEntry, error) {
	resp, err := httpClient.Get(secretListEndpoint(baseURL, storeName))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list failed: %s", string(body))
	}
	var payload struct {
		Keys []listEntry `json:"keys"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	return payload.Keys, nil
}

func readSecretInput(literal, filePath string, stdin, twice bool) ([]byte, error) {
	switch {
	case stdin:
		if term.IsTerminal(int(syscall.Stdin)) {
			fmt.Print("Secret value: ")
			bytesValue, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			if err != nil {
				return nil, err
			}
			first := []byte(strings.TrimSpace(string(bytesValue)))
			if !twice {
				return first, nil
			}
			fmt.Print("Repeat secret value: ")
			confirmValue, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			if err != nil {
				return nil, err
			}
			return confirmSecretMatch(first, []byte(strings.TrimSpace(string(confirmValue))))
		}
		if twice {
			return nil, fmt.Errorf("--twice requires interactive --stdin input")
		}
		return io.ReadAll(os.Stdin)
	case filePath != "":
		if twice {
			return nil, fmt.Errorf("--twice requires --stdin")
		}
		return os.ReadFile(filePath)
	default:
		if twice {
			return nil, fmt.Errorf("--twice requires --stdin")
		}
		return []byte(literal), nil
	}
}

func confirmSecretMatch(first, second []byte) ([]byte, error) {
	if string(first) != string(second) {
		return nil, fmt.Errorf("secret values do not match")
	}
	return first, nil
}

func promptSecretValue() ([]byte, error) {
	fmt.Print("Secret value: ")
	bytesValue, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return nil, err
	}
	value := strings.TrimSpace(string(bytesValue))
	if value == "" {
		return nil, fmt.Errorf("empty secret value")
	}
	return []byte(value), nil
}

func splitKeyArg(args []string, valueFlags map[string]bool) (string, []string) {
	if valueFlags == nil {
		valueFlags = map[string]bool{}
	}
	key := ""
	filtered := make([]string, 0, len(args))
	expectValue := false
	for _, arg := range args {
		flagName := arg
		if expectValue {
			filtered = append(filtered, arg)
			expectValue = false
			continue
		}
		if strings.HasPrefix(flagName, "--") {
			parts := strings.SplitN(flagName, "=", 2)
			flagName = parts[0]
			filtered = append(filtered, arg)
			if valueFlags[flagName] && len(parts) == 1 {
				expectValue = true
			}
			continue
		}
		if strings.HasPrefix(flagName, "-") {
			filtered = append(filtered, arg)
			continue
		}
		if key == "" {
			key = arg
			continue
		}
		filtered = append(filtered, arg)
	}
	return key, filtered
}

type listEntry struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	UpdatedAt string `json:"updated_at"`
}

func printSecretTable(entries []listEntry, out io.Writer) {
	if len(entries) == 0 {
		fmt.Fprintln(out, "(no secrets)")
		return
	}
	nameW, timeW, verW := len("KEY"), len("UPDATED"), len("VERSION")
	rows := make([][3]string, len(entries))
	for i, entry := range entries {
		updated := entry.UpdatedAt
		if updated == "" {
			updated = "-"
		}
		rows[i] = [3]string{entry.Name, updated, entry.Version}
		if len(entry.Name) > nameW {
			nameW = len(entry.Name)
		}
		if len(updated) > timeW {
			timeW = len(updated)
		}
		if len(entry.Version) > verW {
			verW = len(entry.Version)
		}
	}
	fmt.Fprintf(out, "%-*s  %-*s  %-*s\n", nameW, "KEY", timeW, "UPDATED", verW, "VERSION")
	fmt.Fprintf(out, "%s  %s  %s\n", strings.Repeat("-", nameW), strings.Repeat("-", timeW), strings.Repeat("-", verW))
	for _, row := range rows {
		fmt.Fprintf(out, "%-*s  %-*s  %-*s\n", nameW, row[0], timeW, row[1], verW, row[2])
	}
}

func printKeyValueTable(entries [][2]string, out io.Writer) {
	keyWidth := 0
	for _, entry := range entries {
		if len(entry[0]) > keyWidth {
			keyWidth = len(entry[0])
		}
	}
	for _, entry := range entries {
		fmt.Fprintf(out, "%-*s  %s\n", keyWidth, entry[0], entry[1])
	}
}

func printTable(headers []string, rows [][]string, out io.Writer) {
	const maxWidth = 79
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = len(header)
	}
	for _, row := range rows {
		for i, value := range row {
			if i < len(widths) && len(value) > widths[i] {
				widths[i] = len(value)
			}
		}
	}
	if len(widths) > 0 {
		fixedWidth := 0
		for i := 0; i < len(widths)-1; i++ {
			fixedWidth += widths[i]
		}
		fixedWidth += (len(widths) - 1) * 2
		lastWidth := maxWidth - fixedWidth
		if lastWidth < 20 {
			lastWidth = 20
		}
		if widths[len(widths)-1] > lastWidth {
			widths[len(widths)-1] = lastWidth
		}
	}
	for i, header := range headers {
		fmt.Fprintf(out, "%-*s", widths[i], header)
		if i < len(headers)-1 {
			fmt.Fprint(out, "  ")
		}
	}
	fmt.Fprintln(out)
	separatorWidth := 0
	for _, width := range widths {
		separatorWidth += width
	}
	separatorWidth += (len(widths) - 1) * 2
	if separatorWidth > maxWidth {
		separatorWidth = maxWidth
	}
	fmt.Fprintln(out, strings.Repeat("-", separatorWidth))
	for _, row := range rows {
		wrappedLast := wrapText(row[len(row)-1], widths[len(widths)-1])
		if len(wrappedLast) == 0 {
			wrappedLast = []string{""}
		}
		for lineIndex := range wrappedLast {
			for i := 0; i < len(headers)-1; i++ {
				value := ""
				if lineIndex == 0 {
					value = row[i]
				}
				fmt.Fprintf(out, "%-*s", widths[i], value)
				fmt.Fprint(out, "  ")
			}
			fmt.Fprintf(out, "%-*s", widths[len(widths)-1], wrappedLast[lineIndex])
			fmt.Fprintln(out)
		}
	}
}

func wrapText(value string, width int) []string {
	if width <= 0 || len(value) <= width {
		return []string{value}
	}
	words := strings.Fields(value)
	if len(words) == 0 {
		return []string{""}
	}
	lines := []string{}
	current := ""
	for _, word := range words {
		if current == "" {
			current = word
			continue
		}
		candidate := current + " " + word
		if len(candidate) <= width {
			current = candidate
			continue
		}
		lines = append(lines, current)
		current = word
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func sanitizeStoreError(value string) string {
	value = strings.TrimPrefix(value, "vaultline: store unavailable: ")
	value = strings.TrimPrefix(value, "vaultline: ")
	return value
}

func readPassphrase() (string, error) {
	if pass := os.Getenv("VAULTLINE_PASSPHRASE"); pass != "" {
		return pass, nil
	}
	fmt.Print("Enter passphrase: ")
	bytesPass, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(bytesPass)), nil
}

func buildBaseURL(addr string) (string, error) {
	if addr == "" {
		addr = "127.0.0.1:8428"
	}
	if !strings.Contains(addr, "://") {
		addr = "http://" + addr
	}
	parsed, err := url.Parse(addr)
	if err != nil {
		return "", err
	}
	host := parsed.Hostname()
	port := parsed.Port()
	if host == "" {
		host = "127.0.0.1"
	}
	if port == "" {
		port = "8428"
	}
	if !isLoopback(host) {
		return "", fmt.Errorf("refusing to connect to non-loopback host: %s", host)
	}
	parsed.Host = net.JoinHostPort(host, port)
	return parsed.String(), nil
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
