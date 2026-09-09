package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"zola/internal/codex"
	"zola/internal/config"
	"zola/internal/secret"
)

func newDoctorCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Inspect the local Codex setup",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			w := cmd.OutOrStdout()
			failures := []string{}

			_, _ = fmt.Fprintf(w, "OS: %s\n", runtime.GOOS)
			_, _ = fmt.Fprintf(w, "Architecture: %s\n", runtime.GOARCH)

			binary := "not found"
			if _, err := exec.LookPath("codex"); err == nil {
				binary = "found"
			} else if os.Getenv("CODEX_BINARY") == "" {
				failures = append(failures, "codex binary was not found in PATH")
			} else {
				binary = "configured via CODEX_BINARY"
			}
			_, _ = fmt.Fprintf(w, "Codex binary: %s\n", binary)

			manager, err := codex.NewManager()
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(w, "Codex config: %s\n", manager.ConfigPath)
			if manager.Exists() {
				_, _ = fmt.Fprintln(w, "Codex config: found")
			} else {
				failures = append(failures, fmt.Sprintf("codex config not found at %s", manager.ConfigPath))
			}

			keychainErr := secret.Check(secret.KeyringStore{})
			if keychainErr == nil {
				_, _ = fmt.Fprintln(w, "OS keychain: available")
			} else {
				_, _ = fmt.Fprintln(w, "OS keychain: unavailable; providers.json fallback may be used")
			}

			store, err := loadDefaultStore()
			if err != nil {
				return err
			}
			if len(store.Providers) == 0 {
				failures = append(failures, "no providers configured")
				_, _ = fmt.Fprintln(w, "Provider: none")
			} else {
				state, err := config.LoadState()
				if err != nil {
					return err
				}
				if state.CurrentProvider == "" {
					failures = append(failures, "no provider selected")
					_, _ = fmt.Fprintln(w, "Provider: none selected")
				} else if provider, ok := store.Find(state.CurrentProvider); ok {
					_, _ = fmt.Fprintf(w, "Provider: %s (%s)\n", provider.ID, provider.Name)
					if _, err := secret.Resolve(provider); err != nil {
						failures = append(failures, fmt.Sprintf("API key is not configured for provider %s", provider.ID))
						_, _ = fmt.Fprintln(w, "API key: not configured")
					} else {
						_, _ = fmt.Fprintln(w, "API key: configured")
					}
				} else {
					failures = append(failures, fmt.Sprintf("selected provider %q does not exist", state.CurrentProvider))
				}
			}

			if len(failures) > 0 {
				_, _ = fmt.Fprintln(w, "\nFound issues:")
				for _, issue := range failures {
					_, _ = fmt.Fprintf(w, "- %s\n", issue)
				}
				return fmt.Errorf("doctor found %d issue(s): %s", len(failures), strings.Join(failures, "; "))
			}
			_, _ = fmt.Fprintln(w, "\nEverything looks good.")
			return nil
		},
	}
}
