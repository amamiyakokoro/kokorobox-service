//go:build windows

package coreapi

import (
	"context"
	"errors"
	"fmt"
	"github.com/amamiyakokoro/kokorobox-service/listen/namedpipe"
	"golang.org/x/sys/windows"
	"net"
	"syscall"
)

func isConnectionRefused(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, windows.WSAECONNREFUSED)
}

func dialCoreController(ctx context.Context, network string, address string) (net.Conn, error) {
	if network != "pipe" {
		return nil, fmt.Errorf("Windows core controller only supports named pipes")
	}
	return namedpipe.DialContext(ctx, address)
}

func directCoreControllerEndpoint() (string, string) { return "pipe", `\\.\pipe\KokoroBox\mihomo` }

func dialDirectCoreController(ctx context.Context) (net.Conn, error) {
	network, address := directCoreControllerEndpoint()
	return dialCoreController(ctx, network, address)
}
