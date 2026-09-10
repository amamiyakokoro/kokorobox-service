//go:build windows

package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/amamiyakokoro/kokorobox-service/identity"
	kservice "github.com/kardianos/service"
	"golang.org/x/sys/windows"
)

const serviceInstallDirectoryName = "KokoroBox Service"

// prepareServiceInstallExecutable stages the service in a machine-protected,
// content-addressed directory. The SCM must never execute a service binary
// from a per-user Desktop installation because that directory is writable by
// the unprivileged user.
func prepareServiceInstallExecutable(source string) (string, error) {
	programFiles, err := windows.KnownFolderPath(windows.FOLDERID_ProgramFiles, windows.KF_FLAG_DEFAULT)
	if err != nil {
		return "", fmt.Errorf("resolve Program Files directory: %w", err)
	}

	sourceHash, err := executableSHA256(source)
	if err != nil {
		return "", fmt.Errorf("hash service executable: %w", err)
	}
	directory := filepath.Join(programFiles, serviceInstallDirectoryName, sourceHash[:16])
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("create protected service directory: %w", err)
	}
	target := filepath.Join(directory, identity.ServiceExecutable+".exe")

	if targetHash, err := executableSHA256(target); err == nil {
		if targetHash != sourceHash {
			return "", errors.New("existing protected service executable has an unexpected digest")
		}
		return target, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect protected service executable: %w", err)
	}

	temporary, err := os.CreateTemp(directory, ".kokorobox-service-*.exe")
	if err != nil {
		return "", fmt.Errorf("create temporary service executable: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	input, err := os.Open(source)
	if err != nil {
		_ = temporary.Close()
		return "", fmt.Errorf("open service executable: %w", err)
	}
	_, copyErr := io.Copy(temporary, input)
	closeInputErr := input.Close()
	syncErr := temporary.Sync()
	closeTemporaryErr := temporary.Close()

	switch {
	case copyErr != nil:
		return "", fmt.Errorf("copy service executable: %w", copyErr)
	case closeInputErr != nil:
		return "", fmt.Errorf("close source service executable: %w", closeInputErr)
	case syncErr != nil:
		return "", fmt.Errorf("flush protected service executable: %w", syncErr)
	case closeTemporaryErr != nil:
		return "", fmt.Errorf("close protected service executable: %w", closeTemporaryErr)
	}

	if err := os.Rename(temporaryPath, target); err != nil {
		return "", fmt.Errorf("publish protected service executable: %w", err)
	}
	return target, nil
}

func executableSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// installServiceRegistration replaces an existing registration only after its
// process has stopped. Content-addressed paths avoid overwriting a running
// Windows executable during upgrades.
func installServiceRegistration(s kservice.Service) error {
	status, err := s.Status()
	if errors.Is(err, kservice.ErrNotInstalled) {
		return s.Install()
	}
	if err != nil {
		return fmt.Errorf("query existing service: %w", err)
	}

	if status != kservice.StatusStopped {
		if err := s.Stop(); err != nil {
			return fmt.Errorf("stop existing service: %w", err)
		}
		if err := waitForServiceStatus(s, kservice.StatusStopped, 20*time.Second); err != nil {
			return err
		}
	}
	if err := s.Uninstall(); err != nil {
		return fmt.Errorf("remove existing service registration: %w", err)
	}
	if err := waitForServiceRemoval(s, 20*time.Second); err != nil {
		return err
	}
	if err := s.Install(); err != nil {
		return fmt.Errorf("register protected service executable: %w", err)
	}
	return nil
}

func waitForServiceStatus(s kservice.Service, expected kservice.Status, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status, err := s.Status()
		if err == nil && status == expected {
			return nil
		}
		if err != nil && !errors.Is(err, kservice.ErrNotInstalled) {
			return fmt.Errorf("query service while waiting for state change: %w", err)
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for service state %v", expected)
}

func waitForServiceRemoval(s kservice.Service, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_, err := s.Status()
		if errors.Is(err, kservice.ErrNotInstalled) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("query service while waiting for removal: %w", err)
		}
		time.Sleep(250 * time.Millisecond)
	}
	return errors.New("timed out waiting for service removal")
}
