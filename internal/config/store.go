package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var ErrProviderExists = errors.New("provider already exists")

type Store struct {
	Path      string
	Providers []Provider
}

type State struct {
	CurrentProvider string `json:"current_provider"`
	Mode            string `json:"mode,omitempty"`
	ProxyBaseURL    string `json:"proxy_base_url,omitempty"`
}

const (
	ModeDirect = "direct"
	ModeProxy  = "proxy"
)

func NewStore(path string) *Store {
	return &Store{Path: path}
}

func LoadStore(path string) (*Store, error) {
	store := NewStore(path)
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read provider store: %w", err)
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return store, nil
	}
	if err := json.Unmarshal(b, &store.Providers); err != nil {
		return nil, fmt.Errorf("parse provider store %s: %w", path, err)
	}
	return store, nil
}

func (s *Store) Save() error {
	if s.Path == "" {
		return errors.New("provider store path is empty")
	}
	b, err := json.MarshalIndent(s.Providers, "", "  ")
	if err != nil {
		return fmt.Errorf("encode provider store: %w", err)
	}
	b = append(b, '\n')
	if err := writeFileAtomic(s.Path, b, 0o600); err != nil {
		return err
	}
	return nil
}

func (s *Store) Add(provider Provider) error {
	provider.Normalize()
	if err := provider.Validate(); err != nil {
		return err
	}
	if i := s.index(provider.ID); i >= 0 {
		return fmt.Errorf("%w: %s", ErrProviderExists, provider.ID)
	}
	s.Providers = append(s.Providers, provider)
	return s.Save()
}

func (s *Store) Update(id string, provider Provider) error {
	provider.Normalize()
	if err := provider.Validate(); err != nil {
		return err
	}
	i := s.index(id)
	if i < 0 {
		return fmt.Errorf("provider %q does not exist", id)
	}
	provider.ID = id
	s.Providers[i] = provider
	return s.Save()
}

func (s *Store) Remove(id string) (Provider, bool) {
	i := s.index(id)
	if i < 0 {
		return Provider{}, false
	}
	removed := s.Providers[i]
	s.Providers = append(s.Providers[:i], s.Providers[i+1:]...)
	return removed, true
}

func (s *Store) Find(id string) (Provider, bool) {
	i := s.index(id)
	if i < 0 {
		return Provider{}, false
	}
	return s.Providers[i], true
}

func (s *Store) index(id string) int {
	for i := range s.Providers {
		if s.Providers[i].ID == id {
			return i
		}
	}
	return -1
}

func LoadState() (State, error) {
	path, err := StatePath()
	if err != nil {
		return State{}, err
	}
	return LoadStateFile(path)
}

func LoadStateFile(path string) (State, error) {
	state := State{}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, fmt.Errorf("read state: %w", err)
	}
	if err := json.Unmarshal(b, &state); err != nil {
		return state, fmt.Errorf("parse state: %w", err)
	}
	return state, nil
}

func SaveState(currentProvider string) error {
	return SaveStateWithMode(currentProvider, ModeDirect, "")
}

func SaveStateWithMode(currentProvider, mode, proxyBaseURL string) error {
	path, err := StatePath()
	if err != nil {
		return err
	}
	return SaveStateFile(path, State{
		CurrentProvider: currentProvider,
		Mode:            mode,
		ProxyBaseURL:    proxyBaseURL,
	})
}

func SaveStateFile(path string, state State) error {
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	b = append(b, '\n')
	if err := writeFileAtomic(path, b, 0o600); err != nil {
		return err
	}
	return nil
}

func writeFileAtomic(path string, b []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".zola-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return fmt.Errorf("set file permissions: %w", err)
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return fmt.Errorf("write file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if runtime.GOOS == "windows" {
		_ = os.Remove(path)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace file %s: %w", path, err)
	}
	return nil
}
