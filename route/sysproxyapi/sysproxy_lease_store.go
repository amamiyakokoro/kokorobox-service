package sysproxyapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/UruhaLushia/sysproxy-go/sysproxy"
	"github.com/amamiyakokoro/kokorobox-service/log"
)

const managedProxyRecordVersion = 1
const maxManagedProxyRecordSize = 32 << 10

type managedProxyOptions struct {
	Device           string   `json:"device,omitempty"`
	OnlyActiveDevice bool     `json:"only_active_device,omitempty"`
	UseRegistry      bool     `json:"use_registry,omitempty"`
	UserSID          string   `json:"user_sid,omitempty"`
	PeerUID          uint32   `json:"peer_uid,omitempty"`
	PeerGID          uint32   `json:"peer_gid,omitempty"`
	Environment      []string `json:"environment,omitempty"`
}

func (o managedProxyOptions) sysproxyOptions() *sysproxy.Options {
	return &sysproxy.Options{
		Device: o.Device, OnlyActiveDevice: o.OnlyActiveDevice,
		UseRegistry: o.UseRegistry, UserSID: o.UserSID,
		PeerUID: o.PeerUID, PeerGID: o.PeerGID,
		Environment: append([]string(nil), o.Environment...),
	}
}

func managedOptionsFromSysproxy(o *sysproxy.Options) managedProxyOptions {
	return managedProxyOptions{
		Device: o.Device, OnlyActiveDevice: o.OnlyActiveDevice,
		UseRegistry: o.UseRegistry, UserSID: o.UserSID,
		PeerUID: o.PeerUID, PeerGID: o.PeerGID,
		Environment: append([]string(nil), o.Environment...),
	}
}

type managedProxyRecord struct {
	Version  int                   `json:"version"`
	Mode     sysproxyGuardMode     `json:"mode"`
	Options  managedProxyOptions   `json:"options"`
	Expected sysproxyGuardSnapshot `json:"expected"`
}

// Set before serving requests. Only the service writes this root-owned file.
var managedProxyRecordPath string
var managedProxyRecoveryTimer *time.Timer
var managedProxyRecoveryGeneration uint64

const managedProxyRecoveryRetry = 30 * time.Second

func ConfigureManagedProxyRecovery(dataDir string) error {
	return runSysproxyMutation(func() error {
		stopManagedProxyRecovery()
		managedProxyRecordPath = filepath.Join(dataDir, "sysproxy", "managed.json")
		if err := prepareManagedProxyDirectory(filepath.Dir(managedProxyRecordPath)); err != nil {
			return err
		}
		err := recoverManagedProxy()
		if err != nil {
			scheduleManagedProxyRecovery()
		}
		return err
	})
}

func stopManagedProxyRecovery() {
	managedProxyRecoveryGeneration++
	if managedProxyRecoveryTimer != nil {
		managedProxyRecoveryTimer.Stop()
		managedProxyRecoveryTimer = nil
	}
}

func scheduleManagedProxyRecovery() {
	stopManagedProxyRecovery()
	generation := managedProxyRecoveryGeneration
	managedProxyRecoveryTimer = time.AfterFunc(managedProxyRecoveryRetry, func() {
		_ = runSysproxyMutation(func() error {
			if generation != managedProxyRecoveryGeneration {
				return nil
			}
			if activeSysproxyLease != nil {
				return nil
			}
			if err := recoverManagedProxy(); err != nil {
				log.Printf("Failed to retry managed system proxy recovery: %v", err)
				scheduleManagedProxyRecovery()
			}
			return nil
		})
	})
}

func persistManagedProxy(lease *sysproxyLease) error {
	if managedProxyRecordPath == "" {
		return errors.New("managed system proxy recovery is not configured")
	}
	options, err := recoverySysproxyOptions(lease.opts, lease.runner)
	if err != nil {
		return err
	}
	if err := validateManagedProxyRecoveryOptions(managedOptionsFromSysproxy(options)); err != nil {
		return err
	}
	record := managedProxyRecord{
		Version: managedProxyRecordVersion,
		Mode:    lease.mode, Options: managedOptionsFromSysproxy(options),
		Expected: lease.expected,
	}
	return writeManagedProxyRecord(record)
}

func writeManagedProxyRecord(record managedProxyRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if len(data) > maxManagedProxyRecordSize {
		return errors.New("managed system proxy record is too large")
	}
	dir := filepath.Dir(managedProxyRecordPath)
	if err := prepareManagedProxyDirectory(dir); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".managed-*")
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
	if err := os.Rename(tmp.Name(), managedProxyRecordPath); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	parent, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer parent.Close()
	return parent.Sync()
}

func removeManagedProxyRecord() error {
	if managedProxyRecordPath == "" {
		return nil
	}
	err := os.Remove(managedProxyRecordPath)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func recoverManagedProxy() error {
	file, err := os.Open(managedProxyRecordPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxManagedProxyRecordSize+1))
	if err != nil {
		return err
	}
	if len(data) > maxManagedProxyRecordSize {
		return errors.New("managed system proxy record is too large")
	}
	var record managedProxyRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return fmt.Errorf("decode managed system proxy record: %w", err)
	}
	if record.Version != managedProxyRecordVersion || (record.Mode != sysproxyGuardModeProxy && record.Mode != sysproxyGuardModePAC) {
		return errors.New("unsupported managed system proxy record")
	}
	if err := recoverManagedProxyRecord(record); err != nil {
		return err
	}
	return removeManagedProxyRecord()
}

func recoverManagedProxyRecord(record managedProxyRecord) error {
	if err := validateManagedProxyRecoveryOptions(record.Options); err != nil {
		return err
	}
	return recoverManagedProxyRecordWith(record, sysproxy.QueryProxySettings, sysproxy.DisableProxy)
}

func recoverManagedProxyRecordWith(
	record managedProxyRecord,
	query func(*sysproxy.Options) (*sysproxy.ProxyConfig, error),
	disable func(*sysproxy.Options) error,
) error {
	opts := record.Options.sysproxyOptions()
	current, err := query(opts)
	if err != nil {
		return fmt.Errorf("query managed system proxy during recovery: %w", err)
	}
	if !sysproxyGuardMatches(record.Mode, record.Expected, current) {
		log.Println("System proxy differs from the saved lease; leaving current settings intact")
		return nil
	}
	if err := disable(opts); err != nil {
		return fmt.Errorf("disable stale managed system proxy: %w", err)
	}
	log.Println("Recovered stale service-owned system proxy")
	return nil
}
