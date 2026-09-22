package core

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestStartupFatalLineErrorRejectsListenerFailures(t *testing.T) {
	tests := []string{
		`level=error msg="Start Mixed(http+socks) server error: listen tcp 127.0.0.1:7890: bind: address already in use"`,
		`level=error msg="Start HTTP proxy server error: listen tcp 127.0.0.1:7890: bind: address already in use"`,
		`level=error msg="Start SOCKS proxy server error: listen tcp 127.0.0.1:7891: bind: address already in use"`,
		`level=error msg="Start DNS server error: listen udp 127.0.0.1:53: bind: address already in use"`,
	}

	for _, line := range tests {
		if err := startupFatalLineError(line); err == nil {
			t.Fatalf("expected listener failure to be fatal: %s", line)
		}
	}
}

func TestStartupFatalLineErrorIgnoresProviderFailures(t *testing.T) {
	line := `level=error msg="[Provider] example pull error: request failed"`
	if err := startupFatalLineError(line); err != nil {
		t.Fatalf("provider failure must not fail core startup: %v", err)
	}
}

func TestWaitForStartupPrefersFatalErrorAfterPostUp(t *testing.T) {
	expected := errors.New("mixed port listener failed")
	fatal := make(chan error, 1)
	fatal <- expected
	processDone := make(chan error)
	manager := &CoreManager{}
	launch := &launchSession{
		waitReady: func(context.Context) error { return nil },
	}

	err := manager.waitForStartup(launch, newBoundedOutputBuffer(1024), fatal, processDone)
	if err == nil || !strings.Contains(err.Error(), expected.Error()) {
		t.Fatalf("expected fatal listener error, got %v", err)
	}
}
