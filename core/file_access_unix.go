//go:build linux || darwin

package core

import (
	"fmt"
	"os"
)

func ensureCoreLogDir(dir string, access fileAccess) error {
	if err := os.MkdirAll(dir, 0o775); err != nil {
		return err
	}
	if !access.ok {
		return nil
	}
	if os.Geteuid() == 0 {
		if err := os.Chown(dir, -1, access.groupID); err != nil {
			return fmt.Errorf("Failed to set core log directory group: %w", err)
		}
	}

	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	mode := info.Mode() | 0o070 | os.ModeSetgid
	if err := os.Chmod(dir, mode); err != nil {
		return fmt.Errorf("Failed to set core log directory permissions: %w", err)
	}
	return nil
}

func applyCoreLogFileAccess(path string, access fileAccess) error {
	if !access.ok {
		return nil
	}
	if os.Geteuid() == 0 {
		if err := os.Chown(path, -1, access.groupID); err != nil {
			return fmt.Errorf("Failed to set core log file group: %w", err)
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	mode := info.Mode() | 0o060
	if err := os.Chmod(path, mode); err != nil {
		return fmt.Errorf("Failed to set core log file permissions: %w", err)
	}
	return nil
}
