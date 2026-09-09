package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"zola/internal/config"
	"zola/internal/secret"
)

func newRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <id>",
		Aliases: []string{"rm", "delete"},
		Short:   "Remove a provider",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			store, err := loadDefaultStore()
			if err != nil {
				return err
			}
			removed, ok := store.Remove(id)
			if !ok {
				return fmt.Errorf("provider %q does not exist", id)
			}
			if err := store.Save(); err != nil {
				return err
			}
			if removed.APIKey == "" {
				_ = (secret.KeyringStore{}).Delete(id)
			}
			state, err := config.LoadState()
			if err != nil {
				return err
			}
			if state.CurrentProvider == id {
				if err := config.SaveState(""); err != nil {
					return err
				}
			}
			writef(cmd, "Removed provider %s (%s)\n", removed.ID, removed.Name)
			return nil
		},
	}
}
