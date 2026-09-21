//go:build !windows && !linux && !darwin

package sysproxyapi

import (
	"fmt"
	"github.com/UruhaLushia/sysproxy-go/sysproxy"
)

func recoverySysproxyOptions(_ *sysproxy.Options, _ sysproxyGuardRunner) (*sysproxy.Options, error) {
	return nil, fmt.Errorf("managed system proxy recovery is not supported")
}

func validateManagedProxyRecoveryOptions(_ managedProxyOptions) error {
	return fmt.Errorf("managed system proxy recovery is not supported")
}
