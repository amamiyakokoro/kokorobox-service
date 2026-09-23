package sysproxyapi

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/amamiyakokoro/sysproxy-go/v2/sysproxy"
)

type leaseTestRunner struct {
	config   *sysproxy.ProxyConfig
	disabled int
	closed   int
	applied  int
	applyErr error
}

func (r *leaseTestRunner) Query(*sysproxy.Options) (*sysproxy.ProxyConfig, error) {
	return r.config, nil
}
func (r *leaseTestRunner) Apply(sysproxyGuardMode, *sysproxy.Options) error {
	r.applied++
	return r.applyErr
}
func (r *leaseTestRunner) Disable(*sysproxy.Options) error {
	r.disabled++
	return nil
}
func (r *leaseTestRunner) WaitChange(context.Context, *sysproxy.Options) error { return nil }
func (r *leaseTestRunner) WaitChangeReady(context.Context, *sysproxy.Options, func()) error {
	return nil
}
func (r *leaseTestRunner) Close() error {
	r.closed++
	return nil
}

func testProxyConfig(server string) *sysproxy.ProxyConfig {
	config := &sysproxy.ProxyConfig{}
	config.Proxy.Enable = true
	config.Proxy.SameForAll = true
	config.Proxy.Servers = map[string]string{"http_server": server}
	return config
}

func TestManagedProxyStopsOnServiceShutdown(t *testing.T) {
	runner := &leaseTestRunner{config: testProxyConfig("127.0.0.1:7890")}
	lease, err := captureSysproxyLease(sysproxyGuardModeProxy, &sysproxy.Options{}, runner)
	if err != nil {
		t.Fatal(err)
	}
	replaceSysproxyLease(lease)
	t.Cleanup(func() { _ = StopManagedProxy() })

	if err := StopManagedProxy(); err != nil {
		t.Fatal(err)
	}
	if runner.disabled != 1 || runner.closed != 1 {
		t.Fatalf("expected proxy disabled and runner closed once, got disabled=%d closed=%d", runner.disabled, runner.closed)
	}
}

func TestManagedProxyLeavesUserChangeIntact(t *testing.T) {
	runner := &leaseTestRunner{config: testProxyConfig("127.0.0.1:7890")}
	lease, err := captureSysproxyLease(sysproxyGuardModeProxy, &sysproxy.Options{}, runner)
	if err != nil {
		t.Fatal(err)
	}
	replaceSysproxyLease(lease)
	t.Cleanup(func() { _ = StopManagedProxy() })
	runner.config = testProxyConfig("127.0.0.1:9999")

	if err := StopManagedProxy(); err != nil {
		t.Fatal(err)
	}
	if runner.disabled != 0 || runner.closed != 1 {
		t.Fatalf("user change was overwritten or runner leaked: disabled=%d closed=%d", runner.disabled, runner.closed)
	}
}

func TestExpiredProxyLeaseDisablesProxy(t *testing.T) {
	runner := &leaseTestRunner{config: testProxyConfig("127.0.0.1:7890")}
	lease, err := captureSysproxyLease(sysproxyGuardModeProxy, &sysproxy.Options{}, runner)
	if err != nil {
		t.Fatal(err)
	}
	replaceSysproxyLease(lease)
	t.Cleanup(func() { _ = StopManagedProxy() })
	lease.deadline = time.Now().Add(-time.Second)

	expireSysproxyLease(lease.generation)
	if runner.disabled != 1 || activeSysproxyLease != nil {
		t.Fatalf("expired lease was not released: disabled=%d active=%v", runner.disabled, activeSysproxyLease != nil)
	}
}

func TestOldLeaseTimerCannotDisableReplacement(t *testing.T) {
	oldRunner := &leaseTestRunner{config: testProxyConfig("127.0.0.1:7890")}
	oldLease, err := captureSysproxyLease(sysproxyGuardModeProxy, &sysproxy.Options{}, oldRunner)
	if err != nil {
		t.Fatal(err)
	}
	replaceSysproxyLease(oldLease)
	newRunner := &leaseTestRunner{config: testProxyConfig("127.0.0.1:7891")}
	newLease, err := captureSysproxyLease(sysproxyGuardModeProxy, &sysproxy.Options{}, newRunner)
	if err != nil {
		t.Fatal(err)
	}
	replaceSysproxyLease(newLease)
	t.Cleanup(func() { _ = StopManagedProxy() })

	expireSysproxyLease(oldLease.generation)
	if newRunner.disabled != 0 || activeSysproxyLease != newLease {
		t.Fatal("old lease timer released replacement proxy")
	}
}

func TestProxyLeaseReappliesOnlyAfterNetworkTargetChanges(t *testing.T) {
	runner := &leaseTestRunner{config: testProxyConfig("127.0.0.1:7890")}
	lease, err := captureSysproxyLease(sysproxyGuardModeProxy, &sysproxy.Options{}, runner)
	if err != nil {
		t.Fatal(err)
	}
	for _, signature := range []string{"network-a", "network-a", ""} {
		if err := lease.reconcileNetwork(signature); err != nil {
			t.Fatal(err)
		}
	}
	if runner.applied != 0 {
		t.Fatal("initial or unchanged network must not reapply proxy")
	}
	if err := lease.reconcileNetwork("network-b"); err != nil {
		t.Fatal(err)
	}
	if runner.applied != 1 || lease.networkSignature != "network-b" {
		t.Fatalf("network switch was not reconciled: applied=%d signature=%q", runner.applied, lease.networkSignature)
	}
}

func TestProxyLeaseRetriesFailedNetworkReconciliation(t *testing.T) {
	runner := &leaseTestRunner{config: testProxyConfig("127.0.0.1:7890"), applyErr: errors.New("unavailable")}
	lease, err := captureSysproxyLease(sysproxyGuardModeProxy, &sysproxy.Options{}, runner)
	if err != nil {
		t.Fatal(err)
	}
	lease.networkSignature = "network-a"
	if err := lease.reconcileNetwork("network-b"); err == nil {
		t.Fatal("failed reapply must be reported")
	}
	if lease.networkSignature != "network-a" {
		t.Fatal("failed reapply must retain old signature for retry")
	}
	runner.applyErr = nil
	if err := lease.reconcileNetwork("network-b"); err != nil {
		t.Fatal(err)
	}
	if runner.applied != 2 || lease.networkSignature != "network-b" {
		t.Fatal("network reconciliation was not retried")
	}
}

func TestProxyLeaseRenewalKeepsNetworkReconciliationScheduled(t *testing.T) {
	runner := &leaseTestRunner{config: testProxyConfig("127.0.0.1:7890")}
	lease, err := captureSysproxyLease(sysproxyGuardModeProxy, &sysproxy.Options{}, runner)
	if err != nil {
		t.Fatal(err)
	}
	replaceSysproxyLease(lease)
	t.Cleanup(func() { _ = StopManagedProxy() })
	scheduleSysproxyReconcile(lease)
	if !renewSysproxyLease() {
		t.Fatal("expected lease renewal")
	}
	if lease.reconcileTimer == nil || !lease.reconcileTimer.Stop() {
		t.Fatal("lease renewal stopped network reconciliation")
	}
}
