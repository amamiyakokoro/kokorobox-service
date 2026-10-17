//go:build windows

package processrouter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/amamiyakokoro/kokorobox-service/internal/winfirewall"
)

func TestFirewallPolicyIsScopedToTheRouterAndRelayPorts(t *testing.T) {
	path := "C:/Program Files/KokoroBox Service/process-router/kokorobox-process-router.exe"
	rules := routerFirewallRules(path)
	if len(rules) != 2 {
		t.Fatal("expected two relay rules")
	}
	for i, rule := range rules {
		if rule.Group != "KokoroBox Application Routing" || rule.Application != path || rule.Protocol != []int32{6, 17}[i] ||
			rule.LocalPorts != []string{"34010", "34011"}[i] || rule.RemotePorts != "*" || rule.RemoteAddresses != "*" ||
			rule.LocalAddresses != "*" || rule.Direction != 1 || rule.Action != 1 || rule.Profiles != winfirewall.AllProfiles ||
			!rule.Enabled || rule.EdgeTraversal || rule.Service != "" {
			t.Fatalf("unexpected relay scope: %+v", rule)
		}
	}
}

func TestFirewallRejectsInvalidPathsBeforeAccessingPolicy(t *testing.T) {
	for _, path := range []string{"kokorobox-process-router.exe", "C:/router/other.exe", "C:/router/kokorobox-process-router.exe" + string(rune(0))} {
		if err := (windowsFirewall{}).Ensure(path); err == nil {
			t.Fatalf("ensure accepted %q", path)
		}
		if err := (windowsFirewall{}).Check(path); err == nil {
			t.Fatalf("check accepted %q", path)
		}
	}
	dir := filepath.Join(t.TempDir(), "kokorobox-process-router.exe")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := validateFirewallBinaryPath(dir); err == nil {
		t.Fatal("accepted directory")
	}
	file := filepath.Join(t.TempDir(), "kokorobox-process-router.exe")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := validateFirewallBinaryPath(file); err != nil {
		t.Fatal(err)
	}
}
