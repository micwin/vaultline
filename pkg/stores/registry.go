package stores

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/micwin/vaultline/pkg/storage"
)

var storeNamePattern = regexp.MustCompile(`^[a-z0-9-]+$`)

var ErrStoreUnavailable = errors.New("vaultline: store unavailable")

type Entry struct {
	Path       string `json:"path"`
	Passphrase string `json:"passphrase,omitempty"`
}

type Config struct {
	DefaultStore string           `json:"default_store"`
	Stores       map[string]Entry `json:"stores"`
}

type Info struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Default   bool   `json:"default"`
	Available bool   `json:"available"`
	Sealed    bool   `json:"sealed"`
	HasKey    bool   `json:"has_key"`
	Error     string `json:"error,omitempty"`
}

func DefaultConfigPath() string {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "vaultline", "stores.json")
	}
	return filepath.Join(os.Getenv("HOME"), ".config", "vaultline", "stores.json")
}

func LoadConfig(configPath, localStorePath string) (*Config, error) {
	cfg := &Config{
		DefaultStore: "local",
		Stores:       map[string]Entry{},
	}
	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse store config: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if cfg.Stores == nil {
		cfg.Stores = map[string]Entry{}
	}
	localEntry := cfg.Stores["local"]
	localEntry.Path = localStorePath
	cfg.Stores["local"] = localEntry
	if cfg.DefaultStore == "" {
		cfg.DefaultStore = "local"
	}
	return cfg, nil
}

func SaveConfig(configPath string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0o600)
}

func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("store name required")
	}
	if !storeNamePattern.MatchString(name) {
		return fmt.Errorf("invalid store name %q", name)
	}
	return nil
}

type Manager struct {
	configPath string
	cfg        *Config
	loaded     map[string]*storage.Store
}

func NewManager(configPath, localStorePath string) (*Manager, error) {
	cfg, err := LoadConfig(configPath, localStorePath)
	if err != nil {
		return nil, err
	}
	manager := &Manager{
		configPath: configPath,
		cfg:        cfg,
		loaded:     map[string]*storage.Store{},
	}
	if _, err := manager.ensureStore("local", true); err != nil {
		return nil, err
	}
	return manager, nil
}

func (m *Manager) DefaultStore() string {
	return m.cfg.DefaultStore
}

func (m *Manager) ensureStore(name string, createIfMissing bool) (*storage.Store, error) {
	if store, ok := m.loaded[name]; ok {
		return store, nil
	}
	entry, ok := m.cfg.Stores[name]
	if !ok {
		return nil, fmt.Errorf("unknown store %q", name)
	}
	if !createIfMissing {
		if _, err := os.Stat(filepath.Join(entry.Path, ".master_salt")); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("%w: missing store metadata at %s", ErrStoreUnavailable, entry.Path)
			}
			return nil, fmt.Errorf("%w: %v", ErrStoreUnavailable, err)
		}
	}
	store, err := storage.New(entry.Path)
	if err != nil {
		if name != "local" {
			return nil, fmt.Errorf("%w: %v", ErrStoreUnavailable, err)
		}
		return nil, err
	}
	m.loaded[name] = store
	return store, nil
}

func (m *Manager) Store(name string) (*storage.Store, error) {
	return m.ensureStore(name, name == "local")
}

func (m *Manager) Add(name, path string, initialize bool) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	if name == "local" {
		return fmt.Errorf("store name %q is reserved", name)
	}
	if path == "" {
		return fmt.Errorf("store path required")
	}
	if !initialize {
		if _, err := os.Stat(filepath.Join(path, ".master_salt")); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("store %q does not exist at %s", name, path)
			}
			return err
		}
	}
	if _, err := m.ensureStoreWithPath(name, path, initialize); err != nil {
		return err
	}
	m.cfg.Stores[name] = Entry{Path: path}
	return SaveConfig(m.configPath, m.cfg)
}

func (m *Manager) Remove(name string) error {
	if name == "local" {
		return fmt.Errorf("store %q is reserved", name)
	}
	if _, ok := m.cfg.Stores[name]; !ok {
		return fmt.Errorf("unknown store %q", name)
	}
	delete(m.cfg.Stores, name)
	delete(m.loaded, name)
	return SaveConfig(m.configPath, m.cfg)
}

func (m *Manager) ensureStoreWithPath(name, path string, initialize bool) (*storage.Store, error) {
	if !initialize {
		if _, err := os.Stat(filepath.Join(path, ".master_salt")); err != nil {
			return nil, err
		}
	}
	store, err := storage.New(path)
	if err != nil {
		return nil, err
	}
	m.loaded[name] = store
	return store, nil
}

func (m *Manager) Seal(name string, keepKeys bool) error {
	store, err := m.Store(name)
	if err != nil {
		return err
	}
	store.Seal()
	if !keepKeys {
		entry := m.cfg.Stores[name]
		entry.Passphrase = ""
		m.cfg.Stores[name] = entry
		return SaveConfig(m.configPath, m.cfg)
	}
	return nil
}

func (m *Manager) Unseal(name, passphrase string) error {
	entry, ok := m.cfg.Stores[name]
	if !ok {
		return fmt.Errorf("unknown store %q", name)
	}
	if passphrase == "" {
		passphrase = entry.Passphrase
	}
	if passphrase == "" {
		return fmt.Errorf("vaultline: passphrase required")
	}
	store, err := m.Store(name)
	if err != nil {
		return err
	}
	if err := store.Unseal(passphrase); err != nil {
		return err
	}
	entry.Passphrase = passphrase
	m.cfg.Stores[name] = entry
	return SaveConfig(m.configPath, m.cfg)
}

func (m *Manager) Info(name string) (Info, error) {
	entry, ok := m.cfg.Stores[name]
	if !ok {
		return Info{}, fmt.Errorf("unknown store %q", name)
	}
	info := Info{Name: name, Path: entry.Path, Default: name == m.cfg.DefaultStore, HasKey: entry.Passphrase != ""}
	store, err := m.Store(name)
	if err != nil {
		if errors.Is(err, ErrStoreUnavailable) {
			info.Error = err.Error()
			return info, nil
		}
		return Info{}, err
	}
	info.Available = true
	info.Sealed = store.Sealed()
	return info, nil
}

func (m *Manager) List() ([]Info, error) {
	names := make([]string, 0, len(m.cfg.Stores))
	for name := range m.cfg.Stores {
		names = append(names, name)
	}
	sort.Strings(names)
	infos := make([]Info, 0, len(names))
	for _, name := range names {
		info, err := m.Info(name)
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}
