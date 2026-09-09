package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"zola/internal/config"
	"zola/internal/secret"
)

func loadDefaultStore() (*config.Store, error) {
	path, err := config.ProvidersPath()
	if err != nil {
		return nil, err
	}
	return config.LoadStore(path)
}

func loadCurrentProvider() (config.Provider, error) {
	state, err := config.LoadState()
	if err != nil {
		return config.Provider{}, err
	}
	if state.CurrentProvider == "" {
		return config.Provider{}, fmt.Errorf("no provider is selected; run `zola use <id>` first")
	}
	store, err := loadDefaultStore()
	if err != nil {
		return config.Provider{}, err
	}
	provider, ok := store.Find(state.CurrentProvider)
	if !ok {
		return config.Provider{}, fmt.Errorf("selected provider %q no longer exists", state.CurrentProvider)
	}
	return provider, nil
}

func apiKeyFor(provider config.Provider) (string, error) {
	return secret.Resolve(provider)
}

func writef(cmd *cobra.Command, format string, args ...any) {
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), format, args...)
}

func prompt(reader *bufio.Reader, writer io.Writer, label, defaultValue string) (string, error) {
	if defaultValue == "" {
		_, _ = fmt.Fprintf(writer, "%s: ", label)
	} else {
		_, _ = fmt.Fprintf(writer, "%s [%s]: ", label, defaultValue)
	}
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		if err == io.EOF {
			return "", fmt.Errorf("%s is required", label)
		}
		return "", err
	}
	value := strings.TrimSpace(line)
	if value == "" {
		value = defaultValue
	}
	return value, nil
}
