package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"zola/internal/api"
	"zola/internal/config"
)

func newTestCommand(version string) *cobra.Command {
	var modelOverride string
	cmd := &cobra.Command{
		Use:   "test [id]",
		Short: "Test a provider through the Responses API",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var provider config.Provider
			var err error
			if len(args) == 1 {
				store, loadErr := loadDefaultStore()
				if loadErr != nil {
					return loadErr
				}
				provider, err = requireProvider(store, args[0])
			} else {
				provider, err = loadCurrentProvider()
			}
			if err != nil {
				return err
			}
			if provider.WireAPI != config.WireAPIResponses {
				return fmt.Errorf("test currently requires wire_api=responses; provider %q uses %q", provider.ID, provider.WireAPI)
			}
			apiKey, err := apiKeyFor(provider)
			if err != nil {
				return err
			}
			if modelOverride != "" {
				provider.Model = modelOverride
			}

			timeout := time.Duration(provider.Timeout) * time.Second
			client, err := api.NewProbeClient(api.ProbeOptions{
				Model:     provider.Model,
				BaseURL:   provider.BaseURL,
				APIKey:    apiKey,
				Timeout:   timeout,
				UserAgent: "zola/" + version,
			})
			if err != nil {
				return err
			}

			writef(cmd, "Testing provider %s (%s)\n", provider.ID, provider.Name)
			result, err := client.Probe(context.Background())
			if err != nil {
				return fmt.Errorf("provider %s failed: %w", provider.ID, err)
			}
			writef(cmd, "Endpoint: POST %s\n", result.Endpoint)
			writef(cmd, "OK HTTP %d\n", result.HTTPStatus)
			writef(cmd, "OK Responses API status=%s\n", result.ResponseStatus)
			writef(cmd, "OK model=%s\n", provider.Model)
			if result.ResponseModel != "" && result.ResponseModel != provider.Model {
				writef(cmd, "Note: provider returned model=%s\n", result.ResponseModel)
			}
			writef(cmd, "Completed in %s\n", strings.TrimSpace(result.Duration.Round(time.Millisecond).String()))
			return nil
		},
	}
	cmd.Flags().StringVar(&modelOverride, "model", "", "model to test instead of the provider default")
	return cmd
}

func requireProvider(store *config.Store, id string) (config.Provider, error) {
	provider, ok := store.Find(id)
	if !ok {
		return config.Provider{}, fmt.Errorf("provider %q does not exist", id)
	}
	return provider, nil
}
