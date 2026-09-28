package processrouter

import (
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestProxyProbeReportsHandshakeFailures(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response []byte
		delay    bool
		want     string
	}{
		{"invalid version", []byte{4, 0}, false, "received 04 00"},
		{"authentication required", []byte{5, 2}, false, "received 05 02"},
		{"no accepted method", []byte{5, 255}, false, "received 05 ff"},
		{"incomplete response", []byte{5}, false, "unexpected EOF"},
		{"closed connection", nil, false, "EOF"},
		{"handshake timeout", nil, true, "timeout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			done := make(chan struct{})
			defer close(done)
			serverErr := make(chan error, 1)
			go func() {
				connection, err := listener.Accept()
				if err != nil {
					serverErr <- err
					return
				}
				defer connection.Close()
				_ = connection.SetDeadline(time.Now().Add(5 * time.Second))
				greeting := make([]byte, 3)
				if _, err := io.ReadFull(connection, greeting); err != nil {
					serverErr <- err
					return
				}
				if string(greeting) != string([]byte{5, 1, 0}) {
					serverErr <- errors.New("unexpected client greeting")
					return
				}
				if tc.delay {
					serverErr <- nil
					<-done
					return
				}
				_, err = connection.Write(tc.response)
				serverErr <- err
			}()
			err = probeMihomo(listener.Addr().(*net.TCPAddr).Port, true)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
			if err := <-serverErr; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestProxyProbeReportsConnectFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	if err := probeMihomo(port, true); err == nil || !strings.Contains(err.Error(), "connect to proxy") {
		t.Fatalf("expected connection error, got %v", err)
	}
}

func TestProxyHealthTransitionsRecordFailureAndRecoveryOnce(t *testing.T) {
	ClearDiagnosticLogs()
	t.Cleanup(ClearDiagnosticLogs)
	manager := &Manager{state: StateStarting, rules: RulesRequest{ProxyPort: WindowsProxyPort}}
	manager.updateProxyHealthLocked(true, errors.New("read SOCKS5 greeting response: timeout"))
	if manager.state != StateBlocked || manager.mihomoReady || !strings.Contains(manager.lastError, "timeout") {
		t.Fatalf("unexpected blocked status: %+v", manager)
	}
	entries := DiagnosticLogs()
	if len(entries) != 1 || entries[0].Level != "warning" || !strings.Contains(entries[0].Message, "127.0.0.1:7891") {
		t.Fatalf("missing health warning: %+v", entries)
	}
	manager.updateProxyHealthLocked(true, errors.New("connect to proxy: connection refused"))
	if len(DiagnosticLogs()) != 1 || !strings.Contains(manager.lastError, "connection refused") {
		t.Fatal("repeated probes should update status without flooding logs")
	}
	manager.updateProxyHealthLocked(true, nil)
	manager.updateProxyHealthLocked(true, nil)
	entries = DiagnosticLogs()
	if manager.state != StateRunning || !manager.mihomoReady || manager.lastError != "" ||
		len(entries) != 2 || entries[1].Level != "info" || !strings.Contains(entries[1].Message, "recovered") {
		t.Fatalf("unexpected recovery status/logs: %+v %+v", manager, entries)
	}
	manager.updateProxyHealthLocked(false, nil)
	if manager.mihomoReady || len(DiagnosticLogs()) != 2 {
		t.Fatal("direct-only routing must not report a proxy recovery")
	}
}
