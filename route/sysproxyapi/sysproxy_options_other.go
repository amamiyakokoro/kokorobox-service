//go:build !linux

package sysproxyapi

import (
	"net/http"

	"github.com/amamiyakokoro/sysproxy-go/sysproxy"
)

func prepareSysproxyOptions(_ *http.Request, opt *sysproxy.Options) *sysproxy.Options {
	return cloneSysproxyOptions(opt)
}
