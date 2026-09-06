package processrouter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"
)

const (
	monitorInterval = 3 * time.Second
	probeTimeout    = 750 * time.Millisecond
	clientLease     = 20 * time.Second
)

type Manager struct {
	mu            sync.Mutex
	binaryDir     string
	binaryPath    string
	configDir     string
	configPath    string
	process       nativeProcess
	rules         RulesRequest
	desired       bool
	generation    uint64
	activePolicy  string
	state         State
	mihomoReady   bool
	lastError     string
	lastContact   time.Time
	monitorCancel context.CancelFunc
	restoreOnce   sync.Once
}

func NewManager(binaryDir, configDir string) *Manager {
	return &Manager{
		binaryDir:  binaryDir,
		binaryPath: filepath.Join(binaryDir, "kokorobox-process-router.exe"),
		configDir:  configDir,
		configPath: filepath.Join(configDir, "config.json"),
		state:      StateStopped,
		rules: RulesRequest{
			Version: ProtocolVersion, ProxyPort: ProxyPort, FailClosed: true,
		},
	}
}

func NewDefaultManager() *Manager {
	executable, _ := os.Executable()
	binaryDir := filepath.Join(filepath.Dir(executable), "process-router")
	configRoot := os.Getenv("SPARKLE_CONFIG_DIR")
	if configRoot == "" {
		if runtime.GOOS == "windows" {
			configRoot = `C:\ProgramData`
		} else {
			configRoot = os.TempDir()
		}
	}
	return NewManager(binaryDir, filepath.Join(configRoot, "sparkle", "process-router"))
}

func (m *Manager) Restore() error {
	var restoreErr error
	m.restoreOnce.Do(func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if err := os.MkdirAll(m.configDir, 0o700); err != nil {
			restoreErr = err
			return
		}
		if err := hardenProcessRouterPaths(m.binaryPath, m.configDir); err != nil && !errors.Is(err, os.ErrNotExist) {
			restoreErr = fmt.Errorf("harden process router paths: %w", err)
			return
		}
		if err := m.loadLocked(); err != nil && !errors.Is(err, os.ErrNotExist) {
			restoreErr = err
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		m.monitorCancel = cancel
		go m.monitor(ctx)
	})
	if restoreErr != nil {
		return restoreErr
	}
	if m.isDesired() {
		return m.Reconcile()
	}
	return nil
}

func (m *Manager) ReplaceRules(request RulesRequest) (Status, error) {
	normalized, err := normalizeRulesRequest(request)
	if err != nil {
		return m.Status(), &ValidationError{Err: err}
	}
	m.mu.Lock()
	m.rules = normalized
	m.lastContact = time.Now()
	m.generation++
	m.activePolicy = ""
	err = m.persistLocked()
	desired := m.desired
	m.mu.Unlock()
	if err != nil {
		return m.Status(), err
	}
	if desired {
		err = m.Reconcile()
	}
	return m.Status(), err
}

func (m *Manager) Enable() (Status, error) {
	if !Supported() {
		return m.Status(), ErrUnsupported
	}
	m.mu.Lock()
	if !hasEnabledRules(m.rules) {
		m.mu.Unlock()
		return m.Status(), &ValidationError{Err: errors.New("cannot start process router without an enabled rule")}
	}
	m.desired = true
	m.lastContact = time.Now()
	err := m.persistLocked()
	m.mu.Unlock()
	if err == nil {
		err = m.Reconcile()
	}
	return m.Status(), err
}

func (m *Manager) Disable() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.desired = false
	m.activePolicy = ""
	m.state = StateStopped
	m.mihomoReady = false
	m.lastError = ""
	persistErr := m.persistLocked()
	stopErr := m.stopProcessLocked()
	return errors.Join(persistErr, stopErr)
}

func (m *Manager) Cleanup() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.desired = false
	m.rules = RulesRequest{Version: ProtocolVersion, ProxyPort: ProxyPort, FailClosed: true}
	m.generation++
	m.activePolicy = ""
	m.state = StateStopped
	m.mihomoReady = false
	m.lastError = ""
	stopErr := m.stopProcessLocked()
	removeErr := os.Remove(m.configPath)
	if errors.Is(removeErr, os.ErrNotExist) {
		removeErr = nil
	}
	return errors.Join(stopErr, removeErr)
}

func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.monitorCancel != nil {
		m.monitorCancel()
		m.monitorCancel = nil
	}
	return m.stopProcessLocked()
}

