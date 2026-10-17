//go:build windows

package firewall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amamiyakokoro/kokorobox-service/internal/winfirewall"
)

func TestPolicyKeepsCoreInboundScope(t *testing.T) {
	path := "C:/ProgramData/KokoroBox/core-runtime/new hash's folder/mihomo.exe"
	rules := coreRules(path)
	if len(rules) != 2 {
		t.Fatal("expected TCP and UDP rules")
	}
	for i, rule := range rules {
		if rule.Group != "KokoroBox Service Core" || rule.Application != path || rule.Protocol != []int32{6, 17}[i] ||
			rule.LocalPorts != "*" || rule.RemotePorts != "*" || rule.LocalAddresses != "*" || rule.RemoteAddresses != "*" ||
			rule.Direction != 1 || rule.Action != 1 || rule.Profiles != winfirewall.AllProfiles || !rule.Enabled || rule.EdgeTraversal || rule.Service != "" {
			t.Fatalf("unexpected core scope: %+v", rule)
		}
	}
}

func TestEnsureRejectsInvalidExecutableBeforeTouchingFirewall(t *testing.T) {
	for _, path := range []string{"mihomo.exe", "C:/core/not-an-executable.txt", "C:/core/mihomo.exe" + string(rune(0)) + "other"} {
		if err := Ensure(path); err == nil {
			t.Fatalf("accepted invalid path %q", path)
		}
	}
	dir := filepath.Join(t.TempDir(), "mihomo.exe")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := Ensure(dir); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("accepted directory: %v", err)
	}
}
