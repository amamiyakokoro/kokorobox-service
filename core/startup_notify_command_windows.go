//go:build windows

package core

import (
	"fmt"
	"os"
)

const startupNotifyExecutableEnv = "KOKOROBOX_CORE_STARTUP_NOTIFY_EXECUTABLE"

func startupNotifyCommand() (string, string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", "", fmt.Errorf("Failed to read service executable path: %w", err)
	}
	return "%" + startupNotifyExecutableEnv + "%", `"` + executable + `"`, nil
}
