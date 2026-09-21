//go:build windows

package sysproxyapi

import (
	"os"

	"github.com/amamiyakokoro/kokorobox-service/core/security"
	"golang.org/x/sys/windows"
)

func prepareManagedProxyDirectory(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return security.RestrictPath(dir, windows.CONTAINER_INHERIT_ACE|windows.OBJECT_INHERIT_ACE)
}
