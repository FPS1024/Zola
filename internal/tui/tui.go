package tui

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"zola/internal/api"
	"zola/internal/codex"
	"zola/internal/config"
	localproxy "zola/internal/proxy"
	"zola/internal/secret"
)

const (
	screenList   = "list"
	screenForm   = "form"
	screenDelete = "delete"
)

type Model struct {
	Version        string
	Providers      []config.Provider
	Selected       int
	Current        string
	Status         string
	LaunchProvider *config.Provider
	Width          int
	Screen         string
	Form           *providerForm
	PendingDelete  config.Provider
	Mode           string
	ProxyBaseURL   string
	httpServer     *http.Server
}

func Run(version string) error {
	model, err := newModel(version)
	if err != nil {
		return err
	}
	program := tea.NewProgram(model, tea.WithAltScreen())
	final, err := program.Run()
	if err != nil {
		return fmt.Errorf("run terminal UI: %w", err)
	}
	model, ok := final.(Model)
	if !ok {
		return nil
	}
	if model.LaunchProvider == nil {
		model.stopProxyServer()
		return nil
	}
	defer model.stopProxyServer()

	provider := *model.LaunchProvider
	manager, err := codex.NewManager()
	if err != nil {
		return err
	}
	apiKey := ""
	state, err := config.LoadState()
	if err != nil {
		return err
	}
	if state.Mode == config.ModeProxy && state.ProxyBaseURL != "" {
		if err := manager.ApplyProxy(provider, state.ProxyBaseURL); err != nil {
			return err
		}
	} else {
		if err := manager.Apply(provider); err != nil {
			return err
		}
		apiKey, err = secret.Resolve(provider)
		if err != nil {
			return err
		}
	}
	_, _ = fmt.Fprintf(os.Stderr, "Launching codex for provider %s\n", provider.ID)
	return manager.LaunchWithKey(provider, apiKey, nil)
}

func (m *Model) stopProxyServer() {
	if m.httpServer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = m.httpServer.Shutdown(ctx)
	m.httpServer = nil
}

func (m *Model) startProxyServer(provider config.Provider) error {
	apiKey, err := secret.Resolve(provider)
	if err != nil {
		return err
	}
	addr := "127.0.0.1:8317"
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("start local proxy on %s: %w", addr, err)
	}
	handler := localproxy.NewHandler(localproxy.HandlerOptions{
		Provider: provider,
		APIKey:   apiKey,
		Logger:   nil,
	})
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	m.httpServer = server
	go func() {
		_ = server.Serve(listener)
	}()
	return nil
}

func newModel(version string) (Model, error) {
	model := Model{Version: version, Screen: screenList}
	store, err := loadStore()
	if err != nil {
		return model, err
	}
	model.Providers = store.Providers
	state, err := config.LoadState()
	if err != nil {
		return model, err
	}
	model.Current = state.CurrentProvider
	model.Mode = state.Mode
	model.ProxyBaseURL = state.ProxyBaseURL
	for i, provider := range model.Providers {
		if provider.ID == model.Current {
			model.Selected = i
			break
		}
	}
	return model, nil
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	default:
		return m, nil
	}
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch m.Screen {
	case screenForm:
		return m.handleFormKey(msg)
	case screenDelete:
		return m.handleDeleteKey(msg)
	default:
		return m.handleListKey(msg)
	}
}

