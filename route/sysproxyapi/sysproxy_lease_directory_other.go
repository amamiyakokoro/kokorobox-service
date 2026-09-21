//go:build !windows

package sysproxyapi

import "os"

func prepareManagedProxyDirectory(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.Chmod(dir, 0o700)
}
