package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	envConfigDir   = "ZOLA_CONFIG_DIR"
	envXdgConfig   = "XDG_CONFIG_HOME"
	defaultDirName = "zola"
)

// ConfigDir returns the directory where Zola stores providers and state.
func ConfigDir() (string, error) {
	if override := os.Getenv(envConfigDir); override != "" {
		dir, err := filepath.Abs(override)
		if err != nil {
			return "", fmt.Errorf("resolve ZOLA_CONFIG_DIR: %w", err)
		}
		return dir, nil
	}
	if xdg := os.Getenv(envXdgConfig); xdg != "" {
		return filepath.Join(xdg, defaultDirName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", defaultDirName), nil
}

func ProvidersPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "providers.json"), nil
}

func StatePath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "state.json"), nil
}

func EnsureConfigDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create config directory %s: %w", dir, err)
	}
	return dir, nil
}
