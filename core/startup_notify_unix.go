//go:build !windows

package core

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const startupNotifyDirPrefix = "kokorobox-core-notify-"

func createNativeStartupHook(token string) (*coreStartupHook, error) {
	cleanupStaleStartupNotifyDirs()

	socketDir, err := os.MkdirTemp("", fmt.Sprintf("%s%d-*", startupNotifyDirPrefix, os.Getpid()))
	if err != nil {
		return nil, fmt.Errorf("Failed to create core startup notification directory: %w", err)
	}
	if err := os.Chmod(socketDir, 0o700); err != nil {
		_ = os.RemoveAll(socketDir)
		return nil, fmt.Errorf("Failed to set core startup notification directory permissions: %w", err)
	}

	socketPath := filepath.Join(socketDir, token+".sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		_ = os.RemoveAll(socketDir)
		return nil, fmt.Errorf("Failed to create core startup notification Unix socket: %w", err)
	}
	if err := os.Chmod(socketPath, 0o600); err != nil {
		_ = listener.Close()
		_ = os.RemoveAll(socketDir)
		return nil, fmt.Errorf("Failed to set core startup notification Unix socket permissions: %w", err)
	}
	postUpCommand, err := startupNotifyCommand()
	if err != nil {
		_ = listener.Close()
		_ = os.RemoveAll(socketDir)
		return nil, err
	}
	waitNotification := func() (bool, error) {
		conn, err := listener.Accept()
		if err != nil {
			return true, err
		}
		return false, readStartupNotification(conn, token)
	}

	return newCoreStartupHook(waitNotification, socketPath, postUpCommand, noopShellCommand(), startupNotificationEnv("unix", socketPath, token), func() {
		_ = listener.Close()
		_ = os.RemoveAll(socketDir)
	}), nil
}

func cleanupStaleStartupNotifyDirs() {
	dirs, err := filepath.Glob(filepath.Join(os.TempDir(), startupNotifyDirPrefix+"*"))
	if err != nil {
		log.Printf("Failed to find stale core startup notification directories: %v", err)
		return
	}

	for _, dir := range dirs {
		if startupNotifyOwnerActive(dir) {
			continue
		}
		if err := removeStartupNotifyDir(dir); err != nil {
			log.Printf("Failed to clean up stale core startup notification directory: %v", err)
		}
	}
}

func startupNotifyOwnerActive(dir string) bool {
	name := strings.TrimPrefix(filepath.Base(dir), startupNotifyDirPrefix)
	pidValue, _, ok := strings.Cut(name, "-")
	if !ok {
		return true
	}
	pid, err := strconv.Atoi(pidValue)
	if err != nil || pid <= 0 {
		return true
	}
	err = syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func removeStartupNotifyDir(dir string) error {
	dir = filepath.Clean(dir)
	if filepath.Dir(dir) != filepath.Clean(os.TempDir()) ||
		!strings.HasPrefix(filepath.Base(dir), startupNotifyDirPrefix) {
		return nil
	}
	info, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("Failed to read directory %s: %w", dir, err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 ||
		int(stat.Uid) != os.Geteuid() || info.Mode().Perm() != 0o700 {
		return nil
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("Failed to remove directory %s: %w", dir, err)
	}
	return nil
}

func sendNativeStartupNotification(network string, address string, token string) error {
	if network != "unix" {
		return fmt.Errorf("Unix startup notifications only support Unix sockets")
	}
	conn, err := net.DialTimeout("unix", address, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Write([]byte(token))
	return err
}
