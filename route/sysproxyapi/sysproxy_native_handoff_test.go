package sysproxyapi

import (
	"net/http/httptest"
	"testing"

	"github.com/amamiyakokoro/sysproxy-go/v2/sysproxy"
)

func TestNativeHandoffSuspendsOldGuardAndLeaseWithoutOSWrites(t *testing.T) {
	runner := &leaseTestRunner{config: testProxyConfig("127.0.0.1:18327")}
	lease, err := captureSysproxyLease(sysproxyGuardModeProxy, &sysproxy.Options{}, runner)
	if err != nil {
		t.Fatal(err)
	}
	replaceSysproxyLease(lease)
	w := httptest.NewRecorder()
	prepareNativeProxy(w, httptest.NewRequest("POST", "/native/prepare", nil))
	if w.Code != 204 || runner.applied != 0 || runner.disabled != 0 || runner.closed != 1 || activeSysproxyLease != nil {
		t.Fatalf("handoff modified the OS or retained stale lease: code=%d runner=%+v", w.Code, runner)
	}
}
func TestNativeLeaseConfirmsActualSettingsWithoutApplyingOrDisabling(t *testing.T) {
	runner := &leaseTestRunner{config: testProxyConfig("127.0.0.1:18327")}
	runner.config.Proxy.Bypass = "<local>;localhost"
	opts := &sysproxy.Options{Proxy: "127.0.0.1:18327", Bypass: "localhost,<local>"}
	if _, err := captureNativeProxyLease(sysproxyGuardModeProxy, opts, runner); err != nil {
		t.Fatal(err)
	}
	runner.config = testProxyConfig("127.0.0.1:18328")
	if _, err := captureNativeProxyLease(sysproxyGuardModeProxy, opts, runner); err == nil {
		t.Fatal("adopted externally changed address")
	}
	runner.config = &sysproxy.ProxyConfig{}
	runner.config.PAC.Enable = true
	runner.config.PAC.URL = "http://127.0.0.1:18329/pac"
	if _, err := captureNativeProxyLease(sysproxyGuardModePAC, &sysproxy.Options{PACURL: runner.config.PAC.URL}, runner); err != nil {
		t.Fatal(err)
	}
	if runner.applied != 0 || runner.disabled != 0 {
		t.Fatal("lease confirmation performed OS writes")
	}
}
