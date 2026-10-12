package sysproxyapi

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestManagedPACServerScopeAndLifecycle(t *testing.T) {
	script := "function FindProxyForURL() { return 'DIRECT'; }"
	server, err := startManagedPACServer(script)
	if err != nil {
		t.Fatal(err)
	}
	defer server.close()
	if !strings.HasPrefix(server.url, "http://127.0.0.1:") {
		t.Fatal(server.url)
	}
	response, err := http.Get(server.url)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if string(data) != script || response.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("incorrect PAC content")
	}
	response, err = http.Post(server.url, "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 404 {
		t.Fatal("unexpected write endpoint")
	}
	server.close()
	if response, err = http.Get(server.url); err == nil {
		response.Body.Close()
		t.Fatal("server still listening")
	}
}
func TestManagedPACServerBounds(t *testing.T) {
	for _, script := range []string{"", strings.Repeat("x", maxPACScriptBytes+1)} {
		if server, err := startManagedPACServer(script); err == nil {
			server.close()
			t.Fatal("invalid script accepted")
		}
	}
}

func TestProxyLeaseCleanupClosesManagedPACServer(t *testing.T) {
	server, err := startManagedPACServer("function FindProxyForURL() { return 'DIRECT'; }")
	if err != nil {
		t.Fatal(err)
	}
	runner := &leaseTestRunner{config: testProxyConfig("127.0.0.1:7890")}
	activeSysproxyLease = &sysproxyLease{runner: runner, pacServer: server}
	clearSysproxyLease()
	response, err := http.Get(server.url)
	if err == nil {
		response.Body.Close()
		t.Fatal("lease cleanup left PAC listener open")
	}
	if runner.closed != 1 {
		t.Fatal("lease runner was not closed")
	}
}
