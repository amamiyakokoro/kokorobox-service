// Package winfirewall manages the application's Windows Firewall rules.
package winfirewall

import (
	"fmt"
	"strings"
)

const AllProfiles int32 = 0x7fffffff

type Mode int

const (
	Ensure Mode = iota
	Check
	Remove
)

// Rule describes the complete scope of an application inbound exception.
type Rule struct {
	Name, Group, Description, Application, Service           string
	Protocol                                                 int32
	LocalPorts, RemotePorts, LocalAddresses, RemoteAddresses string
	Direction, Action, Profiles                              int32
	InterfaceTypes                                           string
	EdgeTraversal, Enabled                                   bool
}

func Inbound(name, group, application string, protocol int32, ports string) Rule {
	return Rule{
		Name: name, Group: group, Application: application, Protocol: protocol,
		LocalPorts: ports, RemotePorts: "*", LocalAddresses: "*", RemoteAddresses: "*",
		Direction: 1, Action: 1, Profiles: AllProfiles, InterfaceTypes: "All", Enabled: true,
	}
}

type ruleStore interface {
	List() ([]Rule, error)
	Modifiable() error
	Add(Rule) error
	Delete(string) error
}

func sameRule(a, b Rule) bool {
	// Windows paths and rule identities are case insensitive.
	a.Name, b.Name = strings.ToLower(a.Name), strings.ToLower(b.Name)
	a.Group, b.Group = strings.ToLower(a.Group), strings.ToLower(b.Group)
	a.Application, b.Application = strings.ToLower(a.Application), strings.ToLower(b.Application)
	a.InterfaceTypes, b.InterfaceTypes = strings.ToLower(a.InterfaceTypes), strings.ToLower(b.InterfaceTypes)
	// Both representations mean all currently supported profiles.
	if a.Profiles&7 == 7 {
		a.Profiles = AllProfiles
	}
	if b.Profiles&7 == 7 {
		b.Profiles = AllProfiles
	}
	return a == b
}

func matching(rules []Rule, name string) []Rule {
	var result []Rule
	for _, rule := range rules {
		if strings.EqualFold(rule.Name, name) {
			result = append(result, rule)
		}
	}
	return result
}

func reconcile(store ruleStore, group string, desired []Rule, mode Mode, repairDuplicates bool) error {
	if mode != Ensure && mode != Check && mode != Remove {
		return fmt.Errorf("invalid firewall operation")
	}
	rules, err := store.List()
	if err != nil {
		return err
	}
	// Preflight every reserved name before changing any rule. Never delete or
	// overwrite a rule belonging to another application, even during repair.
	for _, want := range desired {
		found := matching(rules, want.Name)
		for _, rule := range found {
			if !strings.EqualFold(rule.Group, group) {
				return fmt.Errorf("firewall rule %q belongs to another group", rule.Name)
			}
		}
		if len(found) > 1 && !repairDuplicates {
			return fmt.Errorf("duplicate reserved firewall rule %q", want.Name)
		}
	}
	// Removing a legacy group member must also preserve foreign rules sharing its name.
	if mode == Remove {
		for _, rule := range rules {
			if !strings.EqualFold(rule.Group, group) {
				continue
			}
			for _, other := range matching(rules, rule.Name) {
				if !strings.EqualFold(other.Group, group) {
					return fmt.Errorf("firewall rule %q belongs to another group", rule.Name)
				}
			}
		}
	}
	if mode != Check {
		if err := store.Modifiable(); err != nil {
			return err
		}
	}
	if mode == Remove {
		for _, rule := range rules {
			if strings.EqualFold(rule.Group, group) {
				if err := store.Delete(rule.Name); err != nil {
					return err
				}
			}
		}
		remaining, err := store.List()
		if err != nil {
			return err
		}
		for _, rule := range remaining {
			if strings.EqualFold(rule.Group, group) {
				return fmt.Errorf("firewall rule %q was not removed", rule.Name)
			}
		}
		return nil
	}
	for _, want := range desired {
		found := matching(rules, want.Name)
		if len(found) == 1 && sameRule(found[0], want) {
			continue
		}
		if mode == Check {
			return fmt.Errorf("firewall rule %q is missing or invalid", want.Name)
		}
		// Recreate stale rules so obsolete service/interface restrictions cannot
		// survive an upgrade. Healthy rules are left untouched.
		for _, rule := range found {
			if err := store.Delete(rule.Name); err != nil {
				return err
			}
		}
		if err := store.Add(want); err != nil {
			return err
		}
	}
	actual, err := store.List()
	if err != nil {
		return err
	}
	for _, want := range desired {
		found := matching(actual, want.Name)
		if len(found) != 1 || !sameRule(found[0], want) {
			return fmt.Errorf("failed to verify firewall rule %q", want.Name)
		}
	}
	return nil
}
