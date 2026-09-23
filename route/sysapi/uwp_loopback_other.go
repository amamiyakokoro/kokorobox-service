//go:build !windows

package sysapi

import "github.com/amamiyakokoro/kokorobox-service/route/httphelper"

func updateUwpLoopback(_ string, _ bool) error {
	return httphelper.BadRequest("UWP loopback is only available on Windows")
}
