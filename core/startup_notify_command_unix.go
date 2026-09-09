//go:build !windows

package core

import (
	"fmt"
	"os"
	"strings"
)

func startupNotifyCommand() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("Failed to read service executable path: %w", err)
	}
	return shellQuote(executable), nil
}

func shellQuote(value string) string {
	return `'` + strings.ReplaceAll(value, `'`, `'\''`) + `'`
}
