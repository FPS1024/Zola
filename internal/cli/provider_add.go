package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"zola/internal/config"
	"zola/internal/secret"
)

type providerOptions struct {
	id            string
	name          string
	baseURL       string
	apiKey        string
	model         string
	wireAPI       string
	envKey        string
	timeout       int
	codexModel    string
	contextWindow int
}

func newAddCommand() *cobra.Command {
	options := &providerOptions{}
	cmd := &cobra.Command{
		Use:   "add [id]",
		Short: "Add a provider",
		Long:  "Add a provider. Known presets: deepseek, openai, openrouter.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return addOrEditProvider(cmd, options, args, false)
		},
	}
	bindProviderFlags(cmd, options)
	return cmd
}

func newEditCommand() *cobra.Command {
	options := &providerOptions{}
	cmd := &cobra.Command{
		Use:   "edit [id]",
		Short: "Edit a provider",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return addOrEditProvider(cmd, options, args, true)
		},
	}
	bindProviderFlags(cmd, options)
	return cmd
}

func bindProviderFlags(cmd *cobra.Command, options *providerOptions) {
	cmd.Flags().StringVar(&options.id, "id", "", "provider id")
	cmd.Flags().StringVar(&options.name, "name", "", "display name")
	cmd.Flags().StringVar(&options.baseURL, "base-url", "", "API base URL")
	cmd.Flags().StringVar(&options.apiKey, "api-key", "", "API key (stored in the OS keychain when available)")
	cmd.Flags().StringVar(&options.model, "model", "", "default model")
	cmd.Flags().StringVar(&options.wireAPI, "wire-api", "", "responses or chat")
	cmd.Flags().StringVar(&options.envKey, "env-key", "", "Codex environment variable name")
	cmd.Flags().IntVar(&options.timeout, "timeout", 0, "request timeout in seconds")
	cmd.Flags().StringVar(&options.codexModel, "codex-model", "", "model name shown to Codex")
	cmd.Flags().IntVar(&options.contextWindow, "context-window", 0, "Codex model_context_window override")
}

func addOrEditProvider(cmd *cobra.Command, options *providerOptions, args []string, edit bool) error {
	flagID := strings.TrimSpace(options.id)
	id := flagID
	if len(args) == 1 {
		argID := strings.TrimSpace(args[0])
		if id == "" {
			id = argID
		} else if id != argID {
			return fmt.Errorf("provider id specified twice with different values: %q and %q", id, argID)
		}
	}

	reader := bufio.NewReader(cmd.InOrStdin())
	writer := cmd.ErrOrStderr()
	if id == "" {
		var err error
		id, err = prompt(reader, writer, "Provider id", "")
		if err != nil {
			return err
		}
	}

	store, err := loadDefaultStore()
	if err != nil {
		return err
	}

	var provider config.Provider
	if edit {
		existing, ok := store.Find(id)
		if !ok {
			return fmt.Errorf("provider %q does not exist", id)
		}
		provider = existing
	} else {
		preset, known := config.Preset(id)
		if known {
			provider = preset
		} else {
			provider = config.Provider{ID: id, Enabled: true}
		}
	}
	if options.name != "" {
		provider.Name = options.name
	}
	if options.baseURL != "" {
		provider.BaseURL = options.baseURL
	}
	if options.model != "" {
		provider.Model = options.model
	}
	if options.wireAPI != "" {
		provider.WireAPI = options.wireAPI
	}
	if options.envKey != "" {
		provider.EnvKey = options.envKey
	}
	if options.codexModel != "" {
		provider.CodexModel = options.codexModel
	}
	if options.contextWindow > 0 {
		provider.ContextWindow = options.contextWindow
	}
	if options.apiKey != "" {
		provider.APIKey = options.apiKey
	}
	provider.Enabled = true

	if !edit {
		if provider.Name == "" {
			name, err := prompt(reader, writer, "Provider name", provider.Name)
			if err != nil {
				return err
			}
			provider.Name = name
		}
		if provider.BaseURL == "" {
			baseURL, err := prompt(reader, writer, "Base URL", provider.BaseURL)
			if err != nil {
				return err
			}
			provider.BaseURL = baseURL
		}
		if provider.Model == "" {
			model, err := prompt(reader, writer, "Default model", provider.Model)
			if err != nil {
				return err
			}
			provider.Model = model
		}
	}
	if provider.WireAPI == "" {
		wireAPI, err := prompt(reader, writer, "Wire API", config.WireAPIResponses)
		if err != nil {
			return err
		}
		provider.WireAPI = wireAPI
	}
	if provider.APIKey == "" && !edit {
		apiKey, err := prompt(reader, writer, "API key", "")
		if err != nil {
			return err
		}
		provider.APIKey = apiKey
	}
	if options.timeout > 0 {
		provider.Timeout = options.timeout
	}
	provider.Normalize()
	if err := provider.Validate(); err != nil {
		return err
	}
	if provider.APIKey != "" {
		if err := (secret.KeyringStore{}).Set(provider.ID, provider.APIKey); err != nil {
			if os.Getenv("ZOLA_SECRET_BACKEND") == "keyring" {
				return fmt.Errorf("store API key in OS keychain: %w", err)
			}
			_, _ = fmt.Fprintf(writer, "Warning: OS keychain unavailable (%v); keeping API key in providers.json with 0600 permissions\n", err)
		} else {
			provider.APIKey = ""
		}
	}

	if edit {
		if err := store.Update(id, provider); err != nil {
			return err
		}
	} else {
		if err := store.Add(provider); err != nil {
			return err
		}
	}

	action := "Added"
	if edit {
		action = "Updated"
	}
	writef(cmd, "%s provider %s (%s)\n", action, provider.ID, provider.Name)
	return nil
}
