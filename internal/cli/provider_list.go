package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"zola/internal/config"
)

func newListCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List providers",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := loadDefaultStore()
			if err != nil {
				return err
			}
			state, err := config.LoadState()
			if err != nil {
				return err
			}
			if len(store.Providers) == 0 {
				writef(cmd, "No providers configured. Add one with `zola add deepseek --api-key <key>`.\n")
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			_, _ = w.Write([]byte("ID\tNAME\tMODEL\tBASE URL\tWIRE API\tSTATUS\n"))
			for _, provider := range store.Providers {
				status := ""
				if provider.ID == state.CurrentProvider {
					status = "current"
				} else if !provider.Enabled {
					status = "disabled"
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
					provider.ID,
					provider.Name,
					provider.Model,
					provider.BaseURL,
					provider.WireAPI,
					status,
				)
			}
			return w.Flush()
		},
	}
}
