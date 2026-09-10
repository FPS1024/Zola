package codex

import (
	"os"
	"path/filepath"
	"testing"

	"zola/internal/config"
)

func TestApplyCreatesAndReadsCodexConfig(t *testing.T) {
	home := t.TempDir()
	manager := NewManagerAt(home)
	provider := config.Provider{
		ID:      "deepseek",
		Name:    "DeepSeek",
		BaseURL: "https://api.deepseek.com/",
		Model:   "deepseek-v4-flash",
		WireAPI: config.WireAPIResponses,
		EnvKey:  "DEEPSEEK_API_KEY",
	}
	if err := manager.Apply(provider); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	cfg, err := manager.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if cfg["model"] != provider.Model {
		t.Fatalf("model = %v, want %q", cfg["model"], provider.Model)
	}
	if cfg["model_provider"] != provider.ID {
		t.Fatalf("model_provider = %v, want %q", cfg["model_provider"], provider.ID)
	}
	providers, ok := cfg["model_providers"].(map[string]any)
	if !ok {
		t.Fatalf("model_providers = %T, want map[string]any", cfg["model_providers"])
	}
	entry, ok := providers["deepseek"].(map[string]any)
	if !ok {
		t.Fatalf("deepseek entry = %T, want map[string]any", providers["deepseek"])
	}
	if entry["env_key"] != "DEEPSEEK_API_KEY" || entry["wire_api"] != "responses" {
		t.Fatalf("deepseek entry = %v", entry)
	}
}

func TestApplyPreservesOtherCodexSettings(t *testing.T) {
	home := t.TempDir()
	manager := NewManagerAt(home)
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	original := "approval_policy = \"on-request\"\n\n[model_providers.other]\nname = \"Other\"\nbase_url = \"http://127.0.0.1:9000\"\nenv_key = \"OTHER_API_KEY\"\nwire_api = \"responses\"\n"
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(original), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	provider := config.Provider{
		ID: "deepseek", Name: "DeepSeek", BaseURL: "https://api.deepseek.com",
		Model: "deepseek-v4-flash", WireAPI: config.WireAPIResponses,
		EnvKey: "DEEPSEEK_API_KEY",
	}
	if err := manager.Apply(provider); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	cfg, err := manager.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if cfg["approval_policy"] != "on-request" {
		t.Fatalf("approval_policy lost: %v", cfg["approval_policy"])
	}
	providers, ok := cfg["model_providers"].(map[string]any)
	if !ok {
		t.Fatalf("model_providers = %T, want map[string]any", cfg["model_providers"])
	}
	if _, ok := providers["other"]; !ok {
		t.Fatalf("existing provider was lost: %v", providers)
	}
}

func TestApplyProxyOmitsEnvKey(t *testing.T) {
	home := t.TempDir()
	manager := NewManagerAt(home)
	provider := config.Provider{
		ID: "deepseek", Name: "DeepSeek", BaseURL: "https://api.deepseek.com",
		Model: "deepseek-v4-flash", WireAPI: config.WireAPIResponses,
		EnvKey: "DEEPSEEK_API_KEY",
	}
	if err := manager.ApplyProxy(provider, "http://127.0.0.1:8317/v1"); err != nil {
		t.Fatalf("ApplyProxy: %v", err)
	}
	cfg, err := manager.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	providers := cfg["model_providers"].(map[string]any)
	entry := providers["deepseek"].(map[string]any)
	if entry["base_url"] != "http://127.0.0.1:8317/v1" {
		t.Fatalf("base_url = %v", entry["base_url"])
	}
	if _, ok := entry["env_key"]; ok {
		t.Fatalf("proxy provider config should not contain env_key: %v", entry)
	}
	if entry["wire_api"] != config.WireAPIResponses {
		t.Fatalf("proxy wire_api = %v, want responses", entry["wire_api"])
	}
}

func TestApplyBearerTokenPersistsToken(t *testing.T) {
	home := t.TempDir()
	manager := NewManagerAt(home)
	provider := config.Provider{
		ID: "deepseek", Name: "DeepSeek", BaseURL: "https://api.deepseek.com",
		Model: "deepseek-v4-flash", WireAPI: config.WireAPIResponses,
		EnvKey: "DEEPSEEK_API_KEY",
	}
	if err := manager.ApplyBearerToken(provider, "sk-persisted"); err != nil {
		t.Fatalf("ApplyBearerToken: %v", err)
	}
	cfg, err := manager.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	providers := cfg["model_providers"].(map[string]any)
	entry := providers["deepseek"].(map[string]any)
	if entry["experimental_bearer_token"] != "sk-persisted" {
		t.Fatalf("experimental_bearer_token = %v", entry["experimental_bearer_token"])
	}
	if _, ok := entry["env_key"]; ok {
		t.Fatalf("env_key should be removed: %v", entry)
	}
	if _, ok := entry["requires_openai_auth"]; ok {
		t.Fatalf("requires_openai_auth should be removed: %v", entry)
	}
}

func TestApplyUsesCodexModelAliasAndContextWindow(t *testing.T) {
	home := t.TempDir()
	manager := NewManagerAt(home)
	provider := config.Provider{
		ID: "deepseek", Name: "DeepSeek", BaseURL: "https://api.deepseek.com",
		Model: "deepseek-v4-pro", CodexModel: "gpt-5.4",
		WireAPI: config.WireAPIResponses, ContextWindow: 1000000,
	}
	if err := manager.ApplyProxy(provider, "http://127.0.0.1:8317/v1"); err != nil {
		t.Fatalf("ApplyProxy: %v", err)
	}
	cfg, err := manager.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if cfg["model"] != "gpt-5.4" {
		t.Fatalf("model = %v, want gpt-5.4", cfg["model"])
	}
	if cfg["model_context_window"] != int64(1000000) {
		t.Fatalf("model_context_window = %#v", cfg["model_context_window"])
	}
}

func TestApplyPreservesExtraFieldsForSameProvider(t *testing.T) {
	home := t.TempDir()
	manager := NewManagerAt(home)
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	original := "[model_providers.deepseek]\nname = \"DeepSeek\"\nbase_url = \"https://old.example\"\ncustom_property = \"kept\"\nrequires_openai_auth = true\nexperimental_bearer_token = \"LOCAL_TOKEN\"\nwire_api = \"responses\"\n"
	if err := os.WriteFile(manager.ConfigPath, []byte(original), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	provider := config.Provider{
		ID: "deepseek", Name: "DeepSeek", BaseURL: "https://api.deepseek.com",
		Model: "deepseek-v4-flash", WireAPI: config.WireAPIResponses,
		EnvKey: "DEEPSEEK_API_KEY",
	}
	if err := manager.Apply(provider); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	cfg, err := manager.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	providers := cfg["model_providers"].(map[string]any)
	entry := providers["deepseek"].(map[string]any)
	if entry["custom_property"] != "kept" {
		t.Fatalf("custom_property was lost: %v", entry)
	}
	if _, ok := entry["experimental_bearer_token"]; ok {
		t.Fatalf("experimental_bearer_token should be removed in env mode: %v", entry)
	}
	if _, ok := entry["requires_openai_auth"]; ok {
		t.Fatalf("requires_openai_auth should be removed in env mode: %v", entry)
	}
}

func TestCodexHomeUsesOverride(t *testing.T) {
	t.Setenv("CODEX_HOME", filepath.Join(t.TempDir(), "codex"))
	home, err := CodexHome()
	if err != nil {
		t.Fatalf("CodexHome: %v", err)
	}
	if home == "" {
		t.Fatal("CodexHome returned empty string")
	}
}
