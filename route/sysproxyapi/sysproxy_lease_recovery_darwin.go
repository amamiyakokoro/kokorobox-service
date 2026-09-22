//go:build darwin

package sysproxyapi

import "github.com/amamiyakokoro/sysproxy-go/sysproxy"

func recoverySysproxyOptions(opts *sysproxy.Options, _ sysproxyGuardRunner) (*sysproxy.Options, error) {
	return cloneSysproxyOptions(opts), nil
}

func validateManagedProxyRecoveryOptions(_ managedProxyOptions) error { return nil }
