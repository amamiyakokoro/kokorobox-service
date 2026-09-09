package processrouter

import (
	"os"
	"path/filepath"
)

const (
	FirewallRuleGroup = "KokoroBox Application Routing"
	TCPRelayPort      = 34010
	UDPRelayPort      = 34011
)

type firewallController interface {
	Ensure(string) error
	Check(string) error
	Remove() error
}

func DefaultBinaryPath() string {
	executable, _ := os.Executable()
	return filepath.Join(filepath.Dir(executable), "process-router", "kokorobox-process-router.exe")
}

func EnsureFirewallRules(binaryPath string) error {
	return newProcessRouterFirewall().Ensure(binaryPath)
}

func CheckFirewallRules(binaryPath string) error {
	return newProcessRouterFirewall().Check(binaryPath)
}

func RemoveFirewallRules() error {
	return newProcessRouterFirewall().Remove()
}
