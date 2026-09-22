package sysproxyapi

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/amamiyakokoro/sysproxy-go/v2/sysproxy"
)

func TestRecoveredLeaseOnlyDisablesMatchingProxy(t *testing.T) {
	record := managedProxyRecord{
		Version:  managedProxyRecordVersion,
		Mode:     sysproxyGuardModeProxy,
		Expected: newSysproxyGuardSnapshot(sysproxyGuardModeProxy, testProxyConfig("127.0.0.1:7890")),
	}
	var disabled int
	disable := func(*sysproxy.Options) error { disabled++; return nil }
	query := func(*sysproxy.Options) (*sysproxy.ProxyConfig, error) {
		return testProxyConfig("127.0.0.1:7890"), nil
	}
	if err := recoverManagedProxyRecordWith(record, query, disable); err != nil {
		t.Fatal(err)
	}
	if disabled != 1 {
		t.Fatalf("matching proxy was not disabled: %d", disabled)
	}
	query = func(*sysproxy.Options) (*sysproxy.ProxyConfig, error) {
		return testProxyConfig("127.0.0.1:9999"), nil
	}
	if err := recoverManagedProxyRecordWith(record, query, disable); err != nil {
		t.Fatal(err)
	}
	if disabled != 1 {
		t.Fatal("user-modified proxy was disabled")
	}
	query = func(*sysproxy.Options) (*sysproxy.ProxyConfig, error) {
		return nil, errors.New("session unavailable")
	}
	if err := recoverManagedProxyRecordWith(record, query, disable); err == nil {
		t.Fatal("failed query should retain the recovery record for a later restart")
	}
}

func TestManagedProxyRecordAtomicReplacement(t *testing.T) {
	previousPath := managedProxyRecordPath
	managedProxyRecordPath = filepath.Join(t.TempDir(), "sysproxy", "managed.json")
	t.Cleanup(func() { managedProxyRecordPath = previousPath })
	record := managedProxyRecord{Version: managedProxyRecordVersion, Mode: sysproxyGuardModePAC}
	if err := writeManagedProxyRecord(record); err != nil {
		t.Fatal(err)
	}
	record.Mode = sysproxyGuardModeProxy
	if err := writeManagedProxyRecord(record); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(managedProxyRecordPath)
	if err != nil {
		t.Fatal(err)
	}
	var saved managedProxyRecord
	if err := json.Unmarshal(data, &saved); err != nil || saved.Mode != sysproxyGuardModeProxy {
		t.Fatalf("replacement record is invalid: mode=%q error=%v", saved.Mode, err)
	}
	info, err := os.Stat(managedProxyRecordPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("recovery record permissions: %v", info.Mode().Perm())
	}
}

func TestShutdownKeepsUnrecoveredRecord(t *testing.T) {
	previousPath := managedProxyRecordPath
	managedProxyRecordPath = filepath.Join(t.TempDir(), "sysproxy", "managed.json")
	t.Cleanup(func() { managedProxyRecordPath = previousPath })
	if err := writeManagedProxyRecord(managedProxyRecord{Version: managedProxyRecordVersion, Mode: sysproxyGuardModeProxy}); err != nil {
		t.Fatal(err)
	}
	if err := StopManagedProxy(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(managedProxyRecordPath); err != nil {
		t.Fatalf("unrecovered record was discarded: %v", err)
	}
}
