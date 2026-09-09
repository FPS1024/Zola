package cli

import (
	"os"

	"github.com/spf13/cobra"

	"zola/internal/codex"
	"zola/internal/config"
	"zola/internal/secret"
)

func newRunCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "run [codex args...]",
		Short: "Launch Codex with the active provider",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			provider, err := loadCurrentProvider()
			if err != nil {
				return err
			}
			state, err := config.LoadState()
			if err != nil {
				return err
			}
			manager, err := codex.NewManager()
			if err != nil {
				return err
			}
			apiKey := ""
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
			if _, err := codex.BinaryPath(); err != nil {
				return err
			}
			if os.Getenv("CODEX_BINARY") != "" {
				writef(cmd, "Launching %s for provider %s\n", os.Getenv("CODEX_BINARY"), provider.ID)
			} else {
				writef(cmd, "Launching codex for provider %s\n", provider.ID)
			}
			return manager.LaunchWithKey(provider, apiKey, args)
		},
	}
}
