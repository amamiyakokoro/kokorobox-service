package processrouter

import (
	"fmt"
	"strconv"
)

type linuxCgroupMode string

const (
	linuxCgroupV2 linuxCgroupMode = "cgroup-v2"
	linuxCgroupV1 linuxCgroupMode = "cgroup-v1-net-cls"

	linuxRoutingMark  = "0x4b42"
	linuxRoutingTable = "20269"
	linuxRulePriority = "10900"
	linuxOutputChain  = "KOKOROBOX_OUT"
	linuxInputChain   = "KOKOROBOX_PRE"
	linuxDNSChain     = "KOKOROBOX_DNS"
	linuxBlockChain   = "KOKOROBOX_BLOCK"
)

var linuxRoutingGroups = []string{
	"proxy-tcp", "proxy-udp", "proxy-both",
	"direct-tcp", "direct-udp", "direct-both",
	"block-tcp", "block-udp", "block-both",
}

var linuxNetClsClassIDs = map[string]string{
	"proxy-tcp":   "0x100001",
	"proxy-udp":   "0x100002",
	"proxy-both":  "0x100003",
	"direct-tcp":  "0x100011",
	"direct-udp":  "0x100012",
	"direct-both": "0x100013",
	"block-tcp":   "0x100021",
	"block-udp":   "0x100022",
	"block-both":  "0x100023",
}

type linuxCommand struct {
	Name string
	Args []string
}

func linuxRoutingGroup(action, protocol string) string {
	return action + "-" + protocol
}

func linuxCgroupMatch(mode linuxCgroupMode, group string) []string {
	if mode == linuxCgroupV2 {
		return []string{"-m", "cgroup", "--path", "kokorobox-app-routing/" + group}
	}
	return []string{"-m", "cgroup", "--cgroup", linuxNetClsClassIDs[group]}
}

func appendLinuxCommand(plan []linuxCommand, name string, args ...string) []linuxCommand {
	return append(plan, linuxCommand{Name: name, Args: args})
}

func linuxProtocols(group string) []string {
	if len(group) >= 4 && group[len(group)-4:] == "both" {
		return []string{"tcp", "udp"}
	}
	if len(group) >= 3 && group[len(group)-3:] == "tcp" {
		return []string{"tcp"}
	}
	return []string{"udp"}
}

