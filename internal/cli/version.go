package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCommand(version, commit, buildTime string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Zola %s\n", version)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "commit: %s\n", commit)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "built: %s\n", buildTime)
			return nil
		},
	}
}
