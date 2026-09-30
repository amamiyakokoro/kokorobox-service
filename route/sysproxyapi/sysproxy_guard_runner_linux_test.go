//go:build linux

package sysproxyapi

import (
	"io"
	"net/http/httptest"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/amamiyakokoro/kokorobox-service/route/pipectx"
	"github.com/amamiyakokoro/sysproxy-go/v2/sysproxy"
)

func TestCaptureLinuxSysproxySessionWithEmptyProcessEnvironment(t *testing.T) {
	process := startLinuxSysproxySessionProcess(t, []string{})
	pid := process.Process.Pid
	environment, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/environ")
	if err != nil || len(environment) != 0 {
		t.Fatalf("expected empty process environment: size=%d, error=%v", len(environment), err)
	}
	peer := pipectx.UnixPeerInfo{PID: pid, UID: uint32(os.Getuid()), GID: uint32(os.Getgid())}
	runner, err := captureLinuxSysproxyGuardRunner(peer)
	if err != nil {
		t.Fatal(err)
	}
	if len(runner.env) == 0 || runner.uid != peer.UID || runner.gid != peer.GID {
		t.Fatalf("session was not captured for the socket user: environment size=%d, uid=%d, gid=%d", len(runner.env), runner.uid, runner.gid)
	}
	// The resolved environment must survive the Desktop process disappearing.
	_ = process.Process.Kill()
	_ = process.Wait()
	options := runner.sessionOptions(&sysproxy.Options{PeerPID: pid})
	if len(options.Environment) == 0 {
		t.Fatal("lease cleanup lost its captured session after the peer exited")
	}
	if peer.UID != 0 {
		recovery, err := recoverySysproxyOptions(options, runner)
		if err != nil {
			t.Fatal(err)
		}
		if recovery.PeerPID != 0 || recovery.PeerUID != peer.UID || len(recovery.Environment) == 0 {
			t.Fatalf("session cannot be recovered after service restart: pid=%d, uid=%d, environment size=%d", recovery.PeerPID, recovery.PeerUID, len(recovery.Environment))
		}
		for _, item := range recovery.Environment {
			key, _, ok := strings.Cut(item, "=")
			if !ok || !recoveryLinuxEnvKeys[key] {
				t.Fatalf("recovery record contains an unexpected environment key: %q", key)
			}
		}
	}
}

func TestCaptureLinuxSysproxySessionPreservesProcessEnvironment(t *testing.T) {
	process := startLinuxSysproxySessionProcess(t, []string{"HOME=/home/example", "XDG_CURRENT_DESKTOP=KDE", "PRIVATE=preserved"})
	peer := pipectx.UnixPeerInfo{PID: process.Process.Pid, UID: uint32(os.Getuid()), GID: uint32(os.Getgid())}
	runner, err := captureLinuxSysproxyGuardRunner(peer)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(runner.env, process.Env) || runner.uid != peer.UID || runner.gid != peer.GID {
		t.Fatalf("process session was not preserved: environment size=%d, uid=%d, gid=%d", len(runner.env), runner.uid, runner.gid)
	}
}

func TestCaptureLinuxSysproxySessionRequiresSocketIdentity(t *testing.T) {
	if _, err := captureSysproxyGuardRunner(httptest.NewRequest("POST", "/proxy", nil)); err == nil {
		t.Fatal("accepted a request without a local user identity")
	}
}

func startLinuxSysproxySessionProcess(t *testing.T, environment []string) *exec.Cmd {
	t.Helper()
	process := exec.Command("sh", "-c", "printf ready; read wait")
	process.Env = environment
	stdin, err := process.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := process.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		_ = process.Process.Kill()
		_ = process.Wait()
	})
	// Start may return while /proc still exposes the pre-exec environment.
	// Wait for the child to finish startup before inspecting its session.
	ready := make([]byte, 5)
	if _, err := io.ReadFull(stdout, ready); err != nil || string(ready) != "ready" {
		t.Fatalf("session process did not become ready: %v", err)
	}
	return process
}
