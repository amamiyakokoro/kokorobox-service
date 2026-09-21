//go:build linux

package coreapi

import (
	corepkg "github.com/amamiyakokoro/kokorobox-service/core"
	"github.com/amamiyakokoro/kokorobox-service/route/pipectx"
	"net/http"
)

func coreLaunchOptions(r *http.Request) []corepkg.LaunchOption {
	group := coreLaunchGroup(r)
	if group == nil {
		return nil
	}
	return []corepkg.LaunchOption{corepkg.WithLogFileGroup(*group)}
}

func coreLaunchGroup(r *http.Request) *uint32 {
	info, ok := pipectx.RequestUnixPeerInfo(r)
	if !ok {
		return nil
	}
	return &info.GID
}
