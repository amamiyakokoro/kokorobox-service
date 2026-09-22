package sysproxyapi

import (
	"context"
	"testing"
	"time"

	"github.com/amamiyakokoro/sysproxy-go/sysproxy"
)

type leaseTestRunner struct {
	config   *sysproxy.ProxyConfig
	disabled int
	closed   int
}

func (r *leaseTestRunner) Query(*sysproxy.Options) (*sysproxy.ProxyConfig, error) {
	return r.config, nil
}
func (r *leaseTestRunner) Apply(sysproxyGuardMode, *sysproxy.Options) error { return nil }
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
