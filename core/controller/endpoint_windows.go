//go:build windows

package controller

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/amamiyakokoro/kokorobox-service/core/security"

	"golang.org/x/sys/windows"
)

func CreatePrivateEndpoint() (string, string, func(), error) {
	token, err := randomToken(16)
	if err != nil {
		return "", "", nil, err
	}
	return "pipe", `\\.\pipe\kokorobox\mihomo-core-` + token, nil, nil
}

func HardenEndpoint(network string, address string) error {
	if network != "pipe" {
		return nil
	}
	if err := security.RestrictPath(address, windows.NO_INHERITANCE); err != nil {
		return fmt.Errorf("Failed to secure core controller pipe permissions: %w", err)
	}
	return nil
}

func randomToken(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("Failed to generate core controller pipe token: %w", err)
	}
	return hex.EncodeToString(data), nil
}
