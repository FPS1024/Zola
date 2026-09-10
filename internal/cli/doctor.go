package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"zola/internal/codex"
	"zola/internal/config"
	"zola/internal/secret"
)

func newDoctorCommand() *cobra.Command {
	var colorMode string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Inspect the local Codex setup",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			w := cmd.OutOrStdout()
			switch colorMode {
			case "auto", "always", "never":
			default:
				return fmt.Errorf("--color must be auto, always, or never")
			}
			colored := shouldColor(w, colorMode)
			pass := func(format string, args ...any) {
				printDoctorLine(w, colored, colorGreen, format, args...)
			}
			warn := func(format string, args ...any) {
				printDoctorLine(w, colored, colorYellow, format, args...)
			}
			fail := func(format string, args ...any) {
				printDoctorLine(w, colored, colorRed, format, args...)
			}
			failures := []string{}

			pass("OS: %s", runtime.GOOS)
			pass("Architecture: %s", runtime.GOARCH)

			binaryPath, binaryErr := codex.BinaryPath()
			if binaryErr != nil {
				failures = append(failures, "codex is not installed or not available on PATH")
				fail("Codex installed: no")
				fail("Codex binary: not found")
				fail("Codex version: unavailable")
			} else {
				pass("Codex installed: yes")
				pass("Codex binary: %s", binaryPath)
				version, versionErr := codexVersion(binaryPath)
				if versionErr != nil {
					failures = append(failures, fmt.Sprintf("could not determine codex version: %v", versionErr))
					fail("Codex version: unavailable")
				} else {
					pass("Codex version: %s", version)
				}
			}

			manager, err := codex.NewManager()
			if err != nil {
				return err
			}
			pass("Codex config: %s", manager.ConfigPath)
			if manager.Exists() {
				pass("Codex config: found")
			} else {
				failures = append(failures, fmt.Sprintf("codex config not found at %s", manager.ConfigPath))
				fail("Codex config: not found")
			}

			keychainErr := secret.Check(secret.KeyringStore{})
			if keychainErr == nil {
				pass("OS keychain: available")
			} else {
				warn("OS keychain: unavailable; providers.json fallback may be used")
			}

			store, err := loadDefaultStore()
			if err != nil {
				return err
			}
			if len(store.Providers) == 0 {
				failures = append(failures, "no providers configured")
				fail("Provider: none")
			} else {
				state, err := config.LoadState()
				if err != nil {
					return err
				}
				if state.CurrentProvider == "" {
					failures = append(failures, "no provider selected")
					fail("Provider: none selected")
				} else if provider, ok := store.Find(state.CurrentProvider); ok {
					pass("Provider: %s (%s)", provider.ID, provider.Name)
					if provider.IsDisguised() {
						pass("Codex model: %s -> %s", provider.CodexModelName(), provider.Model)
					} else {
						pass("Codex model: %s", provider.Model)
					}
					if provider.ContextWindow > 0 {
						pass("Context window: %d", provider.ContextWindow)
					} else {
						pass("Context window: default")
					}
					if _, err := secret.Resolve(provider); err != nil {
						failures = append(failures, fmt.Sprintf("API key is not configured for provider %s", provider.ID))
						fail("API key: not configured")
					} else {
						pass("API key: configured")
					}
				} else {
					failures = append(failures, fmt.Sprintf("selected provider %q does not exist", state.CurrentProvider))
					fail("Provider: %s (missing)", state.CurrentProvider)
				}
			}

			if len(failures) > 0 {
				fail("\nFound issues:")
				for _, issue := range failures {
					fail("- %s", issue)
				}
				return fmt.Errorf("doctor found %d issue(s): %s", len(failures), strings.Join(failures, "; "))
			}
			pass("\nEverything looks good.")
			return nil
		},
	}
	cmd.Flags().StringVar(&colorMode, "color", "auto", "color output: auto, always, or never")
	return cmd
}

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
)

func printDoctorLine(w io.Writer, colored bool, color, format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	if colored {
		line = color + line + colorReset
	}
	_, _ = fmt.Fprintln(w, line)
}

func shouldColor(w io.Writer, mode string) bool {
	switch mode {
	case "always":
		return true
	case "never":
		return false
	}
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

var codexVersionPattern = regexp.MustCompile(`(?i)^codex(?:-cli)?\s+v?\S+`)

func codexVersion(binaryPath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	output, err := exec.CommandContext(ctx, binaryPath, "--version").CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("timed out")
	}
	text := strings.TrimSpace(string(output))
	if err != nil {
		return "", fmt.Errorf("command failed: %w", err)
	}
	return parseCodexVersion(text)
}

func parseCodexVersion(text string) (string, error) {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if codexVersionPattern.MatchString(line) {
			return line, nil
		}
	}
	return "", fmt.Errorf("unexpected version output")
}
