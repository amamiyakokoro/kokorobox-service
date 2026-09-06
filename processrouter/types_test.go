package processrouter

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func validRequest() RulesRequest {
	return RulesRequest{
		Version: ProtocolVersion, ProxyPort: ProxyPort, FailClosed: true,
		Rules: []Rule{{
			ID: "discord", ExecutablePath: `C:\Program Files\Discord\Discord.exe`,
			ExecutableName: "Discord.exe", Protocol: "both", Action: "proxy",
			Enabled: true, Priority: 1,
		}},
	}
}

func TestNormalizeRulesRequest(t *testing.T) {
	request, err := normalizeRulesRequest(validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if len(request.Rules) != 1 || request.Rules[0].ExecutableName != "Discord.exe" {
		t.Fatalf("unexpected normalized request: %#v", request)
	}
}

func TestRejectsUnsafeRules(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*RulesRequest)
	}{
		{"fail open", func(r *RulesRequest) { r.FailClosed = false }},
		{"wrong port", func(r *RulesRequest) { r.ProxyPort = 1080 }},
		{"wildcard", func(r *RulesRequest) {
			r.Rules[0].ExecutablePath = `C:\Apps\*.exe`
			r.Rules[0].ExecutableName = "*.exe"
		}},
		{"relative path", func(r *RulesRequest) { r.Rules[0].ExecutablePath = `Discord.exe` }},
		{"protected process", func(r *RulesRequest) {
			r.Rules[0].ExecutablePath = `C:\Apps\KokoroBox.exe`
			r.Rules[0].ExecutableName = "KokoroBox.exe"
		}},
		{"name mismatch", func(r *RulesRequest) { r.Rules[0].ExecutableName = "Other.exe" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := validRequest()
			test.mutate(&request)
			if _, err := normalizeRulesRequest(request); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestProxyRulesBecomeBlockWhenMihomoIsUnavailable(t *testing.T) {
	request, err := normalizeRulesRequest(validRequest())
	if err != nil {
		t.Fatal(err)
	}
	command := buildRouterCommand(request, false)
	if command.Rules[0].Action != "BLOCK" {
		t.Fatalf("expected fail-closed block, got %s", command.Rules[0].Action)
	}
	payload, err := json.Marshal(command)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) == 0 {
		t.Fatal("expected native router command")
	}
}

func TestProbeRequiresSOCKS5Greeting(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer connection.Close()
		request := make([]byte, 3)
		if _, readErr := connection.Read(request); readErr == nil {
			_, _ = connection.Write([]byte{0x05, 0x00})
		}
	}()
	port := listener.Addr().(*net.TCPAddr).Port
	if !probeMihomo(port) {
		t.Fatal("expected a valid SOCKS5 greeting")
	}

	invalidListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer invalidListener.Close()
	go func() {
		connection, acceptErr := invalidListener.Accept()
		if acceptErr != nil {
			return
		}
		defer connection.Close()
		request := make([]byte, 3)
		if _, readErr := connection.Read(request); readErr == nil {
			_, _ = connection.Write([]byte{0x05, 0xff})
		}
	}()
	invalidPort := invalidListener.Addr().(*net.TCPAddr).Port
	if probeMihomo(invalidPort) {
		t.Fatal("accepted an invalid SOCKS5 greeting")
	}
}

func TestPersistsCanonicalRulesForServiceRestart(t *testing.T) {
	configDir := t.TempDir()
	manager := NewManager(filepath.Join(t.TempDir(), "process-router"), configDir)
	request := validRequest()
	request.Rules = append(request.Rules, Rule{
		ID: "updater", ExecutablePath: `D:\Apps\Updater.exe`, ExecutableName: "Updater.exe",
		Protocol: "tcp", Action: "block", Enabled: true, Priority: 9,
	})
	if _, err := manager.ReplaceRules(request); err != nil {
		t.Fatal(err)
	}
	manager.mu.Lock()
	manager.desired = true
	if err := manager.persistLocked(); err != nil {
		manager.mu.Unlock()
		t.Fatal(err)
	}
	manager.mu.Unlock()

	restored := NewManager(filepath.Join(t.TempDir(), "process-router"), configDir)
	restored.mu.Lock()
	err := restored.loadLocked()
	restored.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if !restored.desired || len(restored.rules.Rules) != 2 {
		t.Fatalf("unexpected restored configuration: %#v", restored.rules)
	}
	if restored.rules.Rules[1].Priority != 2 {
		t.Fatalf("expected normalized priority, got %d", restored.rules.Rules[1].Priority)
	}
	if _, err := os.Stat(filepath.Join(configDir, "config.json.tmp")); !os.IsNotExist(err) {
		t.Fatal("temporary configuration was not cleaned up")
	}
}

func TestExpiredClientLeasePreventsAutomaticRouterRestore(t *testing.T) {
	manager := NewManager(filepath.Join(t.TempDir(), "process-router"), t.TempDir())
	manager.mu.Lock()
	manager.desired = true
	manager.lastContact = time.Now().Add(-clientLease - time.Second)
	manager.state = StateRunning
	manager.mu.Unlock()
	if err := manager.Reconcile(); err != nil {
		t.Fatal(err)
	}
	if state := manager.Status().State; state != StateStopped {
		t.Fatalf("expected expired lease to stop router, got %s", state)
	}
}
