package sysproxyapi

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/amamiyakokoro/kokorobox-service/route/httphelper"
	"github.com/amamiyakokoro/sysproxy-go/v2/sysproxy"
	"github.com/go-chi/render"
)

// The native user-process performs the explicit OS mutation. Suspend the existing
// guard first so it cannot undo that change. No OS writes occur in either handler.
func prepareNativeProxy(w http.ResponseWriter, r *http.Request) {
	err := runSysproxyMutation(func() error {
		stopManagedProxyRecovery()
		StopGuard()
		forgetSysproxyLease()
		return nil
	})
	if err != nil {
		httphelper.SendError(w, err)
		return
	}
	render.NoContent(w, r)
}

// Reuse existing crash cleanup and guard ownership after native applies settings.
// This is lease bookkeeping, not a new diagnostics source or setter.
func adoptNativeProxy(w http.ResponseWriter, r *http.Request) {
	var req proxyRequest
	if err := httphelper.DecodeRequest(r, &req); err != nil {
		httphelper.SendError(w, httphelper.BadRequest("Invalid proxy lease"))
		return
	}
	mode := sysproxyGuardModeProxy
	if req.Url != "" {
		mode = sysproxyGuardModePAC
	}
	if req.Server == "" && req.Url == "" {
		httphelper.SendError(w, httphelper.BadRequest("Missing proxy lease endpoint"))
		return
	}
	opts := prepareSysproxyOptions(r, &sysproxy.Options{Proxy: req.Server, Bypass: req.Bypass, PACURL: req.Url, OnlyActiveDevice: req.OnlyActiveDevice})
	err := runSysproxyMutation(func() error {
		runner, err := captureSysproxyGuardRunner(r)
		if err != nil {
			return err
		}
		lease, err := captureNativeProxyLease(mode, opts, runner)
		if err != nil {
			_ = runner.Close()
			return fmt.Errorf("Unable to confirm native proxy settings")
		}
		if err := persistManagedProxy(lease); err != nil {
			_ = runner.Close()
			return fmt.Errorf("Unable to persist native proxy ownership")
		}
		replaceSysproxyLease(lease)
		configureSysproxyGuardBestEffort(r, req.Guard, mode, opts)
		return nil
	})
	if err != nil {
		httphelper.SendError(w, err)
		return
	}
	render.NoContent(w, r)
}

func captureNativeProxyLease(mode sysproxyGuardMode, opts *sysproxy.Options, runner sysproxyGuardRunner) (*sysproxyLease, error) {
	lease, err := captureSysproxyLease(mode, opts, runner)
	if err != nil {
		return nil, err
	}
	expected := lease.expected
	matches := false
	if mode == sysproxyGuardModePAC {
		matches = expected.PACEnable && !expected.PACProxyConflict && expected.PACURL == opts.PACURL
	} else {
		server := firstNonEmpty(expected.ProxyServers["http_server"], expected.ProxyServers["https_server"], expected.ProxyServers["socks_server"])
		matches = expected.ProxyEnable && !expected.ProxyPACConflict && server == opts.Proxy && normalizedNativeBypass(expected.ProxyBypass) == normalizedNativeBypass(opts.Bypass)
		for _, address := range expected.ProxyServers {
			if address != "" && address != opts.Proxy {
				matches = false
			}
		}
	}
	if !matches {
		return nil, fmt.Errorf("Native proxy settings were not confirmed")
	}
	return lease, nil
}
func normalizedNativeBypass(value string) string {
	entries := strings.FieldsFunc(strings.ToLower(value), func(c rune) bool { return c == ';' || c == ',' })
	for i := range entries {
		entries[i] = strings.TrimSpace(entries[i])
	}
	sort.Strings(entries)
	return strings.Join(entries, ";")
}
