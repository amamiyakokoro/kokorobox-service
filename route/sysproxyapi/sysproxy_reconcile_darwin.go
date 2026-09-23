//go:build darwin

package sysproxyapi

import "github.com/amamiyakokoro/kokorobox-service/sys"

func sysproxyNetworkSignature() (string, error) {
	return sys.ActiveNetworkSignature()
}
