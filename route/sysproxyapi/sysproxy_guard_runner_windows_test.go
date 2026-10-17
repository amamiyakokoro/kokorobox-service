//go:build windows

package sysproxyapi

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/amamiyakokoro/kokorobox-service/route/pipectx"
	"golang.org/x/sys/windows"
)

type sysproxyTestPipeConn struct {
	net.Conn
	handle windows.Handle
}

func (c sysproxyTestPipeConn) Handle() windows.Handle { return c.handle }

// Use a real named-pipe client token without reading or changing OS proxy settings.
func sysproxyPipeRequest(t *testing.T) *http.Request {
	t.Helper()
	name, err := windows.UTF16PtrFromString(fmt.Sprintf(`\\.\pipe\kokorobox-sysproxy-test-%d-%d`, os.Getpid(), time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	server, err := windows.CreateNamedPipe(name, windows.PIPE_ACCESS_DUPLEX,
		windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_NOWAIT,
		1, 1024, 1024, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = windows.CloseHandle(server) })
	client, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE,
		0, nil, windows.OPEN_EXISTING,
		windows.SECURITY_SQOS_PRESENT|windows.SECURITY_IMPERSONATION, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = windows.CloseHandle(client) })
	if err := windows.ConnectNamedPipe(server, nil); err != nil && !errors.Is(err, windows.ERROR_PIPE_CONNECTED) {
		t.Fatal(err)
	}
	var n uint32
	if err := windows.WriteFile(client, []byte{1}, &n, nil); err != nil {
		t.Fatal(err)
	}
	var data [1]byte
	if err := windows.ReadFile(server, data[:], &n, nil); err != nil {
		t.Fatal(err)
	}
	httpServer := &http.Server{}
	pipectx.ConfigureServer(httpServer)
	ctx := httpServer.ConnContext(context.Background(), sysproxyTestPipeConn{handle: server})
	return httptest.NewRequest("POST", "/proxy", nil).WithContext(ctx)
}

func sysproxyThreadTokenPresent() (bool, error) {
	var token windows.Token
	err := windows.OpenThreadToken(windows.CurrentThread(), windows.TOKEN_QUERY, true, &token)
	if errors.Is(err, windows.ERROR_NO_TOKEN) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, token.Close()
}

func TestWindowsSysproxyRequestRunnerKeepsBookkeepingOutsideImpersonation(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	request := sysproxyPipeRequest(t)
	previousPath := managedProxyRecordPath
	managedProxyRecordPath = filepath.Join(t.TempDir(), "sysproxy", "managed.json")
	t.Cleanup(func() { managedProxyRecordPath = previousPath })

	runner, err := captureSysproxyGuardRunner(request)
	if err != nil {
		t.Fatal(err)
	}
	defer runner.Close()
	userRunner, ok := runner.(*windowsTokenSysproxyGuardRunner)
	if !ok {
		t.Fatal("request runner did not retain the named-pipe user's token")
	}
	if present, err := sysproxyThreadTokenPresent(); err != nil || present {
		t.Fatalf("token capture left the service impersonating: present=%v err=%v", present, err)
	}

	operationErr := errors.New("proxy operation failed")
	for _, result := range []error{nil, operationErr} {
		err := userRunner.run(func() error {
			if present, err := sysproxyThreadTokenPresent(); err != nil || !present {
				t.Fatalf("proxy operation lost client identity: present=%v err=%v", present, err)
			}
			return result
		})
		if !errors.Is(err, result) {
			t.Fatalf("operation result changed: got %v want %v", err, result)
		}
		if present, err := sysproxyThreadTokenPresent(); err != nil || present {
			t.Fatalf("bookkeeping still impersonates client: present=%v err=%v", present, err)
		}
		if err := writeManagedProxyRecord(managedProxyRecord{Version: managedProxyRecordVersion, Mode: sysproxyGuardModeProxy}); err != nil {
			t.Fatalf("write protected recovery record: %v", err)
		}
		forgetSysproxyLease()
		if _, err := os.Stat(managedProxyRecordPath); !os.IsNotExist(err) {
			t.Fatalf("recovery record was not removed: %v", err)
		}
	}
}
