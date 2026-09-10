package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"zola/internal/tui"
)

func Execute(version, commit, buildTime string) error {
	root := &cobra.Command{
		Use:           "zola",
		Short:         "Zola manages Codex model providers",
		Long:          "Zola is a terminal provider manager for OpenAI Codex.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	root.RunE = func(_ *cobra.Command, _ []string) error {
		return tui.Run(version)
	}

	root.AddCommand(
		newAddCommand(),
		newEditCommand(),
		newRemoveCommand(),
		newListCommand(),
		newUseCommand(),
		newSaveCommand(),
		newCurrentCommand(),
		newRunCommand(),
		newDoctorCommand(),
		newProxyCommand(),
		newTestCommand(version),
		newTUICommand(version),
		newVersionCommand(version, commit, buildTime),
	)

	return root.Execute()
}

func newTUICommand(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Open the terminal UI",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return tui.Run(version)
		},
	}
}

func PrintError(w io.Writer, err error) {
	line := fmt.Sprintf("Error: %v", err)
	if shouldColor(w, "auto") {
		line = colorRed + line + colorReset
	}
	_, _ = fmt.Fprintln(w, line)
}
