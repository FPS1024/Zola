package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"zola/internal/codex"
	"zola/internal/config"
)

func newUseCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "use <id>",
		Short: "Switch Codex to a provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			store, err := loadDefaultStore()
			if err != nil {
				return err
			}
			provider, ok := store.Find(id)
			if !ok {
				return fmt.Errorf("provider %q does not exist", id)
			}
			if !provider.Enabled {
				return fmt.Errorf("provider %q is disabled", id)
			}
			if provider.IsDisguised() {
				return fmt.Errorf("provider %q uses Codex model alias %q; use `zola proxy use %s` and start the proxy", provider.ID, provider.CodexModelName(), provider.ID)
			}
			manager, err := codex.NewManager()
			if err != nil {
				return err
			}
			if err := manager.Apply(provider); err != nil {
				return err
			}
			if err := config.SaveStateWithMode(id, config.ModeDirect, ""); err != nil {
				return err
			}
			writef(cmd, "Selected provider %s (%s)\n", provider.ID, provider.Name)
			writef(cmd, "Codex config: %s\n", manager.ConfigPath)
			writef(cmd, "Launch Codex with: zola run\n")
			return nil
		},
	}
}
