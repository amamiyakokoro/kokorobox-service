//go:build !windows

package controller

import (
	"fmt"
	"os"
	"path/filepath"
)

func CreatePrivateEndpoint() (string, string, func(), error) {
	dir, err := os.MkdirTemp("", "kokorobox-mihomo-controller-*")
	if err != nil {
		return "", "", nil, fmt.Errorf("Failed to create core controller directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		_ = os.RemoveAll(dir)
		return "", "", nil, fmt.Errorf("Failed to set core controller directory permissions: %w", err)
	}

	address := filepath.Join(dir, "controller.sock")
	cleanup := func() {
		_ = os.RemoveAll(dir)
	}
	return "unix", address, cleanup, nil
}

func HardenEndpoint(network string, address string) error {
	if network != "unix" {
		return nil
	}

	dir := filepath.Dir(address)
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("Failed to secure core controller directory permissions: %w", err)
	}
	if err := os.Chmod(address, 0o600); err != nil {
		return fmt.Errorf("Failed to secure core controller Unix socket permissions: %w", err)
	}
	return nil
}
