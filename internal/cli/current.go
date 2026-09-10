package cli

import (
	"github.com/spf13/cobra"
)

func newCurrentCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Show the active provider",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			provider, err := loadCurrentProvider()
			if err != nil {
				return err
			}
			writef(cmd, "ID:      %s\n", provider.ID)
			writef(cmd, "Name:    %s\n", provider.Name)
			if provider.IsDisguised() {
				writef(cmd, "Model:   %s -> %s\n", provider.CodexModelName(), provider.Model)
			} else {
				writef(cmd, "Model:   %s\n", provider.Model)
			}
			if provider.ContextWindow > 0 {
				writef(cmd, "Context: %d\n", provider.ContextWindow)
			} else {
				writef(cmd, "Context: default\n")
			}
			writef(cmd, "Base URL: %s\n", provider.BaseURL)
			writef(cmd, "Wire API: %s\n", provider.WireAPI)
			return nil
		},
	}
}
