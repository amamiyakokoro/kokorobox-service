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
	"github.com/amamiyakokoro/kokorobox-service/processrouter"
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

	sourceRouterDirectory := filepath.Join(filepath.Dir(source), "process-router")
	routerManifestHash := ""
	if _, err := os.Stat(sourceRouterDirectory); err == nil {
		if err := processrouter.VerifyIntegrity(sourceRouterDirectory); err != nil {
			return "", fmt.Errorf("verify source process-router bundle: %w", err)
		}
		routerManifestHash, err = executableSHA256(filepath.Join(sourceRouterDirectory, "manifest.json"))
		if err != nil {
			return "", fmt.Errorf("hash process-router manifest: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect source process-router bundle: %w", err)
	}

	bundleHash := sha256.Sum256([]byte(sourceHash + "\n" + routerManifestHash))
	bundleID := hex.EncodeToString(bundleHash[:])[:16]
	runtimeRoot := filepath.Join(programFiles, serviceInstallDirectoryName, "runtime")
	targetDirectory := filepath.Join(runtimeRoot, bundleID)
	target := filepath.Join(targetDirectory, identity.ServiceExecutable+".exe")
	withRouter := routerManifestHash != ""

	if _, err := os.Stat(targetDirectory); err == nil {
		if err := validateStagedService(targetDirectory, sourceHash, withRouter); err != nil {
			return "", err
		}
		return target, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect protected service runtime: %w", err)
	}

	if err := os.MkdirAll(runtimeRoot, 0o755); err != nil {
		return "", fmt.Errorf("create protected service runtime root: %w", err)
	}
	temporaryDirectory, err := os.MkdirTemp(runtimeRoot, ".kokorobox-service-*")
	if err != nil {
		return "", fmt.Errorf("create temporary service runtime: %w", err)
	}
	defer os.RemoveAll(temporaryDirectory)

	if err := copyRuntimeFile(source, filepath.Join(temporaryDirectory, identity.ServiceExecutable+".exe")); err != nil {
		return "", fmt.Errorf("stage service executable: %w", err)
	}
	if withRouter {
		temporaryRouterDirectory := filepath.Join(temporaryDirectory, "process-router")
		if err := os.Mkdir(temporaryRouterDirectory, 0o755); err != nil {
			return "", fmt.Errorf("create staged process-router directory: %w", err)
		}
		for _, name := range processrouter.RuntimeBundleFiles() {
			if err := copyRuntimeFile(
				filepath.Join(sourceRouterDirectory, name),
				filepath.Join(temporaryRouterDirectory, name),
			); err != nil {
				return "", fmt.Errorf("stage process-router component %s: %w", name, err)
			}
		}
	}
	if err := validateStagedService(temporaryDirectory, sourceHash, withRouter); err != nil {
		return "", err
	}
	if err := os.Rename(temporaryDirectory, targetDirectory); err != nil {
		if _, statErr := os.Stat(targetDirectory); statErr != nil {
			return "", fmt.Errorf("publish protected service runtime: %w", err)
		}
		if validateErr := validateStagedService(targetDirectory, sourceHash, withRouter); validateErr != nil {
			return "", validateErr
		}
	}
	return target, nil
}

func validateStagedService(directory, expectedServiceHash string, withRouter bool) error {
	servicePath := filepath.Join(directory, identity.ServiceExecutable+".exe")
	serviceHash, err := executableSHA256(servicePath)
	if err != nil {
		return fmt.Errorf("hash staged service executable: %w", err)
	}
	if serviceHash != expectedServiceHash {
		return errors.New("staged service executable has an unexpected digest")
	}
	if withRouter {
		if err := processrouter.VerifyIntegrity(filepath.Join(directory, "process-router")); err != nil {
			return fmt.Errorf("verify staged process-router bundle: %w", err)
		}
	}
	return nil
}

func copyRuntimeFile(source, target string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o755)
	if err != nil {
		_ = input.Close()
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeInputErr := input.Close()
	syncErr := output.Sync()
	closeOutputErr := output.Close()
	switch {
	case copyErr != nil:
		return copyErr
	case closeInputErr != nil:
		return closeInputErr
	case syncErr != nil:
		return syncErr
	default:
		return closeOutputErr
	}
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
