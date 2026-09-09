//go:build windows

package processrouter

import (
	"context"
	"encoding/base64"
	"errors"
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

const firewallCommandTimeout = 15 * time.Second

const firewallScript = `$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
Set-StrictMode -Version Latest
trap {
  [Console]::Error.WriteLine($_.Exception.Message)
  exit 1
}
$group = 'KokoroBox Application Routing'
$mode = $env:KOKOROBOX_FIREWALL_MODE
$program = $env:KOKOROBOX_PROCESS_ROUTER_PATH
$validModes = @('ensure', 'check', 'remove')
if ($mode -notin $validModes) { throw 'Invalid firewall operation' }
$specs = @(
  @{ Name = 'KokoroBox Application Routing TCP Relay'; Protocol = 'TCP'; Port = '34010' },
  @{ Name = 'KokoroBox Application Routing UDP Relay'; Protocol = 'UDP'; Port = '34011' }
)

function Test-KokoroBoxFirewallRule($spec) {
  $rules = @(Get-NetFirewallRule -PolicyStore ActiveStore -DisplayName $spec.Name -ErrorAction SilentlyContinue)
  if ($rules.Count -ne 1) { return $false }
  $rule = $rules[0]
  $profile = [string]$rule.Profile
  $allProfiles = $profile -eq 'Any' -or ($profile.Contains('Domain') -and $profile.Contains('Private') -and $profile.Contains('Public'))
  if ([string]$rule.Group -ne $group -or [string]$rule.Enabled -ne 'True' -or
      [string]$rule.Direction -ne 'Inbound' -or [string]$rule.Action -ne 'Allow' -or
      -not $allProfiles) { return $false }
  $applications = @($rule | Get-NetFirewallApplicationFilter)
  $ports = @($rule | Get-NetFirewallPortFilter)
  $addresses = @($rule | Get-NetFirewallAddressFilter)
  if ($applications.Count -ne 1 -or $ports.Count -ne 1 -or $addresses.Count -ne 1) { return $false }
  if ([string]$applications[0].Program -ine $program -or
      [string]$ports[0].Protocol -ine $spec.Protocol -or
      [string]$ports[0].LocalPort -ne $spec.Port) { return $false }
  $remoteAddresses = @($addresses[0].RemoteAddress)
  return $remoteAddresses.Count -eq 1 -and [string]$remoteAddresses[0] -eq 'Any'
}

if ($mode -eq 'remove') {
  Get-NetFirewallRule -Group $group -ErrorAction SilentlyContinue | Remove-NetFirewallRule
  foreach ($spec in $specs) {
    Get-NetFirewallRule -DisplayName $spec.Name -ErrorAction SilentlyContinue | Remove-NetFirewallRule
  }
  exit 0
}

if ([string]::IsNullOrWhiteSpace($program)) { throw 'Missing process router path' }
$program = [IO.Path]::GetFullPath($program)
$healthy = $true
foreach ($spec in $specs) {
  if (-not (Test-KokoroBoxFirewallRule $spec)) { $healthy = $false }
}

if ($mode -eq 'ensure' -and -not $healthy) {
  Get-NetFirewallRule -Group $group -ErrorAction SilentlyContinue | Remove-NetFirewallRule
  foreach ($spec in $specs) {
    Get-NetFirewallRule -DisplayName $spec.Name -ErrorAction SilentlyContinue | Remove-NetFirewallRule
  }
  foreach ($spec in $specs) {
    New-NetFirewallRule -DisplayName $spec.Name -Group $group -Direction Inbound -Action Allow -Enabled True -Profile Any -Program $program -Protocol $spec.Protocol -LocalPort $spec.Port -RemoteAddress Any | Out-Null
  }
  $healthy = $true
  foreach ($spec in $specs) {
    if (-not (Test-KokoroBoxFirewallRule $spec)) { $healthy = $false }
  }
}

if (-not $healthy) { throw 'KokoroBox application-routing firewall rules are missing or invalid' }
`

type windowsFirewall struct{}

func newProcessRouterFirewall() firewallController { return windowsFirewall{} }

func (windowsFirewall) Ensure(binaryPath string) error {
	if err := validateFirewallBinaryPath(binaryPath); err != nil {
		return err
	}
	return runFirewallCommand("ensure", binaryPath)
}

func (windowsFirewall) Check(binaryPath string) error {
	if err := validateFirewallBinaryPath(binaryPath); err != nil {
		return err
	}
	return runFirewallCommand("check", binaryPath)
}

func (windowsFirewall) Remove() error {
	return runFirewallCommand("remove", "")
}

func validateFirewallBinaryPath(binaryPath string) error {
	if !filepath.IsAbs(binaryPath) || !strings.EqualFold(filepath.Base(binaryPath), "kokorobox-process-router.exe") {
		return errors.New("invalid process router path for firewall rule")
	}
	info, err := os.Stat(binaryPath)
	if err != nil {
		return fmt.Errorf("stat process router for firewall rule: %w", err)
	}
	if info.IsDir() {
		return errors.New("process router path is a directory")
	}
	return nil
}

func runFirewallCommand(mode, binaryPath string) error {
	encoded := encodePowerShell(firewallScript)
	powerShell := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	ctx, cancel := context.WithTimeout(context.Background(), firewallCommandTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, powerShell,
		"-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-OutputFormat", "Text", "-EncodedCommand", encoded,
	)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	command.Env = append(os.Environ(),
		"KOKOROBOX_FIREWALL_MODE="+mode,
		"KOKOROBOX_PROCESS_ROUTER_PATH="+binaryPath,
	)
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		return errors.New("timed out while managing application-routing firewall rules")
	}
	if err != nil {
		message := strings.TrimSpace(string(output))
		if len(message) > 1000 {
			message = message[:1000]
		}
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("manage application-routing firewall rules: %s", message)
	}
	return nil
}

func encodePowerShell(script string) string {
	encoded := utf16.Encode([]rune(script))
	buffer := make([]byte, len(encoded)*2)
	for index, value := range encoded {
		buffer[index*2] = byte(value)
		buffer[index*2+1] = byte(value >> 8)
	}
	return base64.StdEncoding.EncodeToString(buffer)
}
