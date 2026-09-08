package processrouter

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

const (
	ProtocolVersion = 1
	ProxyPort       = 7891
	maxRules        = 256
	maxPathBytes    = 1023
)

var (
	ErrUnsupported = errors.New("process router is only supported on Windows x64")
	validProtocols = map[string]bool{"tcp": true, "udp": true, "both": true}
	validActions   = map[string]bool{"proxy": true, "direct": true, "block": true}
	protectedNames = map[string]bool{
		"kokorobox.exe":                true,
		"mihomo.exe":                   true,
		"mihomo-alpha.exe":             true,
		"kokorobox-service.exe":        true,
		"sparkle-service.exe":          true, // upgrade compatibility
		"kokorobox-process-router.exe": true,
		"crashpad_handler.exe":         true,
		"elevate.exe":                  true,
	}
)

type ValidationError struct {
	Err error
}

func (e *ValidationError) Error() string { return e.Err.Error() }
func (e *ValidationError) Unwrap() error { return e.Err }

type Rule struct {
	ID             string `json:"id"`
	ExecutablePath string `json:"executable_path"`
	ExecutableName string `json:"executable_name"`
	Protocol       string `json:"protocol"`
	Action         string `json:"action"`
	Enabled        bool   `json:"enabled"`
	Priority       int    `json:"priority"`
}

type RulesRequest struct {
	Version    int    `json:"version"`
	ProxyPort  int    `json:"proxy_port"`
	FailClosed bool   `json:"fail_closed"`
	Rules      []Rule `json:"rules"`
}

type State string

const (
	StateStopped  State = "stopped"
	StateStarting State = "starting"
	StateRunning  State = "running"
	StateBlocked  State = "blocked"
	StateError    State = "error"
)

type Status struct {
	Version                   int    `json:"version"`
	Supported                 bool   `json:"supported"`
	State                     State  `json:"state"`
	Generation                uint64 `json:"generation"`
	MihomoAvailable           bool   `json:"mihomo_available"`
	ProtectedApplicationCount int    `json:"protected_application_count"`
	ProxyPort                 int    `json:"proxy_port,omitempty"`
	RouterPID                 int    `json:"router_pid,omitempty"`
	LastError                 string `json:"last_error,omitempty"`
}

type persistedConfig struct {
	Version int          `json:"version"`
	Enabled bool         `json:"enabled"`
	Rules   RulesRequest `json:"rules"`
}

type routerRule struct {
	ExecutablePath string `json:"executablePath"`
	Protocol       string `json:"protocol"`
	Action         string `json:"action"`
	Enabled        bool   `json:"enabled"`
	Priority       int    `json:"priority"`
}

type routerCommand struct {
	Version    int          `json:"version"`
	Command    string       `json:"command"`
	Proxy      routerProxy  `json:"proxy"`
	FailClosed bool         `json:"failClosed"`
	Rules      []routerRule `json:"rules"`
}

type routerProxy struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

func normalizeRulesRequest(request RulesRequest) (RulesRequest, error) {
	if request.Version != ProtocolVersion {
		return RulesRequest{}, fmt.Errorf("unsupported process router version: %d", request.Version)
	}
	if request.ProxyPort != ProxyPort {
		return RulesRequest{}, fmt.Errorf("proxy port must be %d", ProxyPort)
	}
	if !request.FailClosed {
		return RulesRequest{}, errors.New("fail_closed must be true")
	}
	if len(request.Rules) > maxRules {
		return RulesRequest{}, fmt.Errorf("process routing supports at most %d rules", maxRules)
	}

	rules := append([]Rule(nil), request.Rules...)
	sort.SliceStable(rules, func(i, j int) bool { return rules[i].Priority < rules[j].Priority })
	ids := make(map[string]struct{}, len(rules))
	paths := make(map[string]struct{}, len(rules))
	priorities := make(map[int]struct{}, len(rules))
	totalPathBytes := 0
	for index := range rules {
		rule := &rules[index]
		if err := validateRule(*rule); err != nil {
			return RulesRequest{}, fmt.Errorf("invalid rule %d: %w", index+1, err)
		}
		pathKey := strings.ToLower(rule.ExecutablePath)
		if _, exists := ids[rule.ID]; exists {
			return RulesRequest{}, errors.New("rule IDs must be unique")
		}
		if _, exists := paths[pathKey]; exists {
			return RulesRequest{}, errors.New("executable paths must be unique")
		}
		if _, exists := priorities[rule.Priority]; exists {
			return RulesRequest{}, errors.New("rule priorities must be unique")
		}
		ids[rule.ID] = struct{}{}
		paths[pathKey] = struct{}{}
		priorities[rule.Priority] = struct{}{}
		totalPathBytes += len([]byte(rule.ExecutablePath)) + 1
		if totalPathBytes > 30000 {
			return RulesRequest{}, errors.New("application routing paths are too large")
		}
		rule.Priority = index + 1
	}

	request.Rules = rules
	return request, nil
}

