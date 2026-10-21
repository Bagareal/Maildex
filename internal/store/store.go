// Package store keeps all Maildex state in two small JSON files:
//
//	data.json  emails, services and settings (what the user sees and exports)
//	auth.json  password hash, passkeys and sessions (never exported)
//
// The whole dataset is held in memory and rewritten atomically on every change.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Email struct {
	ID       string    `json:"id"`
	Address  string    `json:"address"`
	Label    string    `json:"label"`
	Provider string    `json:"provider"`
	Recovery string    `json:"recovery"`
	Notes    string    `json:"notes"`
	AtRisk   bool      `json:"atRisk"`
	Created  time.Time `json:"created"`
}

type Service struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	EmailID     string    `json:"emailId"`
	Username    string    `json:"username"`
	LoginMethod string    `json:"loginMethod"`
	Category    string    `json:"category"`
	Status      string    `json:"status"`
	TwoFactor   []string  `json:"twoFactor"` // enabled second factors, empty when none
	URL         string    `json:"url"`
	Renewal     string    `json:"renewal"` // YYYY-MM-DD or empty
	Notes       string    `json:"notes"`
	Created     time.Time `json:"created"`
}

type Settings struct {
	// LockMinutes is the idle time after which the app asks to be unlocked
	// again. Only enforced once a passkey is enrolled. 0 disables the idle lock.
	LockMinutes int `json:"lockMinutes"`
}

type Data struct {
	Emails   []Email   `json:"emails"`
	Services []Service `json:"services"`
	Settings Settings  `json:"settings"`
}

type Credential struct {
	ID        string    `json:"id"` // base64url credential ID
	Name      string    `json:"name"`
	PublicKey []byte    `json:"publicKey"` // COSE key
	SignCount uint32    `json:"signCount"`
	Created   time.Time `json:"created"`
}

type SessionRec struct {
	ID      string    `json:"id"` // sha256 hex of the cookie token
	Created time.Time `json:"created"`
	Expires time.Time `json:"expires"`
}

type Auth struct {
	PasswordHash string       `json:"passwordHash"`
	Credentials  []Credential `json:"credentials"`
	Sessions     []SessionRec `json:"sessions"`
}

type Store struct {
	mu   sync.RWMutex
	dir  string
	data Data
	auth Auth
}

const DefaultLockMinutes = 5

// Open loads (or initialises) the store in dir and keeps a data.json.bak copy
// of the previous state.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	s := &Store{dir: dir}
	s.data.Settings.LockMinutes = DefaultLockMinutes
	if err := readJSON(filepath.Join(dir, "data.json"), &s.data); err != nil {
		return nil, err
	}
	if err := readJSON(filepath.Join(dir, "auth.json"), &s.auth); err != nil {
		return nil, err
	}
	if b, err := os.ReadFile(filepath.Join(dir, "data.json")); err == nil {
		_ = os.WriteFile(filepath.Join(dir, "data.json.bak"), b, 0o600)
	}
	return s, nil
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// View runs fn with read access to the state. fn must not modify or retain it.
func (s *Store) View(fn func(d *Data, a *Auth)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	fn(&s.data, &s.auth)
}

// Update runs fn with write access and persists the result when fn succeeds.
// fn should validate before mutating: a returned error skips the save but does
// not roll back changes already made.
func (s *Store) Update(fn func(d *Data, a *Auth) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := fn(&s.data, &s.auth); err != nil {
		return err
	}
	if s.data.Emails == nil {
		s.data.Emails = []Email{}
	}
	if s.data.Services == nil {
		s.data.Services = []Service{}
	}
	if err := writeJSON(filepath.Join(s.dir, "data.json"), &s.data); err != nil {
		return err
	}
	return writeJSON(filepath.Join(s.dir, "auth.json"), &s.auth)
}

// ResetAuth wipes the password, passkeys and sessions in dir (used by the CLI).
func ResetAuth(dir string) error {
	return writeJSON(filepath.Join(dir, "auth.json"), &Auth{})
}

func NewID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
