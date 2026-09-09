package codex

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"zola/internal/config"
)

func BinaryPath() (string, error) {
	if override := os.Getenv("CODEX_BINARY"); override != "" {
		return override, nil
	}
	path, err := exec.LookPath("codex")
	if err != nil {
		return "", fmt.Errorf("codex binary not found in PATH; install it or set CODEX_BINARY: %w", err)
	}
	return path, nil
}

func (m *Manager) Launch(provider config.Provider, args []string) error {
	return m.LaunchWithKey(provider, provider.APIKey, args)
}

func (m *Manager) LaunchWithKey(provider config.Provider, apiKey string, args []string) error {
	binary, err := BinaryPath()
	if err != nil {
		return err
	}
	cmd := exec.Command(binary, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if apiKey != "" {
		cmd.Env = replaceEnv(os.Environ(), provider.CodexEnvKey(), apiKey)
	}
	return cmd.Run()
}

func replaceEnv(env []string, key, value string) []string {
	prefix := key + "="
	found := false
	for i, item := range env {
		if strings.HasPrefix(item, prefix) {
			env[i] = prefix + value
			found = true
		}
	}
	if !found {
		env = append(env, prefix+value)
	}
	return env
}
