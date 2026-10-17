//go:build windows

package winfirewall

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

var policyMu sync.Mutex

// Apply calls INetFwPolicy2/INetFwRules directly. COM interfaces never escape
// their initialized OS thread, and concurrent repairs are serialized.
func Apply(group string, desired []Rule, mode Mode, repairDuplicates bool) error {
	policyMu.Lock()
	defer policyMu.Unlock()
	return withStore(group, desired, func(store *comStore) error {
		return reconcile(store, group, desired, mode, repairDuplicates)
	})
}

func withCOM(run func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
	var oleErr *ole.OleError
	// S_FALSE still increments COM's initialization count and needs cleanup.
	if err != nil && !(errors.As(err, &oleErr) && oleErr.Code() == 1) {
		return fmt.Errorf("initialize firewall COM: %w", err)
	}
	defer ole.CoUninitialize()
	return run()
}

type comStore struct {
	policy, rules *ole.IDispatch
	group         string
	desired       []Rule
}

func createDispatch(programID string) (*ole.IDispatch, error) {
	unknown, err := oleutil.CreateObject(programID)
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", programID, err)
	}
	defer unknown.Release()
	return unknown.QueryInterface(ole.IID_IDispatch)
}

func withStore(group string, desired []Rule, run func(*comStore) error) error {
	return withCOM(func() error {
		policy, err := createDispatch("HNetCfg.FwPolicy2")
		if err != nil {
			return err
		}
		defer policy.Release()
		rules, err := oleutil.GetProperty(policy, "Rules")
		if rules != nil {
			defer rules.Clear()
		}
		if err != nil {
			return fmt.Errorf("get firewall rules: %w", err)
		}
		if rules.ToIDispatch() == nil {
			return fmt.Errorf("invalid firewall rules collection")
		}
		return run(&comStore{policy: policy, rules: rules.ToIDispatch(), group: group, desired: desired})
	})
}

func getProperty[T string | int32 | bool](object *ole.IDispatch, name string) (T, error) {
	var zero T
	value, err := oleutil.GetProperty(object, name)
	if value != nil {
		defer value.Clear()
	}
	if err != nil {
		return zero, fmt.Errorf("get firewall %s: %w", name, err)
	}
	result, ok := value.Value().(T)
	if !ok {
		return zero, fmt.Errorf("invalid firewall %s property type", name)
	}
	return result, nil
}

func (s *comStore) Modifiable() error {
	state, err := getProperty[int32](s.policy, "LocalPolicyModifyState")
	if err != nil {
		return err
	}
	if state != 0 {
		return fmt.Errorf("Windows Firewall policy prevents local inbound exceptions; check administrator policy")
	}
	return nil
}

func readRule(object *ole.IDispatch, full bool) (Rule, error) {
	var rule Rule
	stringsToRead := []struct {
		name   string
		target *string
	}{
		{"Name", &rule.Name}, {"Grouping", &rule.Group},
	}
	if full {
		stringsToRead = append(stringsToRead, []struct {
			name   string
			target *string
		}{
			{"Description", &rule.Description}, {"ApplicationName", &rule.Application}, {"ServiceName", &rule.Service},
			{"LocalPorts", &rule.LocalPorts}, {"RemotePorts", &rule.RemotePorts},
			{"LocalAddresses", &rule.LocalAddresses}, {"RemoteAddresses", &rule.RemoteAddresses}, {"InterfaceTypes", &rule.InterfaceTypes},
		}...)
	}
	for _, property := range stringsToRead {
		value, err := getProperty[string](object, property.name)
		if err != nil {
			return rule, err
		}
		*property.target = value
	}
	if !full {
		return rule, nil
	}
	for _, property := range []struct {
		name   string
		target *int32
	}{
		{"Protocol", &rule.Protocol}, {"Direction", &rule.Direction}, {"Action", &rule.Action}, {"Profiles", &rule.Profiles},
	} {
		value, err := getProperty[int32](object, property.name)
		if err != nil {
			return rule, err
		}
		*property.target = value
	}
	for _, property := range []struct {
		name   string
		target *bool
	}{{"Enabled", &rule.Enabled}, {"EdgeTraversal", &rule.EdgeTraversal}} {
		value, err := getProperty[bool](object, property.name)
		if err != nil {
			return rule, err
		}
		*property.target = value
	}
	return rule, nil
}

