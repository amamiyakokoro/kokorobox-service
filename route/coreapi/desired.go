package coreapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	corepkg "github.com/amamiyakokoro/kokorobox-service/core"
	"github.com/amamiyakokoro/kokorobox-service/log"
)

const coreReconcileInterval = 10 * time.Second

type desiredCoreState struct {
	Version  int     `json:"version"`
	Running  bool    `json:"running"`
	LogGroup *uint32 `json:"log_group,omitempty"`
}

type desiredCoreManager struct {
	mu       sync.Mutex
	path     string
	running  bool
	logGroup *uint32
	core     coreLifecycle
	stop     chan struct{}
}

type coreLifecycle interface {
	GetProcessInfo() (*corepkg.ProcessInfo, error)
	StartCoreWithProfile(*corepkg.LaunchProfile, ...corepkg.LaunchOption) error
}

var desiredCore desiredCoreManager

func ConfigureDesiredCore(dataDir string) error {
	initCoreManager()
	desiredCore.mu.Lock()
	defer desiredCore.mu.Unlock()
	if desiredCore.stop != nil {
		close(desiredCore.stop)
		desiredCore.stop = nil
	}
	desiredCore.path = filepath.Join(dataDir, "core", "desired_state.json")
	desiredCore.core = cm
	if err := desiredCore.load(); err != nil {
		return err
	}
	desiredCore.stop = make(chan struct{})
	go desiredCore.watch(desiredCore.stop)
	if desiredCore.running {
		go desiredCore.reconcile()
	}
	return nil
}

// load is called with m.mu held during service startup.
func (m *desiredCoreManager) load() error {
	m.running = false
	m.logGroup = nil
	data, err := os.ReadFile(m.path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read desired core state: %w", err)
	}
	if err == nil {
		var state desiredCoreState
		if err := json.Unmarshal(data, &state); err != nil || state.Version != 1 {
			return errors.New("invalid desired core state")
		}
		m.running = state.Running
		m.logGroup = state.LogGroup
	}
	return nil
}

func (m *desiredCoreManager) set(running bool, logGroup *uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.path == "" {
		return errors.New("desired core state is not configured")
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
		return err
	}
	if running && logGroup == nil {
		logGroup = m.logGroup
	}
	data, err := json.Marshal(desiredCoreState{Version: 1, Running: running, LogGroup: logGroup})
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(m.path), ".desired-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), m.path); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		m.running = running
		m.logGroup = logGroup
		return nil
	}
	dir, err := os.Open(filepath.Dir(m.path))
	if err != nil {
		return err
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return err
	}
	m.running = running
	m.logGroup = logGroup
	return nil
}

func (m *desiredCoreManager) desired() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

func (m *desiredCoreManager) watch(stop <-chan struct{}) {
	ticker := time.NewTicker(coreReconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			m.reconcile()
		}
	}
}

func (m *desiredCoreManager) reconcile() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running || m.core == nil {
		return
	}
	if _, err := m.core.GetProcessInfo(); err == nil {
		return
	}
	var options []corepkg.LaunchOption
	if m.logGroup != nil {
		options = append(options, corepkg.WithLogFileGroup(*m.logGroup))
	}
	if err := m.core.StartCoreWithProfile(nil, options...); err != nil {
		log.Printf("Failed to reconcile desired core state: %v", err)
	}
}

func (m *desiredCoreManager) stopWatching() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stop != nil {
		close(m.stop)
		m.stop = nil
	}
}

func startDesiredCore(profile *corepkg.LaunchProfile, logGroup *uint32, options ...corepkg.LaunchOption) error {
	if err := desiredCore.set(true, logGroup); err != nil {
		return err
	}
	desiredCore.mu.Lock()
	defer desiredCore.mu.Unlock()
	if _, err := cm.GetProcessInfo(); err == nil {
		return nil
	}
	return cm.StartCoreWithProfile(profile, options...)
}
