package cli

import (
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
	"path/filepath"
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

func usageText() string {
	return fmt.Sprintf("vaultline %s\n\n", version.Version) + `Usage:
  vaultline daemon [--addr HOST:PORT] [--store-dir DIR]
      Starts the HTTPS API server. When --store-dir is omitted the daemon
      uses $XDG_DATA_HOME/vaultline/store or ~/.local/share/vaultline/store.

  vaultline daemon bind <addr>
  vaultline daemon list-binds
  vaultline daemon unbind <addr>
  vaultline daemon allow <addr> <cidr-or-ip>
  vaultline daemon list-allows <addr>
  vaultline daemon unallow <addr> <cidr-or-ip>
      Manage extra remote listeners. Loopback remains implicitly available.

  vaultline completion bash|zsh
      Print shell completion for the requested shell.

  vaultline [--addr HOST:PORT] <command> [flags]
      health                     Check daemon status
      unseal                     Prompt for passphrase and unlock the local store
      seal                       Reseal the local store
      daemon-stop                Ask the daemon to shut down
      completion                 Print shell completion helpers
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

  vaultline store init <name> <path>
      Create a new named store, generate an unseal key, store it in config,
      and leave the store immediately unsealed.

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
  vaultline secret list [store:]

Notes:
  - Store prefixes use the form store:key and default to local when omitted.
  - Secret names must use lowercase letters plus . or -.
  - --twice only applies to interactive --stdin input and aborts on mismatch.
`
}

func storeSubcommandHelp(name string) string {
	switch name {
	case "add":
		return "Usage:\n  vaultline store add <name> <path>\n\nRegister an existing store path in the local registry."
	case "init":
		return "Usage:\n  vaultline store init <name> <path>\n\nCreate a new store at <path>, generate an unseal key, remember it in config, and leave the store unsealed."
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
	case "list":
		return "Usage:\n  vaultline secret list [store:]\n\nList secrets from one store (default: local)."
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
			fmt.Fprintln(out, "Usage:\n  vaultline unseal\n\nUnseal the local store. Uses a remembered passphrase first, then prompts if needed.")
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
			fmt.Fprintln(out, "Usage:\n  vaultline seal [--keep-keys]\n\nSeal the local store. By default remembered passphrases are removed from config.")
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
		return filterCompletions([]string{"health", "unseal", "seal", "daemon-stop", "daemon", "store", "secret", "completion", "--addr", "--output", "--help"}, current)
	}
	switch words[0] {
	case "completion":
		return filterCompletions([]string{"bash", "zsh"}, current)
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

func completeStoreWords(words []string, current string) []string {
	storeNames := listConfiguredStores(true)
	if len(words) == 0 {
		return filterCompletions([]string{"add", "init", "list", "show", "unseal", "seal", "delete", "remove", "rm", "--help"}, current)
	}
	sub := words[0]
	if len(words) == 1 {
		switch sub {
		case "show", "unseal", "seal":
			return filterCompletions(storeNames, current)
		case "delete", "remove", "rm":
			return filterCompletions(filterOut(storeNames, "local"), current)
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
		return filterCompletions([]string{"set", "get", "delete", "list", "--help"}, current)
	}
	sub := words[0]
	flagsForSet := []string{"--name", "--value", "--file", "--stdin", "--twice", "--help"}
	flagsForGet := []string{"--name", "--out", "--help"}
	flagsForDelete := []string{"--name", "--help"}
	if sub == "set" || sub == "get" || sub == "delete" {
		if strings.Contains(current, ":") || (len(words) > 1 && words[len(words)-1] == "--name") {
			return completeQualifiedSecret(current)
		}
	}
	if len(words) == 1 {
		switch sub {
		case "set", "get", "delete":
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
	return filterCompletions(buildQualifiedKeyCompletions(storeName, keys), current)
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

func buildQualifiedKeyCompletions(storeName string, keys []string) []string {
	seen := map[string]bool{}
	results := make([]string, 0, len(keys))
	for _, key := range keys {
		parts := strings.Split(key, ".")
		for i := range parts {
			candidate := strings.Join(parts[:i+1], ".")
			if i < len(parts)-1 {
				candidate += "."
			}
			qualified := storeName + ":" + candidate
			if !seen[qualified] {
				seen[qualified] = true
				results = append(results, qualified)
			}
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
			return []string{"local"}
		}
		return nil
	}
	names := make([]string, 0, len(cfg.Stores))
	for name := range cfg.Stores {
		if !includeLocal && name == "local" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func listStorePrefixes() []string {
	names := listConfiguredStores(true)
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
		return filepath.Join(xdgData, "vaultline", "store")
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "share", "vaultline", "store")
}

func resolveStoreConfigPath() string {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "vaultline", "stores.json")
	}
	return filepath.Join(os.Getenv("HOME"), ".config", "vaultline", "stores.json")
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
		if len(args) != 3 {
			return fmt.Errorf("usage: vaultline store %s <name> <path>", args[0])
		}
		payload, err := json.Marshal(api.StoreCreateRequest{Name: args[1], Path: args[2], Initialize: args[0] == "init"})
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
		fs := flag.NewFlagSet("store "+args[0], flag.ContinueOnError)
		keepKeys := fs.Bool("keep-keys", false, "keep remembered passphrases in store config")
		fs.SetOutput(io.Discard)
		if err := fs.Parse(args[1:]); err != nil {
			if err == flag.ErrHelp {
				fmt.Fprintln(out, storeSubcommandHelp(args[0]))
				return nil
			}
			return err
		}
		if fs.NArg() != 1 {
			return fmt.Errorf("usage: vaultline store %s <name>", args[0])
		}
		storeRaw := fs.Arg(0)
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
		return "local", parts[0], nil
	}
	if parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("qualified keys must look like store:key")
	}
	return parts[0], parts[1], nil
}

func parseStoreSelector(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "local", nil
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
	name := fs.String("name", "", "secret identifier (lowercase letters, dot, dash)")
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

func secretList(baseURL string, args []string, outputFmt string, out io.Writer) error {
	storeName := "local"
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
