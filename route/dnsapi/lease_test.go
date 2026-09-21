package dnsapi

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/amamiyakokoro/kokorobox-service/sys"
)

type fakeBackend struct {
	active    sys.DNSService
	values    map[string][]string
	beforeSet func()
}

func (f *fakeBackend) ActiveDNSService() (sys.DNSService, error) { return f.active, nil }
func (f *fakeBackend) GetDns(name string) ([]string, error) {
	return append([]string(nil), f.values[name]...), nil
}
func (f *fakeBackend) SetDns(name string, servers []string) error {
	if f.beforeSet != nil {
		f.beforeSet()
	}
	f.values[name] = append([]string(nil), servers...)
	return nil
}

func newTestManager(t *testing.T) (*manager, *fakeBackend) {
	t.Helper()
	f := &fakeBackend{
		active: sys.DNSService{ID: "one", Name: "Wi-Fi"},
		values: map[string][]string{"Wi-Fi": {"192.0.2.53"}, "Ethernet": {"198.51.100.53"}},
	}
	m := &manager{backend: f, path: filepath.Join(t.TempDir(), "lease.json")}
	t.Cleanup(m.stopTimers)
	return m, f
}

func TestDNSLeasePersistsBeforeMutationAndRestoresAfterRestart(t *testing.T) {
	m, f := newTestManager(t)
	f.beforeSet = func() {
		if _, err := os.Stat(m.path); err != nil {
			t.Fatalf("DNS changed before durable record: %v", err)
		}
		f.beforeSet = nil
	}
	if err := m.set([]string{"223.5.5.5"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.values["Wi-Fi"], []string{"223.5.5.5"}) {
		t.Fatal(f.values)
	}
	m.stopTimers() // Simulate the process ending without graceful cleanup.
	restarted := &manager{backend: f, path: m.path}
	if err := restarted.recover(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.values["Wi-Fi"], []string{"192.0.2.53"}) {
		t.Fatal(f.values)
	}
	if _, err := os.Stat(m.path); !os.IsNotExist(err) {
		t.Fatalf("recovery record remains: %v", err)
	}
}

func TestDNSLeaseReconcilesNetworkChangeAndLeavesUserEdit(t *testing.T) {
	m, f := newTestManager(t)
	if err := m.set([]string{"223.5.5.5"}); err != nil {
		t.Fatal(err)
	}
	f.active = sys.DNSService{ID: "two", Name: "Ethernet"}
	if err := m.reconcile(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.values["Wi-Fi"], []string{"192.0.2.53"}) || !reflect.DeepEqual(f.values["Ethernet"], []string{"223.5.5.5"}) {
		t.Fatal(f.values)
	}
	f.values["Ethernet"] = []string{"1.1.1.1"}
	if err := m.reconcile(); err != nil {
		t.Fatal(err)
	}
	if err := m.release(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.values["Ethernet"], []string{"1.1.1.1"}) {
		t.Fatal("user DNS edit was overwritten")
	}
}

func TestDNSLeaseExpiryRestoresOriginal(t *testing.T) {
	m, f := newTestManager(t)
	if err := m.set([]string{"223.5.5.5"}); err != nil {
		t.Fatal(err)
	}
	m.lease.Deadline = time.Now().Add(-time.Second)
	m.expire()
	if m.lease != nil || !reflect.DeepEqual(f.values["Wi-Fi"], []string{"192.0.2.53"}) {
		t.Fatal("expired DNS lease was not restored")
	}
}
