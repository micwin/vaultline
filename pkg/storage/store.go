package storage

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

const (
	masterSaltFile = ".master_salt"
	keyLength      = 32
)

var (
	// ErrSealed indicates that the store has not been unlocked yet.
	ErrSealed = errors.New("vaultline: store sealed")
	// ErrSecretNotFound is returned when the requested secret file is missing.
	ErrSecretNotFound = errors.New("vaultline: secret not found")

	keyPattern = regexp.MustCompile(`^[\p{Ll}\p{Nd}@.-]+$`)
)

// Store persists encrypted secrets on disk. Each secret is stored in its own file
// to keep Git diffs readable and minimise merge conflicts.
type Store struct {
	root       string
	masterSalt []byte

	mu        sync.RWMutex
	masterKey []byte
	sealed    bool
}

// Secret holds decrypted secret data and the associated version string.
type Secret struct {
	Data    []byte
	Version string
}

type secretEnvelope struct {
	Version    string    `json:"version"`
	Nonce      string    `json:"nonce"`
	Ciphertext string    `json:"ciphertext"`
	CreatedAt  time.Time `json:"created_at"`
}

// New initialises a Store rooted at the provided directory. The directory is
// created if missing, and an Argon2 salt is generated on first run.
func New(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create store root: %w", err)
	}
	salt, err := loadOrCreateSalt(filepath.Join(root, masterSaltFile))
	if err != nil {
		return nil, err
	}
	return &Store{
		root:       root,
		masterSalt: salt,
		sealed:     true,
	}, nil
}

// Sealed reports whether the store is currently locked.
func (s *Store) Sealed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sealed
}

// Seal wipes derived keys from memory.
func (s *Store) Seal() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.masterKey != nil {
		for i := range s.masterKey {
			s.masterKey[i] = 0
		}
	}
	s.masterKey = nil
	s.sealed = true
}

// Unseal derives the master key from the passphrase and unlocks the store.
func (s *Store) Unseal(passphrase string) error {
	if passphrase == "" {
		return errors.New("vaultline: passphrase required")
	}
	key := argon2.IDKey([]byte(passphrase), s.masterSalt, 3, 64*1024, 4, keyLength)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.masterKey = key
	s.sealed = false
	return nil
}

// Put writes (or overwrites) a secret.
func (s *Store) Put(name string, data []byte) (string, error) {
	key, err := s.deriveKey(name)
	if err != nil {
		return "", err
	}
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return "", fmt.Errorf("new aead: %w", err)
	}
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}
	ciphertext := aead.Seal(nil, nonce, data, nil)
	version := deriveVersion(nonce, ciphertext)
	env := secretEnvelope{
		Version:    version,
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
		CreatedAt:  time.Now().UTC(),
	}
	payload, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal envelope: %w", err)
	}
	path, err := s.secretPath(name)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("create secret dir: %w", err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return "", fmt.Errorf("write secret: %w", err)
	}
	return version, nil
}

// Get decrypts a stored secret.
func (s *Store) Get(name string) (*Secret, error) {
	key, err := s.deriveKey(name)
	if err != nil {
		return nil, err
	}
	path, err := s.secretPath(name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrSecretNotFound
		}
		return nil, fmt.Errorf("read secret: %w", err)
	}
	var env secretEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("decode envelope: %w", err)
	}
	nonce, err := base64.StdEncoding.DecodeString(env.Nonce)
	if err != nil {
		return nil, fmt.Errorf("decode nonce: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(env.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("new aead: %w", err)
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt secret: %w", err)
	}
	return &Secret{Data: plaintext, Version: env.Version}, nil
}

// Delete removes the secret file.
func (s *Store) Delete(name string) error {
	if s.Sealed() {
		return ErrSealed
	}
	path, err := s.secretPath(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrSecretNotFound
		}
		return fmt.Errorf("delete secret: %w", err)
	}
	return nil
}

func (s *Store) secretPath(name string) (string, error) {
	key, err := normalizeKey(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.root, "secrets", key+".vlx"), nil
}

func (s *Store) deriveKey(name string) ([]byte, error) {
	keyName, err := normalizeKey(name)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.sealed || s.masterKey == nil {
		return nil, ErrSealed
	}
	data := []byte(keyName)
	mac := hmac.New(sha256.New, s.masterKey)
	mac.Write(data)
	sum := mac.Sum(nil)
	key := make([]byte, keyLength)
	copy(key, sum)
	return key, nil
}

func normalizeKey(value string) (string, error) {
	if value == "" {
		return "", errors.New("vaultline: empty identifier")
	}
	if !keyPattern.MatchString(value) {
		return "", fmt.Errorf("vaultline: invalid identifier %q (only lowercase letters, digits, @, dot, and dash allowed)", value)
	}
	return value, nil
}

func loadOrCreateSalt(path string) ([]byte, error) {
	if data, err := os.ReadFile(path); err == nil {
		salt, err := base64.StdEncoding.DecodeString(string(data))
		if err != nil {
			return nil, fmt.Errorf("decode master salt: %w", err)
		}
		return salt, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read master salt: %w", err)
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("salt entropy: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(salt)
	if err := os.WriteFile(path, []byte(encoded), 0o600); err != nil {
		return nil, fmt.Errorf("write master salt: %w", err)
	}
	return salt, nil
}

func deriveVersion(nonce, ciphertext []byte) string {
	sum := sha256.Sum256(append(nonce, ciphertext...))
	return fmt.Sprintf("%x", sum[:10])
}
