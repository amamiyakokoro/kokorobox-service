//go:build !windows

package coreapi

import (
	"context"
	"errors"
	"net"
	"syscall"
	"testing"
)

func TestDirectDiagnosticsFindsExistingUnixCoreModes(t *testing.T) {
	for _, live := range directCoreControllerEndpoints() {
		t.Run(live, func(t *testing.T) {
			var inspected []string
			client, server := net.Pipe()
			defer client.Close()
			defer server.Close()
			conn, err := dialFixedDirectControllers(context.Background(), func(_ context.Context, network, address string) (net.Conn, error) {
				if network != "unix" {
					t.Fatal("unexpected transport")
				}
				inspected = append(inspected, address)
				if address == live {
					return client, nil
				}
				return nil, syscall.ENOENT
			})
			if err != nil || conn != client {
				t.Fatalf("could not find live controller: %v", err)
			}
			if inspected[len(inspected)-1] != live {
				t.Fatal("incorrect controller selected")
			}
		})
	}
}
func TestDirectDiagnosticsPreservesInspectionErrorsAndCancellation(t *testing.T) {
	_, err := dialFixedDirectControllers(context.Background(), func(_ context.Context, _, address string) (net.Conn, error) {
		if address == directCoreControllerEndpoints()[0] {
			return nil, syscall.EACCES
		}
		return nil, syscall.ENOENT
	})
	if !errors.Is(err, syscall.EACCES) {
		t.Fatalf("permission failure must not become core stopped: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	_, err = dialFixedDirectControllers(ctx, func(ctx context.Context, _, _ string) (net.Conn, error) { calls++; return nil, ctx.Err() })
	if calls != 1 || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled diagnostic must stop inspecting controllers")
	}
}