func (m Model) handleListKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if len(m.Providers) == 0 {
			return m, nil
		}
		m.Selected--
		if m.Selected < 0 {
			m.Selected = len(m.Providers) - 1
		}
		return m, nil
	case "down", "j":
		if len(m.Providers) == 0 {
			return m, nil
		}
		m.Selected = (m.Selected + 1) % len(m.Providers)
		return m, nil
	case "a", "A":
		m.Form = newProviderForm(false, config.Provider{})
		m.Screen = screenForm
		m.Status = ""
		return m, nil
	case "e", "E":
		provider, ok := m.selectedProvider()
		if !ok {
			m.Status = "No providers configured."
			return m, nil
		}
		m.Form = newProviderForm(true, provider)
		m.Screen = screenForm
		m.Status = ""
		return m, nil
	case "d", "D":
		provider, ok := m.selectedProvider()
		if !ok {
			m.Status = "No providers configured."
			return m, nil
		}
		m.PendingDelete = provider
		m.Screen = screenDelete
		m.Status = ""
		return m, nil
	case "p", "P":
		provider, ok := m.selectedProvider()
		if !ok {
			m.Status = "No providers configured."
			return m, nil
		}
		updated, err := m.toggleProxyMode(provider)
		if err != nil {
			m.Status = err.Error()
			return m, nil
		}
		return updated, nil
	case "enter":
		provider, ok := m.selectedProvider()
		if !ok {
			m.Status = "No providers configured."
			return m, nil
		}
		updated, err := m.useProvider(provider)
		if err != nil {
			m.Status = err.Error()
			return m, nil
		}
		return updated, nil
	case "t":
		provider, ok := m.selectedProvider()
		if !ok {
			m.Status = "No providers configured."
			return m, nil
		}
		m.Status = testProvider(provider, m.Version)
		return m, nil
	case "r":
		provider, ok := m.selectedProvider()
		if !ok {
			m.Status = "No providers configured."
			return m, nil
		}
		updated, err := m.useProvider(provider)
		if err != nil {
			m.Status = err.Error()
			return m, nil
		}
		m = updated
		providerCopy := provider
		m.LaunchProvider = &providerCopy
		m.Status = "Launching codex..."
		return m, tea.Quit
	default:
		return m, nil
	}
}

func (m Model) handleFormKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.Form == nil {
		m.Screen = screenList
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.Screen = screenList
		m.Form = nil
		m.Status = "Canceled."
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "tab", "down":
		m.Form.focusNext()
		return m, nil
	case "shift+tab", "up":
		m.Form.focusPrev()
		return m, nil
	case "enter":
		if m.Form.focused < len(m.Form.inputs)-1 {
			m.Form.focusNext()
			return m, nil
		}
		updated, err := m.submitForm()
		if err != nil {
			m.Status = err.Error()
			return m, nil
		}
		return updated, nil
	default:
		input, cmd := m.Form.inputs[m.Form.focused].Update(msg)
		m.Form.inputs[m.Form.focused] = input
		return m, cmd
	}
}

func (m Model) handleDeleteKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "d", "D", "y", "Y", "enter":
		updated, err := m.deleteProvider()
		if err != nil {
			m.Status = err.Error()
			m.Screen = screenList
			m.PendingDelete = config.Provider{}
			return m, nil
		}
		return updated, nil
	case "esc", "q", "Q":
		m.Screen = screenList
		m.PendingDelete = config.Provider{}
		m.Status = "Delete canceled."
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	default:
		return m, nil
	}
}

func (m Model) submitForm() (Model, error) {
	provider, apiKey, err := m.Form.provider()
	if err != nil {
		return m, err
	}
	if !m.Form.editing && apiKey == "" {
		return m, fmt.Errorf("API key is required")
	}
	originalID := provider.ID
	if m.Form.editing {
		originalID = m.Form.original.ID
	}
	warning, err := saveProvider(provider, originalID, apiKey, m.Form.editing)
	if err != nil {
		return m, err
	}
	if err := m.reload(); err != nil {
		return m, err
	}
	action := "Added"
	if m.Form.editing {
		action = "Updated"
	}
	m.Status = action + " provider " + provider.ID
	if warning != "" {
		m.Status += " | " + warning
	}
	m.selectID(provider.ID)
	if m.Mode == config.ModeProxy && m.ProxyBaseURL != "" && m.Current == provider.ID {
		m.stopProxyServer()
		if err := m.startProxyServer(provider); err != nil {
			m.Status += " | proxy restart failed: " + err.Error()
		}
	}
	m.Screen = screenList
	m.Form = nil
	return m, nil
}

