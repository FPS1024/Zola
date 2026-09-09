package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"

	"zola/internal/config"
	"zola/internal/secret"
)

type providerForm struct {
	editing  bool
	original config.Provider
	labels   []string
	inputs   []textinput.Model
	focused  int
}

func newProviderForm(editing bool, provider config.Provider) *providerForm {
	labels := []string{
		"ID",
		"Name",
		"Base URL",
		"Model",
		"Wire API",
		"Env key",
		"API key",
	}
	placeholders := []string{
		"deepseek",
		"DeepSeek",
		"https://api.deepseek.com",
		"deepseek-v4-flash",
		"responses",
		"DEEPSEEK_API_KEY",
		"sk-...",
	}
	form := &providerForm{
		editing:  editing,
		original: provider,
		labels:   labels,
		focused:  0,
	}
	form.inputs = make([]textinput.Model, len(labels))
	for i := range labels {
		input := textinput.New()
		input.Prompt = "  "
		input.Placeholder = placeholders[i]
		input.PromptStyle = lipgloss.NewStyle().Faint(true)
		input.PlaceholderStyle = lipgloss.NewStyle().Faint(true)
		input.CharLimit = 256
		input.Width = 64
		if i == 6 {
			input.EchoMode = textinput.EchoPassword
		}
		switch {
		case editing && i == 0:
			input.SetValue(provider.ID)
			input.Prompt = "    "
		case editing && i == 1 && provider.Name != "":
			input.SetValue(provider.Name)
		case editing && i == 2:
			input.SetValue(provider.BaseURL)
		case editing && i == 3:
			input.SetValue(provider.Model)
		case editing && i == 4 && provider.WireAPI != "":
			input.SetValue(provider.WireAPI)
		case editing && i == 5 && provider.EnvKey != "":
			input.SetValue(provider.EnvKey)
		}
		if editing && i == 0 {
			input.Blur()
		}
		form.inputs[i] = input
	}
	if editing {
		form.focused = 1
	} else {
		form.focused = 0
	}
	form.focusField(form.focused)
	return form
}

func (f *providerForm) focusField(index int) {
	if len(f.inputs) == 0 {
		return
	}
	if f.editing && index == 0 {
		index = 1
	}
	if index < 0 || index >= len(f.inputs) {
		return
	}
	f.focused = index
	for i := range f.inputs {
		if i == index {
			f.inputs[i].Focus()
		} else {
			f.inputs[i].Blur()
		}
	}
}

func (f *providerForm) focusNext() {
	if f.editing {
		if f.focused == len(f.inputs)-1 {
			f.focusField(1)
		} else {
			f.focusField(f.focused + 1)
		}
		return
	}
	f.focusField((f.focused + 1) % len(f.inputs))
}

func (f *providerForm) focusPrev() {
	if f.editing {
		if f.focused == 1 {
			f.focusField(len(f.inputs) - 1)
		} else {
			f.focusField(f.focused - 1)
		}
		return
	}
	f.focusField((f.focused - 1 + len(f.inputs)) % len(f.inputs))
}

func (f *providerForm) provider() (config.Provider, string, error) {
	values := make([]string, len(f.inputs))
	for i := range f.inputs {
		values[i] = strings.TrimSpace(f.inputs[i].Value())
	}
	if !f.editing && values[0] == "" {
		return config.Provider{}, "", fmt.Errorf("provider id is required")
	}
	provider := config.Provider{
		ID:      strings.TrimSpace(f.original.ID),
		Name:    values[1],
		BaseURL: values[2],
		Model:   values[3],
		WireAPI: values[4],
		EnvKey:  values[5],
		Enabled: true,
	}
	if !f.editing {
		provider.ID = values[0]
	}
	if f.editing {
		provider.APIKey = f.original.APIKey
	}
	if values[4] == "" {
		provider.WireAPI = config.WireAPIResponses
	}
	provider.Normalize()
	if err := provider.Validate(); err != nil {
		return config.Provider{}, "", err
	}
	return provider, values[6], nil
}

func saveProvider(provider config.Provider, originalID, apiKey string, editing bool) (string, error) {
	path, err := config.ProvidersPath()
	if err != nil {
		return "", err
	}
	store, err := config.LoadStore(path)
	if err != nil {
		return "", err
	}

	if apiKey != "" {
		provider.APIKey = apiKey
	}
	warning := ""
	if provider.APIKey != "" {
		if err := (secret.KeyringStore{}).Set(provider.ID, provider.APIKey); err != nil {
			if os.Getenv("ZOLA_SECRET_BACKEND") == "keyring" {
				return "", fmt.Errorf("store API key in OS keychain: %w", err)
			}
			warning = "OS keychain unavailable; API key stored in providers.json with 0600 permissions"
		} else {
			provider.APIKey = ""
		}
	}

	if editing {
		if err := store.Update(originalID, provider); err != nil {
			return warning, err
		}
		return warning, nil
	}
	if err := store.Add(provider); err != nil {
		return warning, err
	}
	return warning, nil
}
