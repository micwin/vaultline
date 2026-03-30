package daemoncfg

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type Bind struct {
	Addr   string   `json:"addr"`
	Allows []string `json:"allows"`
}

type Config struct {
	Binds map[string]*Bind `json:"binds"`
}

type Manager struct {
	path string
	mu   sync.Mutex
	cfg  Config
}

func DefaultPath() string {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "vaultline", "daemon.json")
	}
	return filepath.Join(os.Getenv("HOME"), ".config", "vaultline", "daemon.json")
}

func New(path string) (*Manager, error) {
	m := &Manager{path: path, cfg: Config{Binds: map[string]*Bind{}}}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &m.cfg); err != nil {
			return nil, fmt.Errorf("parse daemon config: %w", err)
		}
		if m.cfg.Binds == nil {
			m.cfg.Binds = map[string]*Bind{}
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return m, nil
}

func (m *Manager) Save() error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m.cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, data, 0o600)
}

func validateBindAddr(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid bind address %q", addr)
	}
	if host == "" {
		return fmt.Errorf("bind address host required")
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return fmt.Errorf("loopback is always available and cannot be configured as extra bind")
	}
	if host == "localhost" {
		return fmt.Errorf("loopback is always available and cannot be configured as extra bind")
	}
	return nil
}

func normalizeAllow(rule string) (string, error) {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return "", fmt.Errorf("allow rule required")
	}
	if strings.Contains(rule, "/") {
		if _, _, err := net.ParseCIDR(rule); err != nil {
			return "", fmt.Errorf("invalid allow rule %q", rule)
		}
		return rule, nil
	}
	ip := net.ParseIP(rule)
	if ip == nil {
		return "", fmt.Errorf("invalid allow rule %q", rule)
	}
	if ip.To4() != nil {
		return rule + "/32", nil
	}
	return rule + "/128", nil
}

func (m *Manager) AddBind(addr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := validateBindAddr(addr); err != nil {
		return err
	}
	if _, ok := m.cfg.Binds[addr]; ok {
		return fmt.Errorf("bind %s already exists", addr)
	}
	m.cfg.Binds[addr] = &Bind{Addr: addr, Allows: []string{}}
	return m.Save()
}

func (m *Manager) RemoveBind(addr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.cfg.Binds[addr]; !ok {
		return fmt.Errorf("bind %s not found", addr)
	}
	delete(m.cfg.Binds, addr)
	return m.Save()
}

func (m *Manager) AddAllow(addr, rule string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	bind, ok := m.cfg.Binds[addr]
	if !ok {
		return fmt.Errorf("bind %s not found", addr)
	}
	normalized, err := normalizeAllow(rule)
	if err != nil {
		return err
	}
	for _, existing := range bind.Allows {
		if existing == normalized {
			return nil
		}
	}
	bind.Allows = append(bind.Allows, normalized)
	sort.Strings(bind.Allows)
	return m.Save()
}

func (m *Manager) RemoveAllow(addr, rule string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	bind, ok := m.cfg.Binds[addr]
	if !ok {
		return fmt.Errorf("bind %s not found", addr)
	}
	normalized, err := normalizeAllow(rule)
	if err != nil {
		return err
	}
	filtered := bind.Allows[:0]
	found := false
	for _, existing := range bind.Allows {
		if existing == normalized {
			found = true
			continue
		}
		filtered = append(filtered, existing)
	}
	if !found {
		return fmt.Errorf("allow %s not found on %s", normalized, addr)
	}
	bind.Allows = filtered
	return m.Save()
}

func (m *Manager) ListBinds() []Bind {
	m.mu.Lock()
	defer m.mu.Unlock()
	addrs := make([]string, 0, len(m.cfg.Binds))
	for addr := range m.cfg.Binds {
		addrs = append(addrs, addr)
	}
	sort.Strings(addrs)
	result := make([]Bind, 0, len(addrs))
	for _, addr := range addrs {
		bind := m.cfg.Binds[addr]
		copyAllows := append([]string(nil), bind.Allows...)
		result = append(result, Bind{Addr: bind.Addr, Allows: copyAllows})
	}
	return result
}

func (m *Manager) ListAllows(addr string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	bind, ok := m.cfg.Binds[addr]
	if !ok {
		return nil, fmt.Errorf("bind %s not found", addr)
	}
	return append([]string(nil), bind.Allows...), nil
}
