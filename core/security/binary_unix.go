//go:build !windows

package security

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/amamiyakokoro/kokorobox-service/identity"
)

func SecureBinary(corePath string) error {
	if identity.Environment("KOKOROBOX_SKIP_CORE_ACL_HARDENING", "SPARKLE_SKIP_CORE_ACL_HARDENING") == "1" {
		return nil
	}

	if err := removeGroupAndOtherWrite(filepath.Dir(corePath)); err != nil {
		return fmt.Errorf("Failed to secure core directory permissions: %w", err)
	}
	if err := removeGroupAndOtherWrite(corePath); err != nil {
		return fmt.Errorf("Failed to secure core file permissions: %w", err)
	}

	return nil
}

// PrepareBinary hardens an executable in place on Unix installations, where
// packaged application directories are already outside an ordinary user's
// writable application-data directory.
func PrepareBinary(corePath string) (string, error) {
	if err := SecureBinary(corePath); err != nil {
		return "", err
	}
	return corePath, nil
}

func removeGroupAndOtherWrite(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	mode := info.Mode()
	if mode&0o022 == 0 {
		return nil
	}

	return os.Chmod(path, mode&^0o022)
}
