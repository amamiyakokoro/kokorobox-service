//go:build !windows && !linux && !darwin

package sysproxyapi

import (
	"fmt"
	"net/http"
)

func captureSysproxyGuardRunner(_ *http.Request) (sysproxyGuardRunner, error) {
	return nil, fmt.Errorf("System proxy guard is not supported on this platform")
}