func validateRule(rule Rule) error {
	if rule.ID == "" || len(rule.ID) > 128 {
		return errors.New("invalid rule ID")
	}
	if !validWindowsExecutablePath(rule.ExecutablePath) {
		return errors.New("executable_path must be an absolute canonical Windows .exe path")
	}
	name := windowsBaseName(rule.ExecutablePath)
	if name == "" || !strings.EqualFold(name, rule.ExecutableName) {
		return errors.New("executable_name does not match executable_path")
	}
	if isProtectedProcess(name) {
		return fmt.Errorf("%s cannot be intercepted", name)
	}
	if !validProtocols[rule.Protocol] {
		return errors.New("protocol must be tcp, udp, or both")
	}
	if !validActions[rule.Action] {
		return errors.New("action must be proxy, direct, or block")
	}
	if rule.Priority < 1 || rule.Priority > maxRules {
		return errors.New("priority is out of range")
	}
	return nil
}

func validWindowsExecutablePath(value string) bool {
	if value == "" || len([]byte(value)) > maxPathBytes || strings.ContainsRune(value, '\x00') {
		return false
	}
	if strings.ContainsAny(value, "*?;,") || strings.Contains(value, "/") {
		return false
	}
	if strings.HasPrefix(value, `\\?\`) || strings.HasPrefix(value, `\\.\`) {
		return false
	}
	drivePath := len(value) > 3 && isASCIIAlpha(value[0]) && value[1] == ':' && value[2] == '\\'
	uncPath := len(value) > 4 && strings.HasPrefix(value, `\\`)
	if !drivePath && !uncPath {
		return false
	}
	if !strings.HasSuffix(strings.ToLower(value), ".exe") {
		return false
	}
	pathBody := value[3:]
	minimumSegments := 1
	if uncPath {
		pathBody = value[2:]
		minimumSegments = 3
	}
	segments := strings.Split(pathBody, `\`)
	if len(segments) < minimumSegments {
		return false
	}
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." || strings.HasSuffix(segment, ".") || strings.HasSuffix(segment, " ") || strings.ContainsAny(segment, `<>:"|`) {
			return false
		}
		for _, character := range segment {
			if character < 32 {
				return false
			}
		}
	}
	return true
}

func isASCIIAlpha(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func windowsBaseName(value string) string {
	if index := strings.LastIndexByte(value, '\\'); index >= 0 {
		return value[index+1:]
	}
	return value
}

func isProtectedProcess(name string) bool {
	normalized := strings.ToLower(name)
	if protectedNames[normalized] {
		return true
	}
	return strings.HasPrefix(normalized, "kokorobox-desktop-windows-") && strings.HasSuffix(normalized, "-setup.exe")
}

func buildRouterCommand(request RulesRequest, proxyAvailable bool) routerCommand {
	rules := make([]routerRule, 0, len(request.Rules))
	for _, rule := range request.Rules {
		action := strings.ToUpper(rule.Action)
		if rule.Action == "proxy" && !proxyAvailable {
			action = "BLOCK"
		}
		rules = append(rules, routerRule{
			ExecutablePath: rule.ExecutablePath,
			Protocol:       strings.ToUpper(rule.Protocol),
			Action:         action,
			Enabled:        rule.Enabled,
			Priority:       rule.Priority,
		})
	}
	return routerCommand{
		Version:    ProtocolVersion,
		Command:    "replace_rules",
		Proxy:      routerProxy{Host: "127.0.0.1", Port: ProxyPort},
		FailClosed: true,
		Rules:      rules,
	}
}

func hasProxyRules(request RulesRequest) bool {
	for _, rule := range request.Rules {
		if rule.Enabled && rule.Action == "proxy" {
			return true
		}
	}
	return false
}

func hasEnabledRules(request RulesRequest) bool {
	for _, rule := range request.Rules {
		if rule.Enabled {
			return true
		}
	}
	return false
}

func protectedRuleCount(request RulesRequest) int {
	count := 0
	for _, rule := range request.Rules {
		if rule.Enabled && rule.Action == "proxy" {
			count++
		}
	}
	return count
}
