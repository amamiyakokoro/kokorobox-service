package processrouter

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

var processRouterFiles = []string{
	"kokorobox-process-router.exe",
	"ProxyBridgeCore.dll",
	"WinDivert.dll",
	"WinDivert64.sys",
}

// RuntimeBundleFiles returns the files that must stay next to the privileged
// service runtime for Windows application routing.
func RuntimeBundleFiles() []string {
	return append([]string{"manifest.json"}, processRouterFiles...)
}

// VerifyIntegrity validates a staged Windows process-router bundle.
func VerifyIntegrity(binaryDir string) error {
	return verifyProcessRouterIntegrity(binaryDir)
}

type integrityManifest struct {
	Version int               `json:"version"`
	SHA256  map[string]string `json:"sha256"`
}

func verifyProcessRouterIntegrity(binaryDir string) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	manifestBytes, err := os.ReadFile(filepath.Join(binaryDir, "manifest.json"))
	if err != nil {
		return fmt.Errorf("read process router manifest: %w", err)
	}
	var manifest integrityManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return fmt.Errorf("decode process router manifest: %w", err)
	}
	if manifest.Version != ProtocolVersion {
		return fmt.Errorf("unsupported process router manifest version: %d", manifest.Version)
	}
	for _, name := range processRouterFiles {
		expected := manifest.SHA256[name]
		if len(expected) != sha256.Size*2 {
			return fmt.Errorf("missing process router checksum: %s", name)
		}
		actual, err := fileSHA256(filepath.Join(binaryDir, name))
		if err != nil {
			return err
		}
		if actual != expected {
			return fmt.Errorf("process router checksum mismatch: %s", name)
		}
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open process router component %s: %w", filepath.Base(path), err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash process router component %s: %w", filepath.Base(path), err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
