//go:build windows

package core

import (
	"errors"
	"testing"
)

func TestRepairCoreFirewallTargetsCurrentLaunchAndPropagatesFailure(t *testing.T) {
	failure := errors.New("firewall policy denied")
	calls := 0
	cm := &CoreManager{ensureFirewall: func(path string) error {
		calls++
		if path != "staged.exe" {
			t.Fatalf("used source path: %s", path)
		}
		return failure
	}}
	if err := cm.RepairCoreFirewall(); err == nil {
		t.Fatal("accepted repair without running core")
	}
	if calls != 0 {
		t.Fatal("touched firewall without launch")
	}
	cm.launch = &launchSession{sourcePath: "source.exe", executablePath: "staged.exe"}
	cm.isRunning.Store(true)
	if err := cm.RepairCoreFirewall(); !errors.Is(err, failure) {
		t.Fatalf("swallowed failure: %v", err)
	}
	if calls != 1 {
		t.Fatalf("repair count = %d", calls)
	}
}
