package identity

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	ProductName         = "KokoroBox"
	ServiceName         = "KokoroBoxService"
	ServiceDisplayName  = "KokoroBox Service"
	ServiceDescription  = "KokoroBox privileged service"
	ServiceExecutable   = "kokorobox-service"
	ConfigDirectoryName = "KokoroBox"

	WindowsServicePipe = `\\.\pipe\kokorobox\service`
	UnixServiceSocket  = "/tmp/kokorobox-service.sock"

	ConfigDirectoryEnv       = "KOKOROBOX_CONFIG_DIR"
	LegacyConfigDirectoryEnv = "SPARKLE_CONFIG_DIR"
)

// ConfigDirectoryOverride gives new deployments priority while retaining the
// old environment variable as an upgrade-only compatibility input.
func ConfigDirectoryOverride() string {
	if value := os.Getenv(ConfigDirectoryEnv); value != "" {
		return value
	}
	return os.Getenv(LegacyConfigDirectoryEnv)
}

// Environment returns the current environment value first and the legacy one
// only when the current variable is not configured.
func Environment(currentName, legacyName string) string {
	if value := os.Getenv(currentName); value != "" {
		return value
	}
	return os.Getenv(legacyName)
}

// DataDirectory returns the current service data directory.  During an
// upgrade it atomically moves the complete legacy directory only when the
// current directory does not already exist, so an existing KokoroBox install
// is never merged with or overwritten by old state.
func DataDirectory(configRoot string) (string, error) {
	current := filepath.Join(configRoot, ConfigDirectoryName)
	if _, err := os.Lstat(current); err == nil {
		return current, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect KokoroBox service data directory: %w", err)
	}

	legacy := filepath.Join(configRoot, "sparkle")
	if _, err := os.Lstat(legacy); err != nil {
		if os.IsNotExist(err) {
			return current, nil
		}
		return "", fmt.Errorf("inspect legacy service data directory: %w", err)
	}

	if err := os.Rename(legacy, current); err != nil {
		return "", fmt.Errorf("migrate legacy service data directory: %w", err)
	}
	return current, nil
}
