package sysproxyapi

import (
	"fmt"
	"time"

	"github.com/amamiyakokoro/kokorobox-service/log"
)

// Network reconciliation follows the lease's target. Unlike the optional
// guard, it never restores a proxy changed on the same network.
func scheduleSysproxyReconcile(lease *sysproxyLease) {
	if lease.reconcileTimer != nil {
		lease.reconcileTimer.Stop()
	}
	generation := lease.generation
	lease.reconcileTimer = time.AfterFunc(sysproxyReconcileInterval, func() {
		reconcileSysproxyLease(generation)
	})
}

func reconcileSysproxyLease(generation uint64) {
	err := runSysproxyMutation(func() error {
		lease := activeSysproxyLease
		if lease == nil || lease.generation != generation || !time.Now().Before(lease.deadline) {
			return nil
		}
		defer scheduleSysproxyReconcile(lease)
		signature, err := sysproxyNetworkSignature()
		if err != nil {
			return err
		}
		return lease.reconcileNetwork(signature)
	})
	if err != nil {
		log.Printf("Failed to reconcile system proxy network target: %v", err)
	}
}

func (lease *sysproxyLease) reconcileNetwork(signature string) error {
	if signature == "" {
		return nil
	}
	if lease.networkSignature == "" || lease.networkSignature == signature {
		lease.networkSignature = signature
		return nil
	}
	// QueryProxySettings can inspect a different service from the newly active
	// one when the lease covers all interfaces, so always apply on a target
	// transition even if one queried service still matches.
	if err := lease.runner.Apply(lease.mode, lease.opts); err != nil {
		return fmt.Errorf("apply system proxy after network change: %w", err)
	}
	current, err := lease.runner.Query(lease.opts)
	if err != nil {
		return fmt.Errorf("verify system proxy after network change: %w", err)
	}
	if !sysproxyGuardMatches(lease.mode, lease.expected, current) {
		return fmt.Errorf("system proxy does not match its lease after network reconciliation")
	}
	log.Println("Reapplied service-owned system proxy after network change")
	lease.networkSignature = signature
	return nil
}
