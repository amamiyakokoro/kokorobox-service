package dnsapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/amamiyakokoro/kokorobox-service/log"
	"github.com/amamiyakokoro/kokorobox-service/sys"
)

const leaseDuration = 60 * time.Second
const reconcileInterval = 5 * time.Second
const retryInterval = 15 * time.Second
const maxRecordSize = 16 << 10

type backend interface {
	ActiveDNSService() (sys.DNSService, error)
	GetDns(string) ([]string, error)
	SetDns(string, []string) error
}

type systemBackend struct{}

func (systemBackend) ActiveDNSService() (sys.DNSService, error)  { return sys.ActiveDNSService() }
func (systemBackend) GetDns(name string) ([]string, error)       { return sys.GetDns(name) }
func (systemBackend) SetDns(name string, servers []string) error { return sys.SetDns(name, servers) }

type entry struct {
	Service  sys.DNSService `json:"service"`
	Original []string       `json:"original"`
	Desired  []string       `json:"desired"`
	Applied  bool           `json:"applied"`
}

type lease struct {
	Version  int       `json:"version"`
	Deadline time.Time `json:"deadline"`
	Entries  []entry   `json:"entries"`
}

type manager struct {
	mu      sync.Mutex
	backend backend
	path    string
	lease   *lease
	timer   *time.Timer
	watch   *time.Ticker
	stop    chan struct{}
}

var global = &manager{backend: systemBackend{}}

func Configure(dataDir string) error {
	global.mu.Lock()
	defer global.mu.Unlock()
	global.path = filepath.Join(dataDir, "dns", "lease.json")
	dir := filepath.Dir(global.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	return global.recover()
}

func Stop() error {
	global.mu.Lock()
	defer global.mu.Unlock()
	global.stopTimers()
	return global.release()
}

func validateServers(servers []string) error {
	if len(servers) == 0 || len(servers) > 4 {
		return errors.New("DNS lease requires one to four servers")
	}
	for _, server := range servers {
		if net.ParseIP(server) == nil {
			return fmt.Errorf("invalid DNS server address: %q", server)
		}
	}
	return nil
}

func (m *manager) set(servers []string) error {
	if m.path == "" {
		return errors.New("DNS lease recovery is not configured")
	}
	if m.lease != nil {
		if reflect.DeepEqual(m.lease.Entries[0].Desired, servers) {
			return m.renew()
		}
		if err := m.release(); err != nil {
			return err
		}
	}
	service, err := m.backend.ActiveDNSService()
	if err != nil {
		return err
	}
	original, err := m.backend.GetDns(service.Name)
	if err != nil {
		return err
	}
	m.lease = &lease{
		Version: 1, Deadline: time.Now().Add(leaseDuration),
		Entries: []entry{{Service: service, Original: original, Desired: append([]string(nil), servers...)}},
	}
	// Persist before mutation so a crash at any point can restore the original.
	if err := m.persist(); err != nil {
		m.lease = nil
		return err
	}
	if err := m.backend.SetDns(service.Name, servers); err != nil {
		_ = m.release()
		return err
	}
	m.lease.Entries[0].Applied = true
	m.schedule()
	return m.persist()
}

func (m *manager) renew() error {
	if m.lease == nil {
		return errors.New("No service-owned DNS lease to renew")
	}
	m.lease.Deadline = time.Now().Add(leaseDuration)
	if err := m.persist(); err != nil {
		return err
	}
	m.schedule()
	return nil
}

func (m *manager) schedule() {
	if m.timer != nil {
		m.timer.Stop()
	}
	m.timer = time.AfterFunc(time.Until(m.lease.Deadline), m.expire)
	if m.watch == nil {
		m.watch = time.NewTicker(reconcileInterval)
		m.stop = make(chan struct{})
		go m.watchNetwork(m.watch, m.stop)
	}
}

func (m *manager) expire() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.lease == nil {
		return
	}
	if remaining := time.Until(m.lease.Deadline); remaining > 0 {
		m.timer = time.AfterFunc(remaining, m.expire)
		return
	}
	if err := m.release(); err != nil {
		log.Printf("Failed to release expired DNS lease: %v", err)
		m.timer = time.AfterFunc(retryInterval, m.expire)
	}
}

func (m *manager) watchNetwork(ticker *time.Ticker, stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			m.mu.Lock()
			if m.lease != nil {
				if err := m.reconcile(); err != nil {
					log.Printf("Failed to reconcile DNS lease: %v", err)
				}
			}
			m.mu.Unlock()
		}
	}
}

