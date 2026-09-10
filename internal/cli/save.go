package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"zola/internal/codex"
	"zola/internal/config"
	"zola/internal/secret"
)

func newSaveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "save [id]",
		Short: "Persist provider access so codex can run directly",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider, err := providerForOptionalIDAndArgs("", args)
			if err != nil {
				return err
			}
			if provider.IsDisguised() {
				return fmt.Errorf("provider %q uses Codex model alias %q; use proxy mode instead", provider.ID, provider.CodexModelName())
			}
			apiKey, err := secret.Resolve(provider)
			if err != nil {
				return err
			}
			manager, err := codex.NewManager()
			if err != nil {
				return err
			}
			if err := manager.ApplyBearerToken(provider, apiKey); err != nil {
				return err
			}
			if err := config.SaveStateWithMode(provider.ID, config.ModeDirect, ""); err != nil {
				return err
			}
			writef(cmd, "Persisted provider %s into Codex config.\n", provider.ID)
			writef(cmd, "Config: %s\n", manager.ConfigPath)
			writef(cmd, "You can now run `codex` directly.\n")
			return nil
		},
	}
}
