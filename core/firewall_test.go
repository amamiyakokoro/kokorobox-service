package core

import (
	"errors"
	"strings"
	"testing"
)

func TestCoreFirewallUsesStagedExecutableAcrossUpgrades(t *testing.T) {
	var paths []string
	cm := &CoreManager{ensureFirewall: func(path string) error { paths = append(paths, path); return nil }}
	for _, path := range []string{`C:\ProgramData\KokoroBox\core-runtime\oldhash\mihomo.exe`, `C:\ProgramData\KokoroBox\core-runtime\newhash\mihomo.exe`} {
		launch := &launchSession{sourcePath: `C:\App\mihomo.exe`, executablePath: path}
		cm.configureLaunchFirewall(launch)
		if paths[len(paths)-1] != path {
			t.Fatalf("wrong firewall path: %v", paths)
		}
	}
	if len(paths) != 2 {
		t.Fatalf("expected both launches to reconcile rules: %v", paths)
	}
}

func TestCoreFirewallFailureIsReportedWithoutClearingCoreState(t *testing.T) {
	expected := errors.New("policy rejected")
	cm := &CoreManager{ensureFirewall: func(string) error { return expected }}
	cm.isRunning.Store(true)
	events, unsubscribe := cm.SubscribeEvents(4)
	defer unsubscribe()
	<-events
	cm.configureLaunchFirewall(&launchSession{executablePath: "staged.exe"})
	event := <-events
	if event.Type != CoreEventLog || !strings.Contains(event.Message, "policy rejected") {
		t.Fatalf("missing warning: %+v", event)
	}
	if !cm.isRunning.Load() {
		t.Fatal("firewall failure cleared running state")
	}
	if !errors.Is(cm.ensureLaunchFirewall(&launchSession{}), expected) {
		t.Fatal("explicit repair must propagate errors")
	}
}
