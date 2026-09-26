package processrouter

import (
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"
)

func TestRestoreFailureKeepsMonitorRunning(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		configDir := filepath.Join(t.TempDir(), "config")
		// A file in place of the directory makes restoration fail before any
		// platform-specific setup, without requiring elevated privileges.
		if err := os.WriteFile(configDir, []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}
		firewall := &fakeFirewall{}
		manager := newManager(t.TempDir(), configDir, firewall)
		defer manager.Close()
		if err := manager.Restore(); err == nil {
			t.Fatal("expected configuration directory failure")
		}

		// Simulate a later activation whose client lease has expired. The
		// background monitor must reconcile it even though restoration failed.
		manager.mu.Lock()
		manager.desired = true
		manager.state = StateBlocked
		manager.lastContact = time.Now().Add(-clientLease - time.Second)
		manager.mu.Unlock()
		synctest.Wait()
		time.Sleep(monitorInterval)
		synctest.Wait()
		if state := manager.Status().State; state != StateStopped {
			t.Fatalf("monitor did not reconcile after restore failure: got %s", state)
		}
		manager.mu.Lock()
		removeCalls := firewall.removeCalls
		manager.mu.Unlock()
		if removeCalls != 1 {
			t.Fatalf("expected expired lease cleanup, got %d calls", removeCalls)
		}

		if err := manager.Close(); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		manager.mu.Lock()
		manager.state = StateBlocked
		manager.mu.Unlock()
		time.Sleep(monitorInterval)
		synctest.Wait()
		if state := manager.Status().State; state != StateBlocked {
			t.Fatalf("monitor continued reconciling after Close: got %s", state)
		}
	})
}
