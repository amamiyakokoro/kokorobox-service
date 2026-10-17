package winfirewall

import (
	"errors"
	"strings"
	"testing"
)

type memoryStore struct {
	rules                              []Rule
	writes                             int
	modErr, listErr, addErr, deleteErr error
	discardAdds                        bool
}

func (s *memoryStore) List() ([]Rule, error) { return append([]Rule(nil), s.rules...), s.listErr }
func (s *memoryStore) Modifiable() error     { return s.modErr }
func (s *memoryStore) Add(rule Rule) error {
	s.writes++
	if s.addErr != nil {
		return s.addErr
	}
	if !s.discardAdds {
		s.rules = append(s.rules, rule)
	}
	return nil
}
func (s *memoryStore) Delete(name string) error {
	s.writes++
	if s.deleteErr != nil {
		return s.deleteErr
	}
	for i, rule := range s.rules {
		if strings.EqualFold(rule.Name, name) {
			s.rules = append(s.rules[:i], s.rules[i+1:]...)
			break
		}
	}
	return nil
}

func testRules(path string) []Rule {
	return []Rule{Inbound("test TCP", "test group", path, 6, "34010"), Inbound("test UDP", "test group", path, 17, "34011")}
}

func TestReconcileCreatesRepairsAndRemovesOwnedRules(t *testing.T) {
	store := &memoryStore{rules: []Rule{Inbound("another app", "another group", "other.exe", 6, "80")}}
	desired := testRules("C:/old/core.exe")
	if err := reconcile(store, "test group", desired, Ensure, true); err != nil {
		t.Fatal(err)
	}
	if len(store.rules) != 3 {
		t.Fatalf("unexpected rules: %+v", store.rules)
	}
	writes := store.writes
	if err := reconcile(store, "test group", desired, Ensure, true); err != nil {
		t.Fatal(err)
	}
	if store.writes != writes {
		t.Fatal("healthy rules were rewritten")
	}
	desired = testRules("C:/new hash's folder/核心.exe")
	if err := reconcile(store, "test group", desired, Check, true); err == nil {
		t.Fatal("accepted stale application path")
	}
	if store.writes != writes {
		t.Fatal("check modified policy")
	}
	if err := reconcile(store, "test group", desired, Ensure, true); err != nil {
		t.Fatal(err)
	}
	if len(store.rules) != 3 {
		t.Fatal("upgrade accumulated rules")
	}
	if err := reconcile(store, "test group", desired, Check, true); err != nil {
		t.Fatal(err)
	}
	store.rules = append(store.rules, Inbound("legacy rule", "test group", "old.exe", 6, "80"))
	if err := reconcile(store, "test group", desired, Remove, true); err != nil {
		t.Fatal(err)
	}
	if len(store.rules) != 1 || store.rules[0].Group != "another group" {
		t.Fatal("removal changed unrelated rules")
	}
	if err := reconcile(store, "test group", desired, Remove, true); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileDetectsAndRepairsDrift(t *testing.T) {
	for name, damage := range map[string]func(*Rule){
		"disabled":            func(r *Rule) { r.Enabled = false },
		"blocked":             func(r *Rule) { r.Action = 0 },
		"outbound":            func(r *Rule) { r.Direction = 2 },
		"wrong port":          func(r *Rule) { r.LocalPorts = "80" },
		"remote port":         func(r *Rule) { r.RemotePorts = "80" },
		"remote address":      func(r *Rule) { r.RemoteAddresses = "127.0.0.1" },
		"local address":       func(r *Rule) { r.LocalAddresses = "127.0.0.1" },
		"wrong protocol":      func(r *Rule) { r.Protocol = 1 },
		"profile":             func(r *Rule) { r.Profiles = 1 },
		"service restriction": func(r *Rule) { r.Service = "old-service" },
		"interface type":      func(r *Rule) { r.InterfaceTypes = "Wireless" },
		"edge traversal":      func(r *Rule) { r.EdgeTraversal = true },
	} {
		t.Run(name, func(t *testing.T) {
			desired := testRules("C:/core.exe")
			store := &memoryStore{rules: append([]Rule(nil), desired...)}
			damage(&store.rules[0])
			if err := reconcile(store, "test group", desired, Check, true); err == nil {
				t.Fatal("accepted damaged rule")
			}
			if store.writes != 0 {
				t.Fatal("check modified rules")
			}
			if err := reconcile(store, "test group", desired, Ensure, true); err != nil {
				t.Fatal(err)
			}
			if len(store.rules) != 2 {
				t.Fatal("repair accumulated rules")
			}
		})
	}
}

func TestReconcileProtectsForeignRulesAndRejectsCoreDuplicates(t *testing.T) {
	desired := testRules("C:/core.exe")
	for _, mode := range []Mode{Ensure, Check, Remove} {
		store := &memoryStore{rules: []Rule{desired[1]}}
		store.rules[0].Group = "another group"
		if err := reconcile(store, "test group", desired, mode, true); err == nil {
			t.Fatal("accepted foreign reserved name")
		}
		if store.writes != 0 {
			t.Fatal("changed policy before checking all names")
		}
	}
	store := &memoryStore{rules: []Rule{desired[0], desired[0]}}
	if err := reconcile(store, "test group", desired, Ensure, false); err == nil {
		t.Fatal("accepted duplicate core rules")
	}
	if store.writes != 0 {
		t.Fatal("modified duplicate core rules")
	}
	if err := reconcile(store, "test group", desired, Ensure, true); err != nil {
		t.Fatal(err)
	}
	if len(store.rules) != 2 {
		t.Fatal("router duplicate repair failed")
	}
}

func TestReconcilePropagatesPolicyAndStoreFailures(t *testing.T) {
	failure := errors.New("store failed")
	for name, store := range map[string]*memoryStore{
		"list": {listErr: failure}, "administrator policy": {modErr: failure}, "add": {addErr: failure},
	} {
		t.Run(name, func(t *testing.T) {
			if err := reconcile(store, "test group", testRules("C:/core.exe"), Ensure, true); !errors.Is(err, failure) {
				t.Fatalf("lost error: %v", err)
			}
		})
	}
	desired := testRules("C:/core.exe")
	store := &memoryStore{rules: desired, deleteErr: failure}
	if err := reconcile(store, "test group", desired, Remove, true); !errors.Is(err, failure) {
		t.Fatalf("lost removal error: %v", err)
	}
	store = &memoryStore{discardAdds: true}
	if err := reconcile(store, "test group", desired, Ensure, true); err == nil {
		t.Fatal("reported success without persisted rules")
	}
}

func TestCheckAcceptsEquivalentWindowsRepresentations(t *testing.T) {
	desired := testRules("C:/Core.exe")
	store := &memoryStore{rules: testRules("c:/core.EXE")}
	for i := range store.rules {
		store.rules[i].Profiles = 7
		store.rules[i].Group = "TEST GROUP"
	}
	if err := reconcile(store, "test group", desired, Check, true); err != nil {
		t.Fatal(err)
	}
	if store.writes != 0 {
		t.Fatal("check modified policy")
	}
}

func TestRemoveProtectsForeignLegacyNameCollision(t *testing.T) {
	desired := testRules("C:/core.exe")
	store := &memoryStore{rules: []Rule{Inbound("legacy", "test group", "old.exe", 6, "80"), Inbound("legacy", "another group", "other.exe", 6, "80")}}
	if err := reconcile(store, "test group", desired, Remove, true); err == nil {
		t.Fatal("accepted foreign legacy name collision")
	}
	if store.writes != 0 {
		t.Fatal("removed foreign rule")
	}
}
