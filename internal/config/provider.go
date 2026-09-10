package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const (
	WireAPIResponses = "responses"
	WireAPIChat      = "chat"
)

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
var envPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Provider struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key,omitempty"`
	Model   string `json:"model"`
	// CodexModel is the model name presented to Codex. When it differs from
	// Model, the proxy rewrites requests to the upstream model name.
	CodexModel string `json:"codex_model,omitempty"`
	WireAPI    string `json:"wire_api,omitempty"`
	EnvKey     string `json:"env_key,omitempty"`
	Enabled    bool   `json:"enabled,omitempty"`
	Timeout    int    `json:"timeout,omitempty"`
	// ContextWindow overrides Codex's model_context_window when nonzero.
	ContextWindow int `json:"context_window,omitempty"`
}

func (p *Provider) Normalize() {
	p.ID = strings.TrimSpace(p.ID)
	p.Name = strings.TrimSpace(p.Name)
	p.BaseURL = strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
	p.Model = strings.TrimSpace(p.Model)
	p.CodexModel = strings.TrimSpace(p.CodexModel)
	p.WireAPI = strings.ToLower(strings.TrimSpace(p.WireAPI))
	p.EnvKey = strings.TrimSpace(p.EnvKey)
	if p.Name == "" {
		p.Name = p.ID
	}
	if p.WireAPI == "" {
		p.WireAPI = WireAPIResponses
	}
	if p.Timeout <= 0 {
		p.Timeout = 60
	}
}

func (p Provider) Validate() error {
	if !idPattern.MatchString(p.ID) {
		return fmt.Errorf("provider id %q must start with a lowercase letter or digit and contain only lowercase letters, digits, dot, dash, or underscore", p.ID)
	}
	if p.Name == "" {
		return fmt.Errorf("provider name is required")
	}
	if p.Model == "" {
		return fmt.Errorf("provider model is required")
	}
	if p.ContextWindow < 0 {
		return fmt.Errorf("context_window must not be negative")
	}
	u, err := url.Parse(p.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("provider base_url must be a valid http(s) URL")
	}
	if p.WireAPI != WireAPIResponses && p.WireAPI != WireAPIChat {
		return fmt.Errorf("wire_api must be %q or %q", WireAPIResponses, WireAPIChat)
	}
	if p.EnvKey != "" && !envPattern.MatchString(p.EnvKey) {
		return fmt.Errorf("env_key must be a valid environment variable name")
	}
	return nil
}

// CodexModelName returns the model name that Codex should use.
func (p Provider) CodexModelName() string {
	if p.CodexModel != "" {
		return p.CodexModel
	}
	return p.Model
}

// IsDisguised reports whether Codex sees a different model name than the
// upstream provider.
func (p Provider) IsDisguised() bool {
	return p.CodexModel != "" && p.CodexModel != p.Model
}

// CodexEnvKey returns the environment variable used by Codex to read the API key.
func (p Provider) CodexEnvKey() string {
	if p.EnvKey != "" {
		return p.EnvKey
	}
	normalized := strings.ToUpper(regexp.MustCompile(`[^A-Za-z0-9]`).ReplaceAllString(p.ID, "_"))
	return "ZOLA_" + normalized + "_API_KEY"
}
