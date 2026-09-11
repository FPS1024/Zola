package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"zola/internal/codex"
	"zola/internal/config"
	localproxy "zola/internal/proxy"
	"zola/internal/secret"
)

func newProxyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "proxy",
		Short: "Run the optional local Codex proxy",
	}
	cmd.AddCommand(
		newProxyStartCommand(),
		newProxyStatusCommand(),
		newProxyUseCommand(),
		newProxyDirectCommand(),
	)
	return cmd
}

func newProxyStartCommand() *cobra.Command {
	var host string
	var port int
	var providerID string
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the local proxy in the foreground",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if providerID == "" {
				providerID = os.Getenv("ZOLA_PROVIDER")
			}
			provider, err := providerForOptionalID(providerID)
			if err != nil {
				return err
			}
			apiKey, keyErr := secret.Resolve(provider)
			if keyErr != nil {
				if provider.APIKey == "" && os.Getenv(provider.CodexEnvKey()) == "" {
					_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Warning: API key not found; requests will be forwarded without Authorization")
				} else {
					return keyErr
				}
			}

			addr := net.JoinHostPort(host, strconv.Itoa(port))
			logger := log.New(os.Stderr, "zola-proxy ", log.LstdFlags)
			handler := localproxy.NewHandler(localproxy.HandlerOptions{
				Provider: provider,
				APIKey:   apiKey,
				Logger:   logger,
			})
			server := &http.Server{
				Addr:              addr,
				Handler:           handler,
				ReadHeaderTimeout: 10 * time.Second,
			}

			writef(cmd, "Zola proxy listening on http://%s\n", addr)
			writef(cmd, "Provider: %s (%s)\n", provider.ID, provider.Model)
			writef(cmd, "Press Ctrl+C to stop.\n")

			signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()
			errCh := make(chan error, 1)
			go func() {
				errCh <- server.ListenAndServe()
			}()

			select {
			case err := <-errCh:
				if err == http.ErrServerClosed {
					return nil
				}
				return err
			case <-signalCtx.Done():
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := server.Shutdown(ctx); err != nil {
					return err
				}
				writef(cmd, "Zola proxy stopped.\n")
				return nil
			}
		},
	}
	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "listen host")
	cmd.Flags().IntVar(&port, "port", 8317, "listen port")
	cmd.Flags().StringVar(&providerID, "provider", "", "provider id (default: current provider)")
	return cmd
}

func newProxyStatusCommand() *cobra.Command {
	var host string
	var port int
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check whether the local proxy is running",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			addr := net.JoinHostPort(host, strconv.Itoa(port))
			health, err := fetchProxyHealth(cmd.Context(), addr)
			if err != nil {
				return err
			}
			writef(cmd, "Proxy is running at http://%s\n", addr)
			if health.Provider != "" {
				writef(cmd, "Provider: %s\n", health.Provider)
			}
			if health.CodexModel != "" {
				if health.UpstreamModel != "" && health.UpstreamModel != health.CodexModel {
					writef(cmd, "Model: %s -> %s\n", health.CodexModel, health.UpstreamModel)
				} else {
					writef(cmd, "Model: %s\n", health.CodexModel)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "proxy host")
	cmd.Flags().IntVar(&port, "port", 8317, "proxy port")
	return cmd
}

type proxyHealth struct {
	Provider      string
	CodexModel    string
	UpstreamModel string
}

func fetchProxyHealth(ctx context.Context, addr string) (proxyHealth, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/health", nil)
	if err != nil {
		return proxyHealth{}, fmt.Errorf("build proxy health request: %w", err)
	}
	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return proxyHealth{}, fmt.Errorf("proxy is not running at http://%s: %w", addr, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return proxyHealth{}, fmt.Errorf("proxy health returned HTTP %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return proxyHealth{}, fmt.Errorf("parse proxy health response: %w", err)
	}
	health := proxyHealth{}
	if provider, ok := body["provider"].(string); ok {
		health.Provider = provider
	}
	if model, ok := body["codex_model"].(string); ok {
		health.CodexModel = model
	}
	if model, ok := body["upstream_model"].(string); ok {
		health.UpstreamModel = model
	}
	return health, nil
}

func newProxyUseCommand() *cobra.Command {
	var host string
	var port int
	var providerID string
	cmd := &cobra.Command{
		Use:   "use [id]",
		Short: "Point Codex at the local proxy",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider, err := providerForOptionalIDAndArgs(providerID, args)
			if err != nil {
				return err
			}
			proxyURL := "http://" + net.JoinHostPort(host, strconv.Itoa(port)) + "/v1"
			manager, err := codex.NewManager()
			if err != nil {
				return err
			}
			if err := manager.ApplyProxy(provider, proxyURL); err != nil {
				return err
			}
			if err := config.SaveStateWithMode(provider.ID, config.ModeProxy, proxyURL); err != nil {
				return err
			}
			writef(cmd, "Codex is configured to use provider %s through %s\n", provider.ID, proxyURL)
			if runtime.GOOS == "ios" {
				writef(cmd, "Restart the background proxy with: sudo zola service restart\n")
			} else {
				writef(cmd, "Start the proxy with: zola proxy start\n")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "proxy host")
	cmd.Flags().IntVar(&port, "port", 8317, "proxy port")
	cmd.Flags().StringVar(&providerID, "provider", "", "provider id")
	return cmd
}

func newProxyDirectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "direct",
		Short: "Point Codex directly at the current provider",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			provider, err := loadCurrentProvider()
			if err != nil {
				return err
			}
			manager, err := codex.NewManager()
			if err != nil {
				return err
			}
			if err := manager.Apply(provider); err != nil {
				return err
			}
			if err := config.SaveStateWithMode(provider.ID, config.ModeDirect, ""); err != nil {
				return err
			}
			writef(cmd, "Codex is configured to use provider %s directly\n", provider.ID)
			return nil
		},
	}
	return cmd
}

func providerForOptionalID(providerID string) (config.Provider, error) {
	return providerForOptionalIDAndArgs(providerID, nil)
}

func providerForOptionalIDAndArgs(providerID string, args []string) (config.Provider, error) {
	id := providerID
	if id == "" && len(args) == 1 {
		id = args[0]
	}
	store, err := loadDefaultStore()
	if err != nil {
		return config.Provider{}, err
	}
	if id == "" {
		state, err := config.LoadState()
		if err != nil {
			return config.Provider{}, err
		}
		if state.CurrentProvider != "" {
			provider, ok := store.Find(state.CurrentProvider)
			if !ok {
				return config.Provider{}, fmt.Errorf("current provider %q does not exist", state.CurrentProvider)
			}
			return provider, nil
		}
		if len(store.Providers) == 1 {
			return store.Providers[0], nil
		}
		return config.Provider{}, fmt.Errorf("no current provider is selected")
	}
	provider, ok := store.Find(id)
	if !ok {
		return config.Provider{}, fmt.Errorf("provider %q does not exist", id)
	}
	return provider, nil
}
