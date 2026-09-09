//go:build linux

package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/amamiyakokoro/kokorobox-service/identity"
)

const (
	serviceRuntimeExecEnv       = "KOKOROBOX_SERVICE_RUNTIME_EXEC"
	legacyServiceRuntimeExecEnv = "SPARKLE_SERVICE_RUNTIME_EXEC"
)

func ensureServiceRuntimeExecutable() error {
	if identity.Environment(serviceRuntimeExecEnv, legacyServiceRuntimeExecEnv) == "1" {
		return nil
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("读取服务可执行文件路径失败：%w", err)
	}

	target, err := stageServiceExecutable(executable)
	if err != nil {
		return err
	}

	env := append(os.Environ(), serviceRuntimeExecEnv+"=1")
	args := append([]string{target}, os.Args[1:]...)
	return syscall.Exec(target, args, env)
}

func stageServiceExecutable(source string) (string, error) {
	hash, mode, err := hashFile(source)
	if err != nil {
		return "", err
	}

	dir := filepath.Join("/run", "kokorobox", "service", hash)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("Failed to create service runtime directory: %w", err)
	}

	target := filepath.Join(dir, filepath.Base(source))
	if _, err := os.Stat(target); err == nil {
		return target, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("Failed to check service runtime copy: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".kokorobox-service-*")
	if err != nil {
		return "", fmt.Errorf("Failed to create service runtime copy: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	input, err := os.Open(source)
	if err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("Failed to open service binary: %w", err)
	}
	_, copyErr := io.Copy(tmp, input)
	closeInputErr := input.Close()
	chmodErr := tmp.Chmod(mode | 0o500)
	closeTmpErr := tmp.Close()

	switch {
	case copyErr != nil:
		return "", fmt.Errorf("Failed to copy service binary: %w", copyErr)
	case closeInputErr != nil:
		return "", fmt.Errorf("Failed to close service binary: %w", closeInputErr)
	case chmodErr != nil:
		return "", fmt.Errorf("设置服务运行副本权限失败：%w", chmodErr)
	case closeTmpErr != nil:
		return "", fmt.Errorf("关闭服务运行副本失败：%w", closeTmpErr)
	}

	if err := os.Rename(tmpPath, target); err != nil {
		return "", fmt.Errorf("Failed to publish service runtime copy: %w", err)
	}
	return target, nil
}

func hashFile(path string) (string, os.FileMode, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("Failed to open service binary: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", 0, fmt.Errorf("读取服务二进制信息失败：%w", err)
	}
	if info.IsDir() {
		return "", 0, fmt.Errorf("Service path points to a directory, not an executable: %s", path)
	}

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", 0, fmt.Errorf("计算服务二进制摘要失败：%w", err)
	}

	return hex.EncodeToString(hash.Sum(nil))[:16], info.Mode().Perm(), nil
}
