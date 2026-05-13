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
	masterSaltFile         = ".master_salt"
	verifierFile           = ".verifier"
	verifierPlaintext      = "vaultline-store-verifier-v1"
	verifierAdditionalData = "vaultline-store-verifier"
	keyLength              = 32
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

type verifierEnvelope struct {
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
	if err := s.verifyOrCreateVerifier(key); err != nil {
		for i := range key {
			key[i] = 0
		}
		return err
	}
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

func (s *Store) verifyOrCreateVerifier(masterKey []byte) error {
	path := filepath.Join(s.root, verifierFile)
	if data, err := os.ReadFile(path); err == nil {
		if err := verifyMasterKey(data, masterKey); err != nil {
			return err
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read verifier: %w", err)
	}

	if err := s.validateExistingSecretsWithKey(masterKey); err != nil {
		return err
	}
	payload, err := newVerifierPayload(masterKey)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("write verifier: %w", err)
	}
	return nil
}

func verifyMasterKey(data []byte, masterKey []byte) error {
	var env verifierEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return fmt.Errorf("decode verifier: %w", err)
	}
	nonce, err := base64.StdEncoding.DecodeString(env.Nonce)
	if err != nil {
		return fmt.Errorf("decode verifier nonce: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(env.Ciphertext)
	if err != nil {
		return fmt.Errorf("decode verifier ciphertext: %w", err)
	}
	aead, err := chacha20poly1305.NewX(masterKey)
	if err != nil {
		return fmt.Errorf("new verifier aead: %w", err)
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, []byte(verifierAdditionalData))
	if err != nil {
		return fmt.Errorf("vaultline: invalid passphrase")
	}
	if string(plaintext) != verifierPlaintext {
		return fmt.Errorf("vaultline: invalid verifier")
	}
	return nil
}

func newVerifierPayload(masterKey []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(masterKey)
	if err != nil {
		return nil, fmt.Errorf("new verifier aead: %w", err)
	}
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("verifier nonce: %w", err)
	}
	ciphertext := aead.Seal(nil, nonce, []byte(verifierPlaintext), []byte(verifierAdditionalData))
	env := verifierEnvelope{
		Version:    "v1",
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
		CreatedAt:  time.Now().UTC(),
	}
	payload, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal verifier: %w", err)
	}
	return payload, nil
}

func (s *Store) validateExistingSecretsWithKey(masterKey []byte) error {
	secretsDir := filepath.Join(s.root, "secrets")
	entries, err := os.ReadDir(secretsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read secrets for verifier migration: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if len(name) <= len(".vlx") || name[len(name)-len(".vlx"):] != ".vlx" {
			continue
		}
		secretName := name[:len(name)-len(".vlx")]
		if err := s.decryptSecretFileWithMasterKey(filepath.Join(secretsDir, name), secretName, masterKey); err != nil {
			return fmt.Errorf("vaultline: invalid passphrase")
		}
		return nil
	}
	return nil
}

func (s *Store) decryptSecretFileWithMasterKey(pathName, secretName string, masterKey []byte) error {
	data, err := os.ReadFile(pathName)
	if err != nil {
		return fmt.Errorf("read secret for verifier migration: %w", err)
	}
	var env secretEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return fmt.Errorf("decode secret for verifier migration: %w", err)
	}
	nonce, err := base64.StdEncoding.DecodeString(env.Nonce)
	if err != nil {
		return fmt.Errorf("decode migration nonce: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(env.Ciphertext)
	if err != nil {
		return fmt.Errorf("decode migration ciphertext: %w", err)
	}
	key := deriveSecretKey(masterKey, secretName)
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return fmt.Errorf("new migration aead: %w", err)
	}
	if _, err := aead.Open(nil, nonce, ciphertext, nil); err != nil {
		return err
	}
	return nil
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
	return deriveSecretKey(s.masterKey, keyName), nil
}

func deriveSecretKey(masterKey []byte, keyName string) []byte {
	data := []byte(keyName)
	mac := hmac.New(sha256.New, masterKey)
	mac.Write(data)
	sum := mac.Sum(nil)
	key := make([]byte, keyLength)
	copy(key, sum)
	return key
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
