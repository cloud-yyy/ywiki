package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// ConfigDir returns the directory where ywiki stores its configuration.
// Resolution order: YWIKI_CONFIG_DIR env var > platform default.
// On Windows, uses %APPDATA%\ywiki. On Unix, uses XDG_CONFIG_HOME/ywiki
// or falls back to ~/.config/ywiki (following the gh CLI pattern, not
// os.UserConfigDir which returns ~/Library/ on macOS).
func ConfigDir() (string, error) {
	if dir := os.Getenv("YWIKI_CONFIG_DIR"); dir != "" {
		return dir, nil
	}

	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "ywiki"), nil
		}
	}

	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "ywiki"), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine config directory: %w", err)
	}

	return filepath.Join(homeDir, ".config", "ywiki"), nil
}

// ConfigFilePath returns the full path to the ywiki config file.
func ConfigFilePath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "config.yaml"), nil
}
