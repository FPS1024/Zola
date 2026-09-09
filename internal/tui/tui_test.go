package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"zola/internal/config"
)

func TestViewShowsProviderWithoutVersionPrefixDouble(t *testing.T) {
	model := Model{
		Version: "v0.2.0",
		Providers: []config.Provider{{
			ID: "deepseek", Name: "DeepSeek", Model: "deepseek-v4-flash",
			WireAPI: config.WireAPIResponses, BaseURL: "https://api.deepseek.com",
		}},
		Selected: 0,
		Current:  "deepseek",
	}
	view := model.View()
	if strings.Contains(view, "vv0.2.0") {
		t.Fatalf("view contains double version prefix: %s", view)
	}
	if !strings.Contains(view, "deepseek-v4-flash") || !strings.Contains(view, "Zola Codex Provider Manager v0.2.0") {
		t.Fatalf("view is missing expected content: %s", view)
	}
}

func TestSelectNextAndQuit(t *testing.T) {
	model := Model{
		Version: "dev",
		Providers: []config.Provider{
			{ID: "deepseek", Name: "DeepSeek", Model: "deepseek-v4-flash", WireAPI: config.WireAPIResponses, BaseURL: "https://api.deepseek.com"},
			{ID: "openai", Name: "OpenAI", Model: "gpt-5", WireAPI: config.WireAPIResponses, BaseURL: "https://api.openai.com/v1"},
		},
		Selected: 0,
	}
	next, _ := model.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	if next.Selected != 1 {
		t.Fatalf("Selected = %d, want 1", next.Selected)
	}
	quitting, quit := model.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if quitting.LaunchProvider != nil {
		t.Fatal("quit should not launch Codex")
	}
	if quit == nil {
		t.Fatal("quit command is nil")
	}
}

func TestAddAndEditOpenForms(t *testing.T) {
	model := Model{
		Version: "dev",
		Providers: []config.Provider{{
			ID: "deepseek", Name: "DeepSeek", Model: "deepseek-v4-flash",
			WireAPI: config.WireAPIResponses, BaseURL: "https://api.deepseek.com",
		}},
		Selected: 0,
		Screen:   screenList,
	}
	add, _ := model.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if add.Screen != screenForm || add.Form == nil {
		t.Fatalf("add key did not open form: screen=%q form=%#v", add.Screen, add.Form)
	}
	edit, _ := model.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if edit.Screen != screenForm || edit.Form == nil || !edit.Form.editing {
		t.Fatalf("edit key did not open edit form: screen=%q form=%#v", edit.Screen, edit.Form)
	}
}

func TestDeleteOpensConfirmation(t *testing.T) {
	model := Model{
		Providers: []config.Provider{{ID: "deepseek", Name: "DeepSeek", Model: "deepseek-v4-flash", WireAPI: config.WireAPIResponses, BaseURL: "https://api.deepseek.com"}},
		Selected:  0,
		Screen:    screenList,
	}
	deleting, _ := model.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if deleting.Screen != screenDelete || deleting.PendingDelete.ID != "deepseek" {
		t.Fatalf("delete key did not open confirmation: screen=%q pending=%+v", deleting.Screen, deleting.PendingDelete)
	}
}

func TestListViewPromisesFullActions(t *testing.T) {
	model := Model{
		Version:   "dev",
		Screen:    screenList,
		Mode:      config.ModeDirect,
		Providers: []config.Provider{{ID: "deepseek", Name: "DeepSeek", Model: "deepseek-v4-flash", WireAPI: config.WireAPIResponses, BaseURL: "https://api.deepseek.com"}},
	}
	view := model.View()
	for _, expected := range []string{"A Add", "E Edit", "D Delete", "T Test", "R Run", "P Proxy"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("view missing %q:\n%s", expected, view)
		}
	}
}

func TestProviderFormCollectsValues(t *testing.T) {
	form := newProviderForm(false, config.Provider{})
	form.inputs[0].SetValue("local")
	form.inputs[1].SetValue("Local")
	form.inputs[2].SetValue("http://127.0.0.1:8080/v1")
	form.inputs[3].SetValue("local-model")
	form.inputs[4].SetValue("chat")
	form.inputs[5].SetValue("LOCAL_API_KEY")
	form.inputs[6].SetValue("sk-secret")
	provider, apiKey, err := form.provider()
	if err != nil {
		t.Fatalf("provider(): %v", err)
	}
	if provider.ID != "local" || provider.WireAPI != config.WireAPIChat || provider.EnvKey != "LOCAL_API_KEY" {
		t.Fatalf("provider = %+v", provider)
	}
	if apiKey != "sk-secret" {
		t.Fatalf("apiKey = %q", apiKey)
	}
}
