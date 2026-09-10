package codex

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"zola/internal/config"
)

type Manager struct {
	Home       string
	ConfigPath string
}

func NewManager() (*Manager, error) {
	home, err := CodexHome()
	if err != nil {
		return nil, err
	}
	return NewManagerAt(home), nil
}

func NewManagerAt(home string) *Manager {
	return &Manager{
		Home:       home,
		ConfigPath: filepath.Join(home, "config.toml"),
	}
}

func CodexHome() (string, error) {
	if override := os.Getenv("CODEX_HOME"); override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".codex"), nil
}

func (m *Manager) Apply(provider config.Provider) error {
	return m.apply(provider, provider.BaseURL, provider.CodexEnvKey(), provider.WireAPI, nil, []string{"experimental_bearer_token", "requires_openai_auth"})
}

func (m *Manager) ApplyWithEndpoint(provider config.Provider, baseURL string) error {
	return m.apply(provider, baseURL, provider.CodexEnvKey(), provider.WireAPI, nil, []string{"experimental_bearer_token", "requires_openai_auth"})
}

func (m *Manager) ApplyProxy(provider config.Provider, proxyBaseURL string) error {
	return m.apply(provider, proxyBaseURL, "", config.WireAPIResponses, nil, []string{"experimental_bearer_token", "requires_openai_auth"})
}

// ApplyBearerToken persists the API key directly into Codex config so `codex`
// works without exporting an environment variable. Codex documents this field
// as experimental but programmatically supported.
func (m *Manager) ApplyBearerToken(provider config.Provider, token string) error {
	return m.apply(
		provider,
		provider.BaseURL,
		"",
		provider.WireAPI,
		map[string]any{"experimental_bearer_token": token},
		[]string{"env_key", "requires_openai_auth"},
	)
}

func (m *Manager) apply(
	provider config.Provider,
	baseURL, envKey, wireAPI string,
	extra map[string]any,
	unset []string,
) error {
	cfg, err := m.Read()
	if err != nil {
		return err
	}

	providers, ok := cfg["model_providers"].(map[string]any)
	if !ok || providers == nil {
		providers = map[string]any{}
		cfg["model_providers"] = providers
	}
	entry, ok := providers[provider.ID].(map[string]any)
	if !ok || entry == nil {
		entry = map[string]any{}
	}
	entry["name"] = provider.Name
	entry["base_url"] = strings.TrimRight(baseURL, "/")
	entry["wire_api"] = wireAPI
	if envKey != "" {
		entry["env_key"] = envKey
	} else {
		delete(entry, "env_key")
	}
	for key, value := range extra {
		entry[key] = value
	}
	for _, key := range unset {
		delete(entry, key)
	}
	providers[provider.ID] = entry
	cfg["model"] = provider.CodexModelName()
	cfg["model_provider"] = provider.ID
	if provider.ContextWindow > 0 {
		cfg["model_context_window"] = provider.ContextWindow
	} else {
		delete(cfg, "model_context_window")
	}

	return m.Write(cfg)
}

func (m *Manager) Read() (map[string]any, error) {
	cfg := map[string]any{}
	b, err := os.ReadFile(m.ConfigPath)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read codex config %s: %w", m.ConfigPath, err)
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return cfg, nil
	}
	if err := toml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse codex config %s: %w", m.ConfigPath, err)
	}
	return cfg, nil
}

func (m *Manager) Write(cfg map[string]any) error {
	b, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode codex config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(m.ConfigPath), 0o700); err != nil {
		return fmt.Errorf("create codex home %s: %w", filepath.Dir(m.ConfigPath), err)
	}
	if err := os.WriteFile(m.ConfigPath, b, 0o600); err != nil {
		return fmt.Errorf("write codex config %s: %w", m.ConfigPath, err)
	}
	return nil
}

func (m *Manager) Exists() bool {
	_, err := os.Stat(m.ConfigPath)
	return err == nil
}
