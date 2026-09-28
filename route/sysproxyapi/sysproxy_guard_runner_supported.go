//go:build windows || linux || darwin

package sysproxyapi

import (
	"fmt"

	"github.com/amamiyakokoro/sysproxy-go/v2/sysproxy"
)

func querySysproxyGuardSettings(opts *sysproxy.Options) (*sysproxy.ProxyConfig, error) {
	return sysproxy.QueryProxySettings(opts)
}

func applySysproxyGuardSettings(mode sysproxyGuardMode, opts *sysproxy.Options) error {
	switch mode {
	case sysproxyGuardModeProxy:
		return sysproxy.SetProxy(opts)
	case sysproxyGuardModePAC:
		return sysproxy.SetPac(opts)
	default:
		return fmt.Errorf("Unknown system proxy guard mode: %s", mode)
	}
}
