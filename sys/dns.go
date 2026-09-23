package sys

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type DNSService struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func ActiveDNSService() (DNSService, error) {
	if runtime.GOOS != "darwin" {
		return DNSService{}, fmt.Errorf("DNS service discovery is unsupported on %s", runtime.GOOS)
	}
	for _, key := range []string{"State:/Network/Global/IPv4", "State:/Network/Global/IPv6"} {
		state, err := scutilShow(key)
		if err != nil {
			return DNSService{}, err
		}
		id := scutilValue(state, "PrimaryService")
		if id == "" {
			continue
		}
		for _, ch := range id {
			if !strings.ContainsRune("0123456789abcdefABCDEF-", ch) {
				return DNSService{}, fmt.Errorf("invalid active network service identifier")
			}
		}
		setup, err := scutilShow("Setup:/Network/Service/" + id)
		if err != nil {
			return DNSService{}, err
		}
		name := scutilValue(setup, "UserDefinedName")
		if name == "" {
			return DNSService{}, fmt.Errorf("active network service has no name")
		}
		return DNSService{ID: id, Name: name}, nil
	}
	return DNSService{}, fmt.Errorf("no active network service")
}

// ActiveNetworkSignature identifies a macOS network transition without
// treating an unrelated proxy preference edit as a target change. It stays
// internal to the Service and must not be written to logs or lease records.
func ActiveNetworkSignature() (string, error) {
	service, err := ActiveDNSService()
	if err != nil {
		return "", err
	}
	global := ""
	for _, key := range []string{"State:/Network/Global/IPv4", "State:/Network/Global/IPv6"} {
		state, err := scutilShow(key)
		if err != nil {
			return "", err
		}
		if scutilValue(state, "PrimaryService") == service.ID {
			global = state
			break
		}
	}
	if global == "" {
		return "", fmt.Errorf("active network service has no global state")
	}
	iface := scutilValue(global, "PrimaryInterface")
	parts := []string{service.ID, iface, scutilValue(global, "Router")}
	if iface != "" && !strings.HasPrefix(iface, "-") && !strings.ContainsAny(iface, "\x00\r\n/") {
		if state, err := scutilShow("State:/Network/Interface/" + iface + "/IPv4"); err == nil {
			parts = append(parts, state)
		}
		if network, err := exec.Command("networksetup", "-getairportnetwork", iface).Output(); err == nil {
			parts = append(parts, strings.TrimSpace(string(network)))
		}
	}
	return strings.Join(parts, "\x00"), nil
}

func scutilShow(key string) (string, error) {
	cmd := exec.Command("scutil")
	cmd.Stdin = strings.NewReader("show " + key + "\nquit\n")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("scutil %s: %w: %s", key, err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func scutilValue(output, key string) string {
	for _, line := range strings.Split(output, "\n") {
		name, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if ok && strings.TrimSpace(name) == key {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func GetDns(device string) ([]string, error) {
	if runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("DNS query is unsupported on %s", runtime.GOOS)
	}
	if err := validateDNSServiceName(device); err != nil {
		return nil, err
	}
	cmd := exec.Command("networksetup", "-getdnsservers", device)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("query DNS for %s: %w: %s", device, err, strings.TrimSpace(string(output)))
	}
	return parseDNSOutput(string(output))
}

func parseDNSOutput(output string) ([]string, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	servers := make([]string, 0, len(lines))
	for _, line := range lines {
		value := strings.TrimSpace(line)
		if value == "" {
			continue
		}
		if net.ParseIP(value) == nil {
			if len(servers) == 0 && strings.HasPrefix(value, "There aren't any DNS Servers set on") {
				return nil, nil
			}
			return nil, fmt.Errorf("unrecognized networksetup DNS output: %q", value)
		}
		servers = append(servers, value)
	}
	return servers, nil
}

func SetDns(device string, servers []string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("DNS mutation is unsupported on %s", runtime.GOOS)
	}
	if err := validateDNSServiceName(device); err != nil {
		return err
	}
	args := []string{"-setdnsservers", device}
	if len(servers) == 0 {
		args = append(args, "Empty")
	} else {
		for _, server := range servers {
			if net.ParseIP(server) == nil {
				return fmt.Errorf("invalid DNS server address: %q", server)
			}
		}
		args = append(args, servers...)
	}
	if output, err := exec.Command("networksetup", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("set DNS for %s: %w: %s", device, err, strings.TrimSpace(string(output)))
	}
	if output, err := exec.Command("dscacheutil", "-flushcache").CombinedOutput(); err != nil {
		return fmt.Errorf("flush DNS cache: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func validateDNSServiceName(device string) error {
	if device == "" || strings.HasPrefix(device, "-") || strings.ContainsAny(device, "\x00\r\n") {
		return fmt.Errorf("invalid network service name")
	}
	return nil
}
