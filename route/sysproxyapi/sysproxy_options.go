package sysproxyapi

import "github.com/amamiyakokoro/sysproxy-go/v2/sysproxy"

func cloneSysproxyOptions(opt *sysproxy.Options) *sysproxy.Options {
	if opt == nil {
		return &sysproxy.Options{}
	}
	return new(*opt)
}
