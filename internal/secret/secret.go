package secret

import (
	"errors"
	"fmt"
	"os"

	"github.com/zalando/go-keyring"

	"zola/internal/config"
)

const Service = "zola"

var ErrNotFound = errors.New("secret not found")

type Store interface {
	Get(providerID string) (string, error)
	Set(providerID, value string) error
	Delete(providerID string) error
}

type KeyringStore struct{}

func (KeyringStore) Get(providerID string) (string, error) {
	value, err := keyring.Get(Service, providerID)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", ErrNotFound
		}
		return "", err
	}
	return value, nil
}

func (KeyringStore) Set(providerID, value string) error {
	return keyring.Set(Service, providerID, value)
}

func (KeyringStore) Delete(providerID string) error {
	err := keyring.Delete(Service, providerID)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// Resolve returns the API key for a provider, checking env, the legacy
// providers.json field, and then the OS keychain.
func Resolve(provider config.Provider) (string, error) {
	return ResolveWith(KeyringStore{}, provider)
}

func ResolveWith(store Store, provider config.Provider) (string, error) {
	if envKey := os.Getenv(provider.CodexEnvKey()); envKey != "" {
		return envKey, nil
	}
	if provider.APIKey != "" {
		return provider.APIKey, nil
	}
	value, err := store.Get(provider.ID)
	if err == nil {
		return value, nil
	}
	if errors.Is(err, ErrNotFound) {
		return "", fmt.Errorf("API key is not configured for provider %q; set %s, add --api-key, or use the OS keychain", provider.ID, provider.CodexEnvKey())
	}
	return "", fmt.Errorf("read API key from OS keychain for provider %q: %w", provider.ID, err)
}

func Check(store Store) error {
	_, err := store.Get("__zola_probe__")
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}
