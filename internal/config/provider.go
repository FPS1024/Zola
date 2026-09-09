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
	WireAPI string `json:"wire_api,omitempty"`
	EnvKey  string `json:"env_key,omitempty"`
	Enabled bool   `json:"enabled,omitempty"`
	Timeout int    `json:"timeout,omitempty"`
}

func (p *Provider) Normalize() {
	p.ID = strings.TrimSpace(p.ID)
	p.Name = strings.TrimSpace(p.Name)
	p.BaseURL = strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
	p.Model = strings.TrimSpace(p.Model)
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

// CodexEnvKey returns the environment variable used by Codex to read the API key.
func (p Provider) CodexEnvKey() string {
	if p.EnvKey != "" {
		return p.EnvKey
	}
	normalized := strings.ToUpper(regexp.MustCompile(`[^A-Za-z0-9]`).ReplaceAllString(p.ID, "_"))
	return "ZOLA_" + normalized + "_API_KEY"
}