func (m Model) deleteProvider() (Model, error) {
	path, err := config.ProvidersPath()
	if err != nil {
		return m, err
	}
	store, err := config.LoadStore(path)
	if err != nil {
		return m, err
	}
	removed, ok := store.Remove(m.PendingDelete.ID)
	if !ok {
		return m, fmt.Errorf("provider %q does not exist", m.PendingDelete.ID)
	}
	if err := store.Save(); err != nil {
		return m, err
	}
	if removed.APIKey == "" {
		_ = (secret.KeyringStore{}).Delete(removed.ID)
	}
	state, err := config.LoadState()
	if err != nil {
		return m, err
	}
	if state.CurrentProvider == removed.ID {
		if m.Mode == config.ModeProxy {
			m.stopProxyServer()
		}
		if err := config.SaveState(""); err != nil {
			return m, err
		}
	}
	m.Screen = screenList
	m.PendingDelete = config.Provider{}
	m.Form = nil
	if err := m.reload(); err != nil {
		return m, err
	}
	m.Status = "Deleted provider " + removed.ID
	return m, nil
}

func (m Model) useProvider(provider config.Provider) (Model, error) {
	manager, err := codex.NewManager()
	if err != nil {
		return m, err
	}
	if m.Mode == config.ModeProxy && m.ProxyBaseURL != "" {
		if err := manager.ApplyProxy(provider, m.ProxyBaseURL); err != nil {
			return m, err
		}
		if err := config.SaveStateWithMode(provider.ID, config.ModeProxy, m.ProxyBaseURL); err != nil {
			return m, err
		}
		m.Current = provider.ID
		m.Status = "Selected provider " + provider.ID + " through proxy"
		return m, nil
	}
	if err := manager.Apply(provider); err != nil {
		return m, err
	}
	if err := config.SaveStateWithMode(provider.ID, config.ModeDirect, ""); err != nil {
		return m, err
	}
	m.Current = provider.ID
	m.Mode = config.ModeDirect
	m.ProxyBaseURL = ""
	m.Status = "Selected provider " + provider.ID
	return m, nil
}

func (m Model) toggleProxyMode(provider config.Provider) (Model, error) {
	manager, err := codex.NewManager()
	if err != nil {
		return m, err
	}
	if m.Mode == config.ModeProxy && m.ProxyBaseURL != "" {
		if err := manager.Apply(provider); err != nil {
			return m, err
		}
		if err := config.SaveStateWithMode(provider.ID, config.ModeDirect, ""); err != nil {
			return m, err
		}
		m.stopProxyServer()
		m.Current = provider.ID
		m.Mode = config.ModeDirect
		m.ProxyBaseURL = ""
		m.Status = "Proxy mode disabled for " + provider.ID
		return m, nil
	}
	proxyURL := "http://127.0.0.1:8317/v1"
	if err := m.startProxyServer(provider); err != nil {
		return m, err
	}
	if err := manager.ApplyProxy(provider, proxyURL); err != nil {
		m.stopProxyServer()
		return m, err
	}
	if err := config.SaveStateWithMode(provider.ID, config.ModeProxy, proxyURL); err != nil {
		m.stopProxyServer()
		return m, err
	}
	m.Current = provider.ID
	m.Mode = config.ModeProxy
	m.ProxyBaseURL = proxyURL
	m.Status = "Proxy mode enabled for " + provider.ID
	return m, nil
}

func (m *Model) reload() error {
	store, err := loadStore()
	if err != nil {
		return err
	}
	state, err := config.LoadState()
	if err != nil {
		return err
	}
	m.Providers = store.Providers
	if len(m.Providers) == 0 {
		m.Selected = 0
	} else if m.Selected >= len(m.Providers) {
		m.Selected = len(m.Providers) - 1
	}
	m.Current = state.CurrentProvider
	m.Mode = state.Mode
	m.ProxyBaseURL = state.ProxyBaseURL
	return nil
}

func (m *Model) selectID(id string) {
	for i := range m.Providers {
		if m.Providers[i].ID == id {
			m.Selected = i
			return
		}
	}
}

func (m Model) selectedProvider() (config.Provider, bool) {
	if len(m.Providers) == 0 || m.Selected < 0 || m.Selected >= len(m.Providers) {
		return config.Provider{}, false
	}
	return m.Providers[m.Selected], true
}

