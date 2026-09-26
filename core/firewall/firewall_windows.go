//go:build windows

package firewall

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// Ensure receives only the executable resolved by the core manager, never an API-supplied path.
func Ensure(executable string) error {
	if !filepath.IsAbs(executable) || !strings.EqualFold(filepath.Ext(executable), ".exe") || strings.ContainsRune(executable, 0) {
		return fmt.Errorf("invalid core executable path for firewall rules")
	}
	info, err := os.Stat(executable)
	if err != nil {
		return fmt.Errorf("inspect core firewall executable: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("core firewall executable is not a regular file")
	}
	systemDirectory, err := windows.GetSystemDirectory()
	if err != nil {
		return fmt.Errorf("resolve Windows system directory: %w", err)
	}
	units := utf16.Encode([]rune(policyScript))
	encoded := make([]byte, len(units)*2)
	for i, unit := range units {
		encoded[i*2] = byte(unit)
		encoded[i*2+1] = byte(unit >> 8)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, filepath.Join(systemDirectory, "WindowsPowerShell", "v1.0", "powershell.exe"),
		"-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-EncodedCommand", base64.StdEncoding.EncodeToString(encoded))
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	// Replace rather than append to prevent inherited duplicate values from taking precedence.
	for _, entry := range os.Environ() {
		if !strings.EqualFold(strings.SplitN(entry, "=", 2)[0], "KOKOROBOX_CORE_FIREWALL_PATH") {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, "KOKOROBOX_CORE_FIREWALL_PATH="+executable)
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		return fmt.Errorf("timed out while configuring core firewall rules")
	}
	if err != nil {
		message := strings.TrimSpace(string(output))
		if len(message) > 2000 {
			message = message[:2000]
		}
		return fmt.Errorf("configure core firewall rules: %w: %s", err, message)
	}
	return nil
}
