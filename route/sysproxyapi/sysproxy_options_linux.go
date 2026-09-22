//go:build linux

package sysproxyapi

import (
	"net/http"

	"github.com/amamiyakokoro/kokorobox-service/route/pipectx"

	"github.com/amamiyakokoro/sysproxy-go/v2/sysproxy"
)

func prepareSysproxyOptions(r *http.Request, opt *sysproxy.Options) *sysproxy.Options {
	prepared := cloneSysproxyOptions(opt)

	peer, ok := pipectx.RequestUnixPeerInfo(r)
	if !ok {
		return prepared
	}

	prepared.PeerPID = peer.PID
	prepared.PeerUID = peer.UID
	prepared.PeerGID = peer.GID
	return prepared
}
