//go:build linux

package sysproxyapi

import (
	"net/http"
	"os/user"
	"strconv"

	"github.com/amamiyakokoro/kokorobox-service/route/pipectx"

	"github.com/UruhaLushia/sysproxy-go/sysproxy"
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

	// Electron may scrub the original environment block exposed through
	// /proc/<pid>/environ. Recover the graphical session environment from
	// another process owned by the authenticated socket peer so sysproxy can
	// reach the user's desktop settings and session bus.
	account, err := user.LookupId(strconv.FormatUint(uint64(peer.UID), 10))
	if err == nil {
		target, targetErr := sysproxy.OptionsForUser(account.Username)
		if targetErr == nil {
			prepared.Environment = append([]string(nil), target.Environment...)
		}
	}
	return prepared
}
