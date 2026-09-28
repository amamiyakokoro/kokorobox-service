//go:build windows

package coreapi

import (
	"context"
	"fmt"
	"github.com/amamiyakokoro/kokorobox-service/listen/namedpipe"
	"net"
)

func dialCoreController(ctx context.Context, network string, address string) (net.Conn, error) {
	if network != "pipe" {
		return nil, fmt.Errorf("Windows core controller only supports named pipes")
	}
	return namedpipe.DialContext(ctx, address)
}
