//go:build linux

package coreapi

import (
	corepkg "github.com/amamiyakokoro/kokorobox-service/core"
	"github.com/amamiyakokoro/kokorobox-service/route/pipectx"
	"net/http"
)

func coreLaunchOptions(r *http.Request) []corepkg.LaunchOption {
	info, ok := pipectx.RequestUnixPeerInfo(r)
	if !ok {
		return nil
	}
	return []corepkg.LaunchOption{corepkg.WithLogFileGroup(info.GID)}
}