func (m *manager) reconcile() error {
	active, err := m.backend.ActiveDNSService()
	if err != nil {
		return err
	}
	var currentEntry *entry
	changed := false
	for i := range m.lease.Entries {
		if m.lease.Entries[i].Service.ID == active.ID {
			currentEntry = &m.lease.Entries[i]
			break
		}
	}
	if currentEntry == nil {
		if len(m.lease.Entries) >= 8 {
			return errors.New("too many DNS services awaiting restoration")
		}
		original, err := m.backend.GetDns(active.Name)
		if err != nil {
			return err
		}
		desired := append([]string(nil), m.lease.Entries[0].Desired...)
		m.lease.Entries = append(m.lease.Entries, entry{Service: active, Original: original, Desired: desired})
		if err := m.persist(); err != nil {
			m.lease.Entries = m.lease.Entries[:len(m.lease.Entries)-1]
			return err
		}
		currentEntry = &m.lease.Entries[len(m.lease.Entries)-1]
		changed = true
	}
	current, err := m.backend.GetDns(currentEntry.Service.Name)
	if err != nil {
		return err
	}
	if !currentEntry.Applied && reflect.DeepEqual(current, currentEntry.Original) && !reflect.DeepEqual(current, currentEntry.Desired) {
		if err := m.backend.SetDns(currentEntry.Service.Name, currentEntry.Desired); err != nil {
			return err
		}
		currentEntry.Applied = true
		changed = true
	}
	remaining := m.lease.Entries[:0]
	for _, item := range m.lease.Entries {
		if item.Service.ID == active.ID {
			remaining = append(remaining, item)
			continue
		}
		if err := m.restore(item); err != nil {
			remaining = append(remaining, item)
			log.Printf("Failed to restore DNS for %s: %v", item.Service.Name, err)
		} else {
			changed = true
		}
	}
	m.lease.Entries = remaining
	if changed {
		return m.persist()
	}
	return nil
}

func (m *manager) restore(item entry) error {
	current, err := m.backend.GetDns(item.Service.Name)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(current, item.Desired) || reflect.DeepEqual(current, item.Original) {
		return nil
	}
	return m.backend.SetDns(item.Service.Name, item.Original)
}

func (m *manager) release() error {
	if m.lease == nil {
		return nil
	}
	remaining := m.lease.Entries[:0]
	for _, item := range m.lease.Entries {
		if err := m.restore(item); err != nil {
			remaining = append(remaining, item)
		}
	}
	if len(remaining) > 0 {
		m.lease.Entries = remaining
		_ = m.persist()
		return fmt.Errorf("failed to restore %d DNS services", len(remaining))
	}
	m.lease = nil
	m.stopTimers()
	return m.removeRecord()
}

func (m *manager) stopTimers() {
	if m.timer != nil {
		m.timer.Stop()
		m.timer = nil
	}
	if m.watch != nil {
		m.watch.Stop()
		close(m.stop)
		m.watch = nil
		m.stop = nil
	}
}

func (m *manager) persist() error {
	data, err := json.Marshal(m.lease)
	if err != nil {
		return err
	}
	if len(data) > maxRecordSize {
		return errors.New("DNS lease record is too large")
	}
	tmp, err := os.CreateTemp(filepath.Dir(m.path), ".lease-*")
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
	return syncDirectory(filepath.Dir(m.path))
}

func (m *manager) recover() error {
	file, err := os.Open(m.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxRecordSize+1))
	if err != nil {
		return err
	}
	if len(data) > maxRecordSize {
		return errors.New("DNS lease record is too large")
	}
	var saved lease
	if err := json.Unmarshal(data, &saved); err != nil {
		return err
	}
	if saved.Version != 1 || len(saved.Entries) == 0 || len(saved.Entries) > 8 {
		return errors.New("invalid DNS lease record")
	}
	for _, item := range saved.Entries {
		if item.Service.ID == "" || item.Service.Name == "" || validateServers(item.Desired) != nil {
			return errors.New("invalid DNS lease record entry")
		}
		for _, server := range item.Original {
			if net.ParseIP(server) == nil {
				return errors.New("invalid original DNS server in lease record")
			}
		}
	}
	m.lease = &saved
	if err := m.release(); err != nil {
		m.timer = time.AfterFunc(retryInterval, m.expire)
		return err
	}
	return nil
}

func (m *manager) removeRecord() error {
	err := os.Remove(m.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(m.path))
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
