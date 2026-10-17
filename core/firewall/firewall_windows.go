//go:build windows

package firewall

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/amamiyakokoro/kokorobox-service/internal/winfirewall"
)

const coreRuleGroup = "KokoroBox Service Core"

func coreRules(executable string) []winfirewall.Rule {
	rules := []winfirewall.Rule{
		winfirewall.Inbound(coreRuleGroup+" TCP", coreRuleGroup, executable, 6, "*"),
		winfirewall.Inbound(coreRuleGroup+" UDP", coreRuleGroup, executable, 17, "*"),
	}
	for i := range rules {
		rules[i].Description = "Inbound proxy connections to the service-managed KokoroBox core. Mihomo controls LAN access."
	}
	return rules
}

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
	if err := winfirewall.Apply(coreRuleGroup, coreRules(executable), winfirewall.Ensure, false); err != nil {
		return fmt.Errorf("configure core firewall rules: %w", err)
	}
	return nil
}
