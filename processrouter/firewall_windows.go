//go:build windows

package processrouter

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/amamiyakokoro/kokorobox-service/internal/winfirewall"
)

type windowsFirewall struct{}

func newProcessRouterFirewall() firewallController { return windowsFirewall{} }

func routerFirewallRules(binaryPath string) []winfirewall.Rule {
	return []winfirewall.Rule{
		winfirewall.Inbound("KokoroBox Application Routing TCP Relay", FirewallRuleGroup, binaryPath, 6, strconv.Itoa(TCPRelayPort)),
		winfirewall.Inbound("KokoroBox Application Routing UDP Relay", FirewallRuleGroup, binaryPath, 17, strconv.Itoa(UDPRelayPort)),
	}
}

func (windowsFirewall) Ensure(binaryPath string) error {
	if err := validateFirewallBinaryPath(binaryPath); err != nil {
		return err
	}
	return applyFirewall(winfirewall.Ensure, binaryPath)
}

func (windowsFirewall) Check(binaryPath string) error {
	if err := validateFirewallBinaryPath(binaryPath); err != nil {
		return err
	}
	return applyFirewall(winfirewall.Check, binaryPath)
}

func (windowsFirewall) Remove() error { return applyFirewall(winfirewall.Remove, "") }

func applyFirewall(mode winfirewall.Mode, binaryPath string) error {
	if err := winfirewall.Apply(FirewallRuleGroup, routerFirewallRules(binaryPath), mode, true); err != nil {
		return fmt.Errorf("manage application-routing firewall rules: %w", err)
	}
	return nil
}

func validateFirewallBinaryPath(binaryPath string) error {
	if !filepath.IsAbs(binaryPath) || !strings.EqualFold(filepath.Base(binaryPath), "kokorobox-process-router.exe") || strings.ContainsRune(binaryPath, 0) {
		return errors.New("invalid process router path for firewall rule")
	}
	info, err := os.Stat(binaryPath)
	if err != nil {
		return fmt.Errorf("stat process router for firewall rule: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("process router path is not a regular file")
	}
	return nil
}
