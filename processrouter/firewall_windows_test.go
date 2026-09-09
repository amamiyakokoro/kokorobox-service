//go:build windows

package processrouter

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFirewallPowerShellSyntax(t *testing.T) {
	powerShell := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	command := exec.Command(powerShell, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", "[scriptblock]::Create($env:KOKOROBOX_FIREWALL_SCRIPT) | Out-Null")
	command.Env = append(os.Environ(), "KOKOROBOX_FIREWALL_SCRIPT="+firewallScript)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("invalid firewall PowerShell: %v: %s", err, strings.TrimSpace(string(output)))
	}
}

func TestFirewallPolicyIsScopedToTheRouterAndRelayPorts(t *testing.T) {
	expected := []string{
		"KokoroBox Application Routing",
		"-Direction Inbound",
		"-Action Allow",
		"-Profile Any",
		"-Program $program",
		"Protocol = 'TCP'; Port = '34010'",
		"Protocol = 'UDP'; Port = '34011'",
		"-RemoteAddress Any",
	}
	for _, value := range expected {
		if !strings.Contains(firewallScript, value) {
			t.Fatalf("firewall policy is missing %q", value)
		}
	}
}

func TestFirewallScriptSupportsIdempotentRepairAndRemoval(t *testing.T) {
	for _, value := range []string{"Test-KokoroBoxFirewallRule", "Remove-NetFirewallRule", "New-NetFirewallRule", "mode -eq 'remove'", "mode -eq 'ensure'"} {
		if !strings.Contains(firewallScript, value) {
			t.Fatalf("firewall lifecycle is missing %q", value)
		}
	}
}