func buildLinuxFirewallSetup(mode linuxCgroupMode, proxyPort int, proxyUDPDNS bool) []linuxCommand {
	plan := buildLinuxFirewallCleanup()
	for _, binary := range []string{"iptables", "ip6tables"} {
		plan = appendLinuxCommand(plan, binary, "-t", "mangle", "-N", linuxOutputChain)
		plan = appendLinuxCommand(plan, binary, "-t", "mangle", "-N", linuxInputChain)
		plan = appendLinuxCommand(plan, binary, "-t", "filter", "-N", linuxBlockChain)
		plan = appendLinuxCommand(plan, binary, "-t", "mangle", "-A", "OUTPUT", "-j", linuxOutputChain)
		plan = appendLinuxCommand(plan, binary, "-t", "mangle", "-A", "PREROUTING", "-j", linuxInputChain)
		plan = appendLinuxCommand(plan, binary, "-t", "filter", "-A", "OUTPUT", "-j", linuxBlockChain)
		if proxyUDPDNS {
			plan = appendLinuxCommand(plan, binary, "-t", "nat", "-N", linuxDNSChain)
			plan = appendLinuxCommand(plan, binary, "-t", "nat", "-A", "OUTPUT", "-j", linuxDNSChain)
		}

		protected := []string{"127.0.0.0/8", "169.254.0.0/16", "224.0.0.0/4"}
		if binary == "ip6tables" {
			protected = []string{"::1/128", "fe80::/10", "ff00::/8"}
		}
		for _, group := range []string{"proxy-tcp", "proxy-udp", "proxy-both"} {
			match := linuxCgroupMatch(mode, group)
			for _, protocol := range linuxProtocols(group) {
				if proxyUDPDNS {
					args := append([]string{"-t", "nat", "-A", linuxDNSChain}, match...)
					args = append(args, "-p", protocol, "--dport", "53", "-j", "REDIRECT", "--to-ports", strconv.Itoa(DNSPort))
					plan = appendLinuxCommand(plan, binary, args...)
				}
				for _, target := range protected {
					args := append([]string{"-t", "mangle", "-A", linuxOutputChain}, match...)
					args = append(args, "-p", protocol, "-d", target, "-j", "RETURN")
					plan = appendLinuxCommand(plan, binary, args...)
				}
				args := append([]string{"-t", "mangle", "-A", linuxOutputChain}, match...)
				args = append(args, "-p", protocol, "-j", "MARK", "--set-mark", linuxRoutingMark)
				plan = appendLinuxCommand(plan, binary, args...)
			}
		}
		for _, group := range []string{"block-tcp", "block-udp", "block-both"} {
			match := linuxCgroupMatch(mode, group)
			for _, protocol := range linuxProtocols(group) {
				args := append([]string{"-t", "filter", "-A", linuxBlockChain}, match...)
				args = append(args, "-p", protocol, "-j", "DROP")
				plan = appendLinuxCommand(plan, binary, args...)
			}
		}
		for _, protocol := range []string{"tcp", "udp"} {
			plan = appendLinuxCommand(
				plan,
				binary,
				"-t", "mangle", "-A", linuxInputChain,
				"-m", "mark", "--mark", linuxRoutingMark,
				"-p", protocol,
				"-j", "TPROXY",
				"--on-port", strconv.Itoa(proxyPort),
				"--tproxy-mark", linuxRoutingMark+"/0xffffffff",
			)
		}
	}

	plan = appendLinuxCommand(plan, "ip", "-4", "route", "replace", "local", "0.0.0.0/0", "dev", "lo", "table", linuxRoutingTable)
	plan = appendLinuxCommand(plan, "ip", "-6", "route", "replace", "local", "::/0", "dev", "lo", "table", linuxRoutingTable)
	plan = appendLinuxCommand(plan, "ip", "-4", "rule", "add", "priority", linuxRulePriority, "fwmark", linuxRoutingMark+"/0xffffffff", "lookup", linuxRoutingTable)
	plan = appendLinuxCommand(plan, "ip", "-6", "rule", "add", "priority", linuxRulePriority, "fwmark", linuxRoutingMark+"/0xffffffff", "lookup", linuxRoutingTable)
	return plan
}

func buildLinuxFirewallCleanup() []linuxCommand {
	var plan []linuxCommand
	for _, binary := range []string{"iptables", "ip6tables"} {
		for _, item := range []struct {
			table, parent, chain string
		}{
			{"mangle", "OUTPUT", linuxOutputChain},
			{"mangle", "PREROUTING", linuxInputChain},
			{"nat", "OUTPUT", linuxDNSChain},
			{"filter", "OUTPUT", linuxBlockChain},
		} {
			plan = appendLinuxCommand(plan, binary, "-t", item.table, "-D", item.parent, "-j", item.chain)
			plan = appendLinuxCommand(plan, binary, "-t", item.table, "-F", item.chain)
			plan = appendLinuxCommand(plan, binary, "-t", item.table, "-X", item.chain)
		}
	}
	plan = appendLinuxCommand(plan, "ip", "-4", "rule", "del", "priority", linuxRulePriority, "fwmark", linuxRoutingMark+"/0xffffffff", "lookup", linuxRoutingTable)
	plan = appendLinuxCommand(plan, "ip", "-6", "rule", "del", "priority", linuxRulePriority, "fwmark", linuxRoutingMark+"/0xffffffff", "lookup", linuxRoutingTable)
	plan = appendLinuxCommand(plan, "ip", "-4", "route", "del", "local", "0.0.0.0/0", "dev", "lo", "table", linuxRoutingTable)
	plan = appendLinuxCommand(plan, "ip", "-6", "route", "del", "local", "::/0", "dev", "lo", "table", linuxRoutingTable)
	return plan
}

func validateLinuxRoutingGroup(group string) error {
	for _, candidate := range linuxRoutingGroups {
		if group == candidate {
			return nil
		}
	}
	return fmt.Errorf("unsupported Linux routing group: %s", group)
}