func (m Model) View() string {
	switch m.Screen {
	case screenForm:
		return m.formView()
	case screenDelete:
		return m.deleteView()
	default:
		return m.listView()
	}
}

func (m Model) listView() string {
	var out strings.Builder
	out.WriteString("Zola Codex Provider Manager")
	version := strings.TrimPrefix(m.Version, "v")
	if version != "" && version != "dev" {
		out.WriteString(" v" + version)
	}
	out.WriteString("\n\n")

	if len(m.Providers) == 0 {
		out.WriteString("No providers configured. Press A to add one.\n")
	} else {
		out.WriteString("ID          MODEL               WIRE API     BASE URL\n")
		for i, provider := range m.Providers {
			cursor := " "
			if i == m.Selected {
				cursor = ">"
			}
			current := " "
			if provider.ID == m.Current {
				current = "*"
			}
			fmt.Fprintf(&out, "%s%s %-12s %-20s %-10s %s\n",
				cursor,
				current,
				provider.ID,
				provider.Model,
				provider.WireAPI,
				provider.BaseURL,
			)
		}
	}

	out.WriteString("\nCurrent: ")
	if m.Current == "" {
		out.WriteString("none")
	} else {
		out.WriteString(m.Current)
	}
	if m.Mode == config.ModeProxy {
		if m.httpServer != nil {
			out.WriteString(" (proxy running)")
		} else {
			out.WriteString(" (proxy)")
		}
	} else {
		out.WriteString(" (direct)")
	}
	out.WriteString("\n")
	if m.Status != "" {
		out.WriteString("\n" + m.Status + "\n")
	}
	out.WriteString("\nEnter Use | A Add | E Edit | D Delete | T Test | R Run | P Proxy | Q Quit\n")
	return out.String()
}

func (m Model) formView() string {
	var out strings.Builder
	title := "Add Provider"
	if m.Form.editing {
		title = "Edit Provider " + m.Form.original.ID
	}
	out.WriteString(title + "\n\n")
	for i := range m.Form.labels {
		if m.Form.editing && i == 0 {
			fmt.Fprintf(&out, "%-12s %s\n", m.Form.labels[i], m.Form.inputs[i].Value())
			continue
		}
		fmt.Fprintf(&out, "%-12s %s\n", m.Form.labels[i], m.Form.inputs[i].View())
	}
	if m.Status != "" {
		out.WriteString("\n" + m.Status + "\n")
	}
	out.WriteString("\nTab/Up/Down move | Enter next/submit | Esc cancel\n")
	return out.String()
}

func (m Model) deleteView() string {
	return fmt.Sprintf(
		"Delete provider %s (%s)?\n\nD or Enter to confirm | Esc to cancel",
		m.PendingDelete.ID,
		m.PendingDelete.Name,
	)
}

func testProvider(provider config.Provider, version string) string {
	if provider.WireAPI != config.WireAPIResponses {
		return fmt.Sprintf("test requires wire_api=responses; %s uses %s", provider.ID, provider.WireAPI)
	}
	apiKey, err := secret.Resolve(provider)
	if err != nil {
		return err.Error()
	}
	timeout := time.Duration(provider.Timeout) * time.Second
	client, err := api.NewProbeClient(api.ProbeOptions{
		Model:     provider.Model,
		BaseURL:   provider.BaseURL,
		APIKey:    apiKey,
		Timeout:   timeout,
		UserAgent: "zola/" + version,
	})
	if err != nil {
		return err.Error()
	}
	result, err := client.Probe(context.Background())
	if err != nil {
		return err.Error()
	}
	status := fmt.Sprintf("OK HTTP %d status=%s", result.HTTPStatus, result.ResponseStatus)
	if result.ResponseModel != "" {
		status += " model=" + result.ResponseModel
	}
	status += " in " + result.Duration.Round(time.Millisecond).String()
	return status
}

func loadStore() (*config.Store, error) {
	path, err := config.ProvidersPath()
	if err != nil {
		return nil, err
	}
	return config.LoadStore(path)
}
