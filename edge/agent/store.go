package agent

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// Camera is one camera configured on this device. Credentials never leave
// the device: the store is encrypted with the local key, and the cloud only
// knows the camera exists.
type Camera struct {
	Name      string   `json:"name"`
	Host      string   `json:"host"`
	Vendor    string   `json:"vendor"`
	Model     string   `json:"model"`
	ProfileID string   `json:"profile_id"`
	Channel   int      `json:"channel,omitempty"`
	Username  string   `json:"username,omitempty"`
	Password  string   `json:"password,omitempty"`
	RTSPMain  string   `json:"rtsp_main,omitempty"` // manual override
	RTSPSub   string   `json:"rtsp_sub,omitempty"`
	Track     []string `json:"track,omitempty"`
	Audio     bool     `json:"audio,omitempty"`
	PTZ       bool     `json:"ptz,omitempty"`
}

// Store keeps the camera list in an AES-GCM encrypted file.
type Store struct {
	mu   sync.Mutex
	path string
	aead cipher.AEAD
	cams map[string]Camera
}

// OpenStore opens (or creates) the encrypted camera store. keyFile holds a
// 32-byte key; on production hardware it is unsealed from the TPM at boot.
func OpenStore(path, keyFile string) (*Store, error) {
	key, err := loadOrCreateKey(keyFile)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	s := &Store{path: path, aead: aead, cams: map[string]Camera{}}
	return s, s.load()
}

func loadOrCreateKey(path string) ([]byte, error) {
	key, err := os.ReadFile(path)
	if err == nil {
		if len(key) != 32 {
			return nil, fmt.Errorf("%s: key must be 32 bytes", path)
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return key, os.WriteFile(path, key, 0o600)
}

func (s *Store) load() error {
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	ns := s.aead.NonceSize()
	if len(raw) < ns {
		return fmt.Errorf("camera store is corrupt")
	}
	plain, err := s.aead.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return fmt.Errorf("decrypt camera store: %w", err)
	}
	return json.Unmarshal(plain, &s.cams)
}

func (s *Store) save() error {
	plain, err := json.Marshal(s.cams)
	if err != nil {
		return err
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	out := s.aead.Seal(nonce, nonce, plain, nil)
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Put adds or replaces a camera.
func (s *Store) Put(c Camera) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cams[c.Name] = c
	return s.save()
}

// Delete removes a camera.
func (s *Store) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cams, name)
	return s.save()
}

// List returns cameras sorted by name.
func (s *Store) List() []Camera {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Camera, 0, len(s.cams))
	for _, c := range s.cams {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
