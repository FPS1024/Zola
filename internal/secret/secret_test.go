package secret

import (
	"testing"

	"zola/internal/config"
)

type memoryStore struct {
	values map[string]string
}

func newMemoryStore() *memoryStore {
	return &memoryStore{values: map[string]string{}}
}

func (s *memoryStore) Get(providerID string) (string, error) {
	value, ok := s.values[providerID]
	if !ok {
		return "", ErrNotFound
	}
	return value, nil
}

func (s *memoryStore) Set(providerID, value string) error {
	s.values[providerID] = value
	return nil
}

func (s *memoryStore) Delete(providerID string) error {
	delete(s.values, providerID)
	return nil
}

func TestResolveWithKeyringStore(t *testing.T) {
	store := newMemoryStore()
	if err := store.Set("deepseek", "sk-keychain"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	provider := config.Provider{
		ID:      "deepseek",
		EnvKey:  "DEEPSEEK_API_KEY",
		APIKey:  "sk-legacy",
		WireAPI: config.WireAPIResponses,
	}
	got, err := ResolveWith(store, provider)
	if err != nil {
		t.Fatalf("ResolveWith: %v", err)
	}
	if got != "sk-legacy" {
		t.Fatalf("key = %q, want legacy providers.json key", got)
	}

	provider.APIKey = ""
	got, err = ResolveWith(store, provider)
	if err != nil {
		t.Fatalf("ResolveWith: %v", err)
	}
	if got != "sk-keychain" {
		t.Fatalf("key = %q, want keychain key", got)
	}
}