func (s *comStore) List() ([]Rule, error) {
	value, err := oleutil.GetProperty(s.rules, "_NewEnum")
	if value != nil {
		defer value.Clear()
	}
	if err != nil {
		return nil, fmt.Errorf("get firewall rule enumerator: %w", err)
	}
	if value.ToIUnknown() == nil {
		return nil, fmt.Errorf("invalid firewall rule enumerator")
	}
	enumerator, err := value.ToIUnknown().IEnumVARIANT(ole.IID_IEnumVariant)
	if err != nil {
		return nil, fmt.Errorf("query firewall rule enumerator: %w", err)
	}
	defer enumerator.Release()
	var result []Rule
	for {
		item, count, err := enumerator.Next(1)
		var oleErr *ole.OleError
		if err != nil && !(errors.As(err, &oleErr) && oleErr.Code() == 1 && count == 0) {
			_ = item.Clear()
			return nil, fmt.Errorf("enumerate firewall rules: %w", err)
		}
		if count == 0 {
			_ = item.Clear()
			break
		}
		rule, readErr := s.readItem(&item)
		_ = item.Clear()
		if readErr != nil {
			return nil, readErr
		}
		if rule != nil {
			result = append(result, *rule)
		}
	}
	return result, nil
}

func (s *comStore) readItem(item *ole.VARIANT) (*Rule, error) {
	object := item.ToIDispatch()
	if object == nil {
		return nil, fmt.Errorf("invalid firewall rule object")
	}
	identity, err := readRule(object, false)
	if err != nil {
		return nil, err
	}
	owned := strings.EqualFold(identity.Group, s.group)
	reserved := false
	for _, want := range s.desired {
		reserved = reserved || strings.EqualFold(identity.Name, want.Name)
	}

	// Unrelated rules (including ICMP rules) need no TCP/UDP port properties.
	// Non-reserved group members only need their identity for removal.
	if !owned || !reserved {
		return &identity, nil
	}
	protocol, err := getProperty[int32](object, "Protocol")
	if err != nil {
		return nil, err
	}
	if protocol != 6 && protocol != 17 {
		identity.Protocol = protocol
		return &identity, nil
	}
	rule, err := readRule(object, true)
	return &rule, err
}

func putProperty(object *ole.IDispatch, name string, value any) error {
	result, err := oleutil.PutProperty(object, name, value)
	if result != nil {
		_ = result.Clear()
	}
	if err != nil {
		return fmt.Errorf("set firewall %s: %w", name, err)
	}
	return nil
}

func newRule(spec Rule) (*ole.IDispatch, error) {
	rule, err := createDispatch("HNetCfg.FWRule")
	if err != nil {
		return nil, err
	}
	// Protocol precedes port setters. ServiceName is deliberately unset:
	// assigning even an empty string can make INetFwRules.Add fail.
	for _, property := range []struct {
		name  string
		value any
	}{
		{"Name", spec.Name}, {"Grouping", spec.Group}, {"Description", spec.Description},
		{"ApplicationName", spec.Application}, {"Protocol", spec.Protocol},
		{"LocalPorts", spec.LocalPorts}, {"RemotePorts", spec.RemotePorts},
		{"LocalAddresses", spec.LocalAddresses}, {"RemoteAddresses", spec.RemoteAddresses},
		{"Direction", spec.Direction}, {"Action", spec.Action}, {"Profiles", spec.Profiles},
		{"InterfaceTypes", spec.InterfaceTypes}, {"EdgeTraversal", spec.EdgeTraversal}, {"Enabled", spec.Enabled},
	} {
		if err := putProperty(rule, property.name, property.value); err != nil {
			rule.Release()
			return nil, err
		}
	}
	return rule, nil
}

func (s *comStore) Add(spec Rule) error {
	rule, err := newRule(spec)
	if err != nil {
		return err
	}
	defer rule.Release()
	result, err := oleutil.CallMethod(s.rules, "Add", rule)
	if result != nil {
		_ = result.Clear()
	}
	if err != nil {
		return fmt.Errorf("add firewall rule %q: %w", spec.Name, err)
	}
	return nil
}

func (s *comStore) Delete(name string) error {
	result, err := oleutil.CallMethod(s.rules, "Remove", name)
	if result != nil {
		_ = result.Clear()
	}
	if err != nil {
		return fmt.Errorf("remove firewall rule %q: %w", name, err)
	}
	return nil
}
