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
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/micwin/vaultline/pkg/api"
	"github.com/micwin/vaultline/pkg/version"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

func usageText() string {
	return fmt.Sprintf("vaultline %s\n\n", version.Version) + `Usage:
  vaultline daemon [--addr HOST:PORT] [--store-dir DIR]
      Starts the HTTPS API server. When --store-dir is omitted the daemon
      uses $XDG_DATA_HOME/vaultline/store or ~/.local/share/vaultline/store.

  vaultline [--addr HOST:PORT] <command> [flags]
      health                     Check daemon status
      unseal                     Prompt for passphrase and unlock the daemon
      seal                       Reseal the daemon
      daemon-stop                Ask the daemon to shut down
      secret put|get|delete      Manage secrets (keys use lowercase letters plus . and -)
      secrets list|set|get|delete Same operations via the secrets namespace

Examples:
  vaultline daemon --store-dir ./store
  vaultline --addr 127.0.0.1:8428 health
  vaultline --addr 127.0.0.1:8428 secret put --name app.api-key --stdin
`
}

// Run executes the CLI subcommands.
func Run(args []string, out io.Writer) error {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Fprint(out, usageText())
			return nil
		}
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
	case "health":
		return runHealth(baseURL, *output, out)
	case "unseal":
		return runUnseal(baseURL, out)
	case "seal":
		return runSeal(baseURL, out)
	case "daemon-stop":
		return runDaemonStop(baseURL, out)
	case "secret":
		return runSecret(baseURL, remaining[1:], *output, out)
	case "secrets":
		return runSecrets(baseURL, remaining[1:], *output, out)
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
	fmt.Fprintf(out, "sealed=%v status=%v\n", payload["sealed"], payload["status"])
	return nil
}

func runUnseal(baseURL string, out io.Writer) error {
	passphrase, err := readPassphrase()
	if err != nil {
		return err
	}
	req := api.UnsealRequest{Passphrase: passphrase}
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	resp, err := httpClient.Post(baseURL+"/api/v1/unseal", "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unseal failed: %s", body)
	}
	fmt.Fprintln(out, "vaultline unsealed")
	return nil
}

func runSeal(baseURL string, out io.Writer) error {
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/seal", nil)
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

func runSecret(baseURL string, args []string, outputFmt string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("secret command requires subcommand")
	}
	switch args[0] {
	case "put":
		return secretPut(baseURL, args[1:], out)
	case "get":
		return secretGet(baseURL, args[1:], outputFmt, out)
	case "delete":
		return secretDelete(baseURL, args[1:], out)
	default:
		return fmt.Errorf("unknown secret subcommand %q", args[0])
	}
}

func runSecrets(baseURL string, args []string, outputFmt string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("secrets command requires subcommand")
	}
	switch args[0] {
	case "list":
		return secretsList(baseURL, outputFmt, out)
	case "set":
		return secretPut(baseURL, args[1:], out)
	case "get":
		return secretGet(baseURL, args[1:], outputFmt, out)
	case "delete":
		return secretDelete(baseURL, args[1:], out)
	default:
		return fmt.Errorf("unknown secrets subcommand %q", args[0])
	}
}

func secretPut(baseURL string, args []string, out io.Writer) error {
	keyArg, flagArgs := splitKeyArg(args, map[string]bool{"--value": true, "--file": true, "--name": true})
	fs := flag.NewFlagSet("secret put", flag.ContinueOnError)
	name := fs.String("name", "", "secret identifier (lowercase letters, dot, dash)")
	value := fs.String("value", "", "literal secret value")
	filePath := fs.String("file", "", "path to file")
	useStdin := fs.Bool("stdin", false, "read secret from stdin (mask prompt when running interactively)")
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
	data, err := readSecretInput(*value, *filePath, *useStdin)
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
	url := fmt.Sprintf("%s/api/v1/secrets/%s", baseURL, key)
	httpReq, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(payload))
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
	fmt.Fprintf(out, "secret stored (version=%s)\n", version.Version)
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
	url := fmt.Sprintf("%s/api/v1/secrets/%s", baseURL, key)
	resp, err := httpClient.Get(url)
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
	url := fmt.Sprintf("%s/api/v1/secrets/%s", baseURL, key)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
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
	fmt.Fprintln(out, "secret removed")
	return nil
}

func secretsList(baseURL, outputFmt string, out io.Writer) error {
	resp, err := httpClient.Get(baseURL + "/api/v1/secrets")
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
		Keys []string `json:"keys"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return err
	}
	if len(payload.Keys) == 0 {
		fmt.Fprintln(out, "(no secrets)")
		return nil
	}
	for _, key := range payload.Keys {
		fmt.Fprintln(out, key)
	}
	return nil
}

func readSecretInput(literal, filePath string, stdin bool) ([]byte, error) {
	switch {
	case stdin:
		if term.IsTerminal(int(syscall.Stdin)) {
			fmt.Print("Secret value: ")
			bytesValue, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			if err != nil {
				return nil, err
			}
			return []byte(strings.TrimSpace(string(bytesValue))), nil
		}
		return io.ReadAll(os.Stdin)
	case filePath != "":
		return os.ReadFile(filePath)
	default:
		return []byte(literal), nil
	}
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
