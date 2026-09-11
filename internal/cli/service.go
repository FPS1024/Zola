package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	proxyServiceLabel = "com.fps1024.zola.proxy"
	proxyAddress      = "127.0.0.1:8317"
)

type launchdService struct {
	launchctl string
	plist     string
}

func newServiceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Inspect and control the installed launchd service",
	}
	cmd.AddCommand(
		newServiceStatusCommand(),
		newServiceRestartCommand(),
	)
	return cmd
}

func newServiceStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check the launchd service and proxy health",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := findLaunchdService()
			if err != nil {
				return err
			}

			writef(cmd, "Launchd label: %s\n", proxyServiceLabel)
			writef(cmd, "Launchctl: %s\n", service.launchctl)
			writef(cmd, "Plist: %s\n", service.plist)

			launchdErr := runLaunchctl(service.launchctl, "print", "system/"+proxyServiceLabel)
			if launchdErr == nil {
				writef(cmd, "Launchd: loaded\n")
			} else {
				writef(cmd, "Launchd: not loaded\n")
			}

			health, healthErr := fetchProxyHealth(context.Background(), proxyAddress)
			if healthErr == nil {
				writef(cmd, "Proxy health: ok at http://%s\n", proxyAddress)
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
			} else {
				writef(cmd, "Proxy health: failed: %v\n", healthErr)
			}

			if launchdErr != nil || healthErr != nil {
				return fmt.Errorf("zola proxy service is not healthy; inspect %s", serviceLogHint())
			}
			return nil
		},
	}
}

func newServiceRestartCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart the installed launchd service",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if os.Geteuid() != 0 {
				return errors.New("run `zola service restart` as root")
			}
			service, err := findLaunchdService()
			if err != nil {
				return err
			}

			_ = runLaunchctl(service.launchctl, "bootout", "system/"+proxyServiceLabel)
			if err := runLaunchctl(service.launchctl, "bootstrap", "system", service.plist); err != nil {
				if loadErr := runLaunchctl(service.launchctl, "load", "-w", service.plist); loadErr != nil {
					return fmt.Errorf("bootstrap service: %w", err)
				}
			}
			if err := runLaunchctl(service.launchctl, "kickstart", "-k", "system/"+proxyServiceLabel); err != nil {
				if startErr := runLaunchctl(service.launchctl, "start", proxyServiceLabel); startErr != nil {
					return fmt.Errorf("start service: %w", err)
				}
			}

			writef(cmd, "Restarted %s\n", proxyServiceLabel)
			return nil
		},
	}
}

func findLaunchdService() (launchdService, error) {
	launchctlPaths := []string{
		os.Getenv("ZOLA_LAUNCHCTL"),
		"/var/jb/usr/bin/launchctl",
		"/bin/launchctl",
		"/usr/bin/launchctl",
	}
	plistPaths := []string{
		os.Getenv("ZOLA_LAUNCHD_PLIST"),
		"/var/jb/Library/LaunchDaemons/" + proxyServiceLabel + ".plist",
		"/Library/LaunchDaemons/" + proxyServiceLabel + ".plist",
	}

	var service launchdService
	for _, path := range launchctlPaths {
		if path != "" && isExecutable(path) {
			service.launchctl = path
			break
		}
	}
	for _, path := range plistPaths {
		if path != "" && isRegularFile(path) {
			service.plist = path
			break
		}
	}
	if service.launchctl == "" || service.plist == "" {
		return launchdService{}, errors.New("zola launchd service is not installed; install the iOS deb or set ZOLA_LAUNCHCTL and ZOLA_LAUNCHD_PLIST")
	}
	return service, nil
}

func runLaunchctl(launchctl string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	output, err := exec.CommandContext(ctx, launchctl, args...).CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return errors.New("launchctl timed out")
	}
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, firstLines(message, 3))
	}
	return nil
}

func firstLines(text string, limit int) string {
	lines := strings.Split(text, "\n")
	if len(lines) > limit {
		lines = lines[:limit]
	}
	return strings.Join(lines, " | ")
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func serviceLogHint() string {
	for _, path := range []string{
		"/var/mobile/Library/Logs/zola-proxy.log",
		"/var/root/Library/Logs/zola-proxy.log",
	} {
		if isRegularFile(path) {
			return path
		}
	}
	return "the zola-proxy.log file or `launchctl print system/" + proxyServiceLabel + "`"
}
