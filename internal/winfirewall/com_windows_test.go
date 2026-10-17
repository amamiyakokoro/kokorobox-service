//go:build windows

package winfirewall

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// Real, unregistered COM rules exercise property setters/getters without
// changing the host firewall or requiring an elevated test process.
func TestCOMRuleRoundTrip(t *testing.T) {
	err := withCOM(func() error {
		specs := testRules("C:/ProgramData/new hash's folder/核心.exe")
		specs = append(specs, Inbound("core TCP", "core", specs[0].Application, 6, "*"), Inbound("core UDP", "core", specs[0].Application, 17, "*"))
		for _, spec := range specs {
			rule, err := newRule(spec)
			if err != nil {
				return err
			}
			actual, err := readRule(rule, true)
			rule.Release()
			if err != nil {
				return err
			}
			if !sameRule(actual, spec) {
				return fmt.Errorf("COM round trip: got %+v, want %+v", actual, spec)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCOMEnumeratesWithoutChangingHostPolicy(t *testing.T) {
	err := withStore("KokoroBox nonexistent test group", nil, func(store *comStore) error {
		_, err := store.List()
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Explicitly opt in from an elevated process. Only unique, disabled rules
// are registered, and cleanup runs even when an assertion fails.
func TestCOMAddsRepairsAndRemovesRealRules(t *testing.T) {
	if os.Getenv("KOKOROBOX_TEST_FIREWALL_INTEGRATION") != "1" {
		t.Skip("requires elevation and KOKOROBOX_TEST_FIREWALL_INTEGRATION=1")
	}
	group := fmt.Sprintf("KokoroBox Firewall Test %d", time.Now().UnixNano())
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	desired := testRules(executable)
	for i := range desired {
		desired[i].Group = group
		desired[i].Name = fmt.Sprintf("%s %d", group, i)
		desired[i].Enabled = false
	}
	defer func() {
		if err := Apply(group, desired, Remove, true); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	}()
	for range 2 {
		if err := Apply(group, desired, Ensure, true); err != nil {
			t.Fatal(err)
		}
	}
	for i := range desired {
		desired[i].Application = os.Getenv("SystemRoot") + "/System32/cmd.exe"
	}
	if err := Apply(group, desired, Ensure, true); err != nil {
		t.Fatal(err)
	}
	if err := Apply(group, desired, Check, true); err != nil {
		t.Fatal(err)
	}
	if err := Apply(group, desired, Remove, true); err != nil {
		t.Fatal(err)
	}
}
