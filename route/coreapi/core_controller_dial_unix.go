//go:build !windows

package coreapi

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"
)

func isConnectionRefused(err error) bool { return errors.Is(err, syscall.ECONNREFUSED) }

func dialCoreController(ctx context.Context, network string, address string) (net.Conn, error) {
	if network != "unix" {
		return nil, fmt.Errorf("Unix core controller only supports Unix sockets")
	}

	var dialer net.Dialer
	return dialer.DialContext(ctx, "unix", address)
}

func directCoreControllerEndpoints() []string {
	return []string{"/tmp/kokorobox-mihomo-api.sock", "/tmp/kokorobox-mihomo-api-noperm.sock", "/tmp/kokorobox-mihomo-external.sock"}
}
func directCoreControllerEndpoint() (string, string) {
	return "unix", directCoreControllerEndpoints()[0]
}

// Desktop may use the standard, unprivileged or external-core Unix controller.
// Inspect only these fixed KokoroBox endpoints; never accept caller-supplied paths.
func dialDirectCoreController(ctx context.Context) (net.Conn, error) {
	return dialFixedDirectControllers(ctx, dialCoreController)
}

func dialFixedDirectControllers(ctx context.Context, dial func(context.Context, string, string) (net.Conn, error)) (net.Conn, error) {
	var lastError, inspectionError error
	for _, address := range directCoreControllerEndpoints() {
		conn, err := dial(ctx, "unix", address)
		if err == nil {
			return conn, nil
		}
		lastError = err
		if !errors.Is(err, syscall.ENOENT) && !errors.Is(err, syscall.ECONNREFUSED) {
			inspectionError = err
		}
		if ctx.Err() != nil {
			break
		}
	}
	if inspectionError != nil {
		return nil, inspectionError
	}
	return nil, lastError
}
