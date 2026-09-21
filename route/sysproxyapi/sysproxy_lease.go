package sysproxyapi

import (
	"errors"
	"fmt"
	"time"

	"github.com/UruhaLushia/sysproxy-go/sysproxy"
	"github.com/amamiyakokoro/kokorobox-service/log"
)

// The Desktop renews this lease while it owns the proxy. A lost Desktop
// connection eventually releases the OS setting even if normal quit cleanup
// could not reach the service.
const sysproxyLeaseDuration = 60 * time.Second
const sysproxyLeaseRetry = 15 * time.Second

type sysproxyLease struct {
	mode       sysproxyGuardMode
	opts       *sysproxy.Options
	expected   sysproxyGuardSnapshot
	runner     sysproxyGuardRunner
	deadline   time.Time
	generation uint64
	timer      *time.Timer
}

// All accesses are serialized by sysproxyMutationMu.
var activeSysproxyLease *sysproxyLease
var sysproxyLeaseGeneration uint64

func captureSysproxyLease(mode sysproxyGuardMode, opts *sysproxy.Options, runner sysproxyGuardRunner) (*sysproxyLease, error) {
	current, err := runner.Query(opts)
	if err != nil {
		return nil, fmt.Errorf("query managed system proxy: %w", err)
	}
	return &sysproxyLease{
		mode:     mode,
		opts:     cloneSysproxyOptions(opts),
		expected: newSysproxyGuardSnapshot(mode, current),
		runner:   runner,
	}, nil
}

func replaceSysproxyLease(lease *sysproxyLease) {
	clearSysproxyLease()
	sysproxyLeaseGeneration++
	lease.generation = sysproxyLeaseGeneration
	activeSysproxyLease = lease
	renewSysproxyLease()
}

func renewSysproxyLease() bool {
	lease := activeSysproxyLease
	if lease == nil {
		return false
	}
	lease.deadline = time.Now().Add(sysproxyLeaseDuration)
	if lease.timer != nil {
		lease.timer.Stop()
	}
	generation := lease.generation
	lease.timer = time.AfterFunc(sysproxyLeaseDuration, func() { expireSysproxyLease(generation) })
	return true
}

func clearSysproxyLease() {
	lease := activeSysproxyLease
	activeSysproxyLease = nil
	if lease == nil {
		return
	}
	if lease.timer != nil {
		lease.timer.Stop()
	}
	if err := lease.runner.Close(); err != nil {
		log.Printf("Failed to close system proxy owner: %v", err)
	}
}

func expireSysproxyLease(generation uint64) {
	err := runSysproxyMutation(func() error {
		lease := activeSysproxyLease
		if lease == nil || lease.generation != generation {
			return nil
		}
		if remaining := time.Until(lease.deadline); remaining > 0 {
			lease.timer = time.AfterFunc(remaining, func() { expireSysproxyLease(generation) })
			return nil
		}
		StopGuard()
		if err := releaseSysproxyLease(); err != nil {
			lease.timer = time.AfterFunc(sysproxyLeaseRetry, func() { expireSysproxyLease(generation) })
			return err
		}
		clearSysproxyLease()
		return nil
	})
	if err != nil {
		log.Printf("Failed to release expired system proxy lease: %v", err)
	}
}

// releaseSysproxyLease leaves a user-modified setting alone. It is called
// under sysproxyMutationMu, after the guard has stopped.
func releaseSysproxyLease() error {
	lease := activeSysproxyLease
	if lease == nil {
		return nil
	}
	current, err := lease.runner.Query(lease.opts)
	if err != nil {
		return fmt.Errorf("query system proxy before cleanup: %w", err)
	}
	if !sysproxyGuardMatches(lease.mode, lease.expected, current) {
		log.Println("System proxy changed outside the service; leaving current settings intact")
		return nil
	}
	if err := lease.runner.Disable(lease.opts); err != nil {
		return fmt.Errorf("disable managed system proxy: %w", err)
	}
	log.Println("Released service-owned system proxy")
	return nil
}

// StopManagedProxy is used by the service lifecycle, including a service
// restart while Desktop is still running.
func StopManagedProxy() error {
	return runSysproxyMutation(func() error {
		StopGuard()
		err := releaseSysproxyLease()
		clearSysproxyLease()
		return err
	})
}

func beginManagedProxy(runner sysproxyGuardRunner, mode sysproxyGuardMode, opts *sysproxy.Options) error {
	lease, err := captureSysproxyLease(mode, opts, runner)
	if err != nil {
		StopGuard()
		clearSysproxyLease()
		disableErr := runner.Disable(opts)
		_ = runner.Close()
		return errors.Join(err, disableErr)
	}
	replaceSysproxyLease(lease)
	return nil
}
