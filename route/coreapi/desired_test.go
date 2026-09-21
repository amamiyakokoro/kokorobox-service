package coreapi

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	corepkg "github.com/amamiyakokoro/kokorobox-service/core"
)

type fakeCoreLifecycle struct {
	running  bool
	attempts int
	fail     bool
}

func (f *fakeCoreLifecycle) GetProcessInfo() (*corepkg.ProcessInfo, error) {
	if !f.running {
		return nil, errors.New("not running")
	}
	return &corepkg.ProcessInfo{PID: 1}, nil
}

func (f *fakeCoreLifecycle) StartCoreWithProfile(_ *corepkg.LaunchProfile, _ ...corepkg.LaunchOption) error {
	f.attempts++
	if f.fail {
		return errors.New("start failed")
	}
	f.running = true
	return nil
}

func TestDesiredCorePersistsIntentAndRetriesAfterFailure(t *testing.T) {
	core := &fakeCoreLifecycle{fail: true}
	m := &desiredCoreManager{path: filepath.Join(t.TempDir(), "core", "desired_state.json"), core: core}
	group := uint32(501)
	if err := m.set(true, &group); err != nil {
		t.Fatal(err)
	}
	var saved desiredCoreState
	data, err := os.ReadFile(m.path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &saved); err != nil || !saved.Running || saved.LogGroup == nil || *saved.LogGroup != group {
		t.Fatalf("desired state was not saved before launch: %+v, %v", saved, err)
	}
	restarted := &desiredCoreManager{path: m.path, core: core}
	if err := restarted.load(); err != nil || !restarted.running || restarted.logGroup == nil || *restarted.logGroup != group {
		t.Fatalf("service restart lost desired state: %+v, %v", restarted, err)
	}
	m.reconcile()
	if core.attempts != 1 || !m.desired() {
		t.Fatal("a failed launch cleared the desired running state")
	}
	core.fail = false
	m.reconcile()
	m.reconcile()
	if core.attempts != 2 || !core.running {
		t.Fatalf("reconciliation did not retry exactly once: %+v", core)
	}
	if err := m.set(false, nil); err != nil {
		t.Fatal(err)
	}
	core.running = false
	m.reconcile()
	if core.attempts != 2 {
		t.Fatal("reconciliation restarted a stopped core")
	}
}
