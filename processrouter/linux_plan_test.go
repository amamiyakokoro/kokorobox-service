package processrouter

import (
	"slices"
	"strings"
	"testing"
)

func TestLinuxCgroupV2FirewallPlanUsesPathMatchingAndTPROXY(t *testing.T) {
	plan := buildLinuxFirewallSetup(linuxCgroupV2, LinuxProxyPort, true)
	joined := renderLinuxPlan(plan)
	for _, expected := range []string{
		"iptables -t mangle -A KOKOROBOX_OUT -m cgroup --path kokorobox-app-routing/proxy-both",
		"ip6tables -t mangle -A KOKOROBOX_PRE",
		"--on-port 7894",
		"--to-ports 7892",
		"iptables -t filter -A KOKOROBOX_GUARD ! -i lo -p tcp --dport 7894 -j DROP",
		"ip -4 rule add priority 10900 fwmark 0x4b42/0xffffffff lookup 20269",
		"ip -4 rule del priority 10900 fwmark 0x4b42/0xffffffff lookup 20269",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("Linux v2 firewall plan does not contain %q\n%s", expected, joined)
		}
	}
}

func TestLinuxCgroupV1FirewallPlanUsesNetClsClassID(t *testing.T) {
	joined := renderLinuxPlan(buildLinuxFirewallSetup(linuxCgroupV1, LinuxProxyPort, false))
	if !strings.Contains(joined, "--cgroup 0x100003") {
		t.Fatalf("Linux v1 firewall plan does not use the proxy classid:\n%s", joined)
	}
	if strings.Contains(joined, "--to-ports 7892") {
		t.Fatalf("disabled UDP DNS should not install a DNS redirect:\n%s", joined)
	}
}

func TestLinuxRoutingGroupsCoverEveryActionAndProtocol(t *testing.T) {
	for _, action := range []string{"proxy", "direct", "block"} {
		for _, protocol := range []string{"tcp", "udp", "both"} {
			group := linuxRoutingGroup(action, protocol)
			if !slices.Contains(linuxRoutingGroups, group) {
				t.Fatalf("missing Linux routing group %s", group)
			}
			if err := validateLinuxRoutingGroup(group); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func renderLinuxPlan(plan []linuxCommand) string {
	lines := make([]string, 0, len(plan))
	for _, command := range plan {
		lines = append(lines, command.Name+" "+strings.Join(command.Args, " "))
	}
	return strings.Join(lines, "\n")
}