func (m *Manager) Reconcile() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.desired {
		return nil
	}
	if !m.leaseActiveLocked() {
		m.state = StateStopped
		m.mihomoReady = false
		m.lastError = ""
		return m.stopProcessLocked()
	}
	if !Supported() {
		m.setErrorLocked(ErrUnsupported)
		return ErrUnsupported
	}
	if m.process != nil && !m.process.Alive() {
		m.process = nil
		m.activePolicy = ""
	}
	if m.process == nil {
		m.state = StateStarting
		if err := verifyProcessRouterIntegrity(m.binaryDir); err != nil {
			m.setErrorLocked(err)
			return err
		}
		process, err := startNativeProcess(m.binaryPath, m.binaryDir)
		if err != nil {
			m.setErrorLocked(err)
			return err
		}
		m.process = process
		m.activePolicy = ""
	}

	requiresProxy := hasProxyRules(m.rules)
	available := !requiresProxy || probeMihomo(ProxyPort)
	policy := strconv.FormatUint(m.generation, 10) + ":" + strconv.FormatBool(available)
	if policy != m.activePolicy {
		if err := sendRules(m.process, buildRouterCommand(m.rules, available)); err != nil {
			m.setErrorLocked(err)
			return err
		}
		m.activePolicy = policy
	}
	m.mihomoReady = available && requiresProxy
	m.lastError = ""
	if requiresProxy && !available {
		m.state = StateBlocked
	} else {
		m.state = StateRunning
	}
	return nil
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	status := Status{
		Version:                   ProtocolVersion,
		Supported:                 Supported(),
		State:                     m.state,
		Generation:                m.generation,
		MihomoAvailable:           m.mihomoReady,
		ProtectedApplicationCount: protectedRuleCount(m.rules),
		LastError:                 m.lastError,
	}
	if hasProxyRules(m.rules) {
		status.ProxyPort = ProxyPort
	}
	if m.process != nil && m.process.Alive() {
		status.RouterPID = m.process.PID()
	}
	return status
}

func (m *Manager) RenewLease() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.desired {
		m.lastContact = time.Now()
	}
}

func Supported() bool {
	return runtime.GOOS == "windows" && runtime.GOARCH == "amd64"
}

func (m *Manager) monitor(ctx context.Context) {
	ticker := time.NewTicker(monitorInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = m.Reconcile()
		}
	}
}

func (m *Manager) stopProcessLocked() error {
	if m.process == nil {
		return nil
	}
	process := m.process
	m.process = nil
	m.activePolicy = ""
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return process.Stop(ctx)
}

func (m *Manager) setErrorLocked(err error) {
	m.state = StateError
	m.mihomoReady = false
	m.lastError = err.Error()
}

func (m *Manager) leaseActiveLocked() bool {
	return !m.lastContact.IsZero() && time.Since(m.lastContact) <= clientLease
}

func (m *Manager) isDesired() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.desired
}

func (m *Manager) persistLocked() error {
	if err := os.MkdirAll(m.configDir, 0o700); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(persistedConfig{
		Version: ProtocolVersion,
		Enabled: m.desired,
		Rules:   m.rules,
	}, "", "  ")
	if err != nil {
		return err
	}
	temporary := m.configPath + ".tmp"
	if err := os.WriteFile(temporary, payload, 0o600); err != nil {
		return err
	}
	if err := os.Rename(temporary, m.configPath); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func (m *Manager) loadLocked() error {
	payload, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}
	var persisted persistedConfig
	if err := json.Unmarshal(payload, &persisted); err != nil {
		return fmt.Errorf("decode process router configuration: %w", err)
	}
	if persisted.Version != ProtocolVersion {
		return fmt.Errorf("unsupported persisted process router version: %d", persisted.Version)
	}
	rules, err := normalizeRulesRequest(persisted.Rules)
	if err != nil {
		return fmt.Errorf("validate persisted process router configuration: %w", err)
	}
	m.rules = rules
	m.desired = persisted.Enabled
	m.generation++
	return nil
}

func probeMihomo(port int) bool {
	connection, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), probeTimeout)
	if err != nil {
		return false
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(probeTimeout)); err != nil {
		return false
	}
	if _, err := connection.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		return false
	}
	response := make([]byte, 2)
	if _, err := io.ReadFull(connection, response); err != nil {
		return false
	}
	return response[0] == 0x05 && response[1] == 0x00
}
