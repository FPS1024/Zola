package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProviderStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "providers.json")
	store := NewStore(path)
	provider := Provider{
		ID:      "deepseek",
		Name:    "DeepSeek",
		BaseURL: "https://api.deepseek.com",
		APIKey:  "sk-secret",
		Model:   "deepseek-v4-flash",
		WireAPI: WireAPIResponses,
		EnvKey:  "DEEPSEEK_API_KEY",
		Enabled: true,
	}
	if err := store.Add(provider); err != nil {
		t.Fatalf("Add: %v", err)
	}

	loaded, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if len(loaded.Providers) != 1 {
		t.Fatalf("providers length = %d, want 1", len(loaded.Providers))
	}
	got := loaded.Providers[0]
	if got.ID != provider.ID || got.APIKey != provider.APIKey || got.EnvKey != provider.EnvKey {
		t.Fatalf("provider round trip mismatch: %+v", got)
	}
}

func TestProviderStoreRejectsDuplicate(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "providers.json"))
	provider := Provider{
		ID:      "deepseek",
		Name:    "DeepSeek",
		BaseURL: "https://api.deepseek.com",
		Model:   "deepseek-v4-flash",
		WireAPI: WireAPIResponses,
	}
	if err := store.Add(provider); err != nil {
		t.Fatalf("first Add: %v", err)
	}
	if err := store.Add(provider); !errors.Is(err, ErrProviderExists) {
		t.Fatalf("second Add error = %v, want ErrProviderExists", err)
	}
}

func TestProviderValidation(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
		wantErr  bool
	}{
		{
			name: "valid deepseek preset",
			provider: Provider{
				ID: "deepseek", Name: "DeepSeek", BaseURL: "https://api.deepseek.com",
				Model: "deepseek-v4-flash", WireAPI: WireAPIResponses,
			},
		},
		{
			name: "invalid wire api",
			provider: Provider{
				ID: "deepseek", Name: "DeepSeek", BaseURL: "https://api.deepseek.com",
				Model: "deepseek-v4-flash", WireAPI: "magic",
			},
			wantErr: true,
		},
		{
			name: "invalid base url",
			provider: Provider{
				ID: "deepseek", Name: "DeepSeek", BaseURL: "not a url",
				Model: "deepseek-v4-flash", WireAPI: WireAPIResponses,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.provider.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestProviderNormalizeDefaults(t *testing.T) {
	provider := Provider{ID: "my-api", BaseURL: "http://localhost:8000/", Model: "qwen"}
	provider.Normalize()
	if provider.WireAPI != WireAPIResponses {
		t.Fatalf("WireAPI = %q, want responses", provider.WireAPI)
	}
	if provider.Name != "my-api" {
		t.Fatalf("Name = %q, want my-api", provider.Name)
	}
	if provider.Timeout != 60 {
		t.Fatalf("Timeout = %d, want 60", provider.Timeout)
	}
	if strings.HasSuffix(provider.BaseURL, "/") {
		t.Fatalf("BaseURL still has trailing slash: %q", provider.BaseURL)
	}
	if provider.CodexEnvKey() != "ZOLA_MY_API_API_KEY" {
		t.Fatalf("CodexEnvKey = %q, want ZOLA_MY_API_API_KEY", provider.CodexEnvKey())
	}
}

func TestStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := SaveStateFile(path, State{CurrentProvider: "deepseek", Mode: ModeDirect}); err != nil {
		t.Fatalf("SaveStateFile: %v", err)
	}
	state, err := LoadStateFile(path)
	if err != nil {
		t.Fatalf("LoadStateFile: %v", err)
	}
	if state.CurrentProvider != "deepseek" {
		t.Fatalf("CurrentProvider = %q, want deepseek", state.CurrentProvider)
	}
	if state.Mode != ModeDirect {
		t.Fatalf("Mode = %q, want direct", state.Mode)
	}
}

func TestStateProxyRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	want := State{
		CurrentProvider: "deepseek",
		Mode:            ModeProxy,
		ProxyBaseURL:    "http://127.0.0.1:8317/v1",
	}
	if err := SaveStateFile(path, want); err != nil {
		t.Fatalf("SaveStateFile: %v", err)
	}
	state, err := LoadStateFile(path)
	if err != nil {
		t.Fatalf("LoadStateFile: %v", err)
	}
	if state != want {
		t.Fatalf("state = %+v, want %+v", state, want)
	}
}

func TestProviderStoreFilePermissions(t *testing.T) {
	if os.Getenv("ZOLA_SKIP_PERMISSION_TEST") != "" {
		t.Skip("permission assertion disabled")
	}
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not portable on Windows")
	}
	path := filepath.Join(t.TempDir(), "providers.json")
	store := NewStore(path)
	if err := store.Add(Provider{
		ID: "test", Name: "Test", BaseURL: "http://127.0.0.1:8080",
		Model: "model", WireAPI: WireAPIResponses,
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", info.Mode().Perm())
	}
}

func TestProviderJSONDoesNotLeakIntoUnrelatedFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "providers.json")
	store := NewStore(path)
	if err := store.Add(Provider{
		ID: "test", Name: "Test", BaseURL: "http://127.0.0.1:8080",
		Model: "model", WireAPI: WireAPIResponses,
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var decoded []Provider
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(decoded) != 1 {
		t.Fatalf("decoded providers = %d, want 1", len(decoded))
	}
}
