//go:build !windows

package coreapi

import (
	"context"
	"fmt"
	"net"
)

func dialCoreController(ctx context.Context, network string, address string) (net.Conn, error) {
	if network != "unix" {
		return nil, fmt.Errorf("Unix core controller only supports Unix sockets")
	}

	var dialer net.Dialer
	return dialer.DialContext(ctx, "unix", address)
}
