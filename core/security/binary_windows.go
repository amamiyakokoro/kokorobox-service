//go:build windows

package security

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/amamiyakokoro/kokorobox-service/identity"

	"golang.org/x/sys/windows"
)

// PrepareBinary copies a user-supplied packaged core into a protected,
// content-addressed machine runtime before the SYSTEM service executes it.
// The source application directory remains writable by its user and updater.
func PrepareBinary(corePath string) (string, error) {
	if identity.Environment("KOKOROBOX_SKIP_CORE_ACL_HARDENING", "SPARKLE_SKIP_CORE_ACL_HARDENING") == "1" {
		return corePath, nil
	}

	programData, err := windows.KnownFolderPath(windows.FOLDERID_ProgramData, windows.KF_FLAG_DEFAULT)
	if err != nil {
		return "", fmt.Errorf("resolve ProgramData directory: %w", err)
	}
	sourceHash, err := binarySHA256(corePath)
	if err != nil {
		return "", fmt.Errorf("hash core executable: %w", err)
	}
	runtimeRoot := filepath.Join(programData, identity.ConfigDirectoryName, "core-runtime")
	if err := os.MkdirAll(runtimeRoot, 0o755); err != nil {
		return "", fmt.Errorf("create protected core runtime root: %w", err)
	}
	if err := RestrictPath(runtimeRoot, windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT); err != nil {
		return "", fmt.Errorf("protect core runtime root: %w", err)
	}
	targetDirectory := filepath.Join(runtimeRoot, sourceHash[:16])
	target := filepath.Join(targetDirectory, filepath.Base(corePath))

	if targetHash, err := binarySHA256(target); err == nil {
		if targetHash != sourceHash {
			return "", errors.New("staged core executable has an unexpected digest")
		}
		if err := SecureBinary(target); err != nil {
			return "", err
		}
		return target, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect staged core executable: %w", err)
	}

	if err := os.Mkdir(targetDirectory, 0o755); err != nil && !os.IsExist(err) {
		return "", fmt.Errorf("create protected core runtime directory: %w", err)
	}
	temporary, err := os.CreateTemp(targetDirectory, ".kokorobox-core-*.exe")
	if err != nil {
		return "", fmt.Errorf("create temporary core executable: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	input, err := os.Open(corePath)
	if err != nil {
		_ = temporary.Close()
		return "", fmt.Errorf("open core executable: %w", err)
	}
	_, copyErr := io.Copy(temporary, input)
	closeInputErr := input.Close()
	syncErr := temporary.Sync()
	closeTemporaryErr := temporary.Close()
	switch {
	case copyErr != nil:
		return "", fmt.Errorf("copy core executable: %w", copyErr)
	case closeInputErr != nil:
		return "", fmt.Errorf("close source core executable: %w", closeInputErr)
	case syncErr != nil:
		return "", fmt.Errorf("flush staged core executable: %w", syncErr)
	case closeTemporaryErr != nil:
		return "", fmt.Errorf("close staged core executable: %w", closeTemporaryErr)
	}

	temporaryHash, err := binarySHA256(temporaryPath)
	if err != nil {
		return "", fmt.Errorf("hash staged core executable: %w", err)
	}
	if temporaryHash != sourceHash {
		return "", errors.New("copied core executable has an unexpected digest")
	}
	if err := os.Rename(temporaryPath, target); err != nil {
		if targetHash, hashErr := binarySHA256(target); hashErr != nil || targetHash != sourceHash {
			return "", fmt.Errorf("publish protected core executable: %w", err)
		}
	}
	if err := SecureBinary(target); err != nil {
		return "", err
	}
	return target, nil
}

func binarySHA256(path string) (string, error) {
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

func SecureBinary(corePath string) error {
	if identity.Environment("KOKOROBOX_SKIP_CORE_ACL_HARDENING", "SPARKLE_SKIP_CORE_ACL_HARDENING") == "1" {
		return nil
	}

	if err := RestrictPath(filepath.Dir(corePath), windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT); err != nil {
		return fmt.Errorf("加固核心目录权限失败：%w", err)
	}
	if err := RestrictPath(corePath, windows.NO_INHERITANCE); err != nil {
		return fmt.Errorf("加固核心文件权限失败：%w", err)
	}

	return nil
}

func RestrictPath(path string, inheritance uint32) error {
	currentSID, err := CurrentProcessSID()
	if err != nil {
		return err
	}
	systemSID, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return err
	}
	adminSID, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return err
	}

	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{
		fullAccessEntry(currentSID, windows.TRUSTEE_IS_USER, inheritance),
		fullAccessEntry(systemSID, windows.TRUSTEE_IS_USER, inheritance),
		fullAccessEntry(adminSID, windows.TRUSTEE_IS_GROUP, inheritance),
	}, nil)
	if err != nil {
		return err
	}

	return windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		acl,
		nil,
	)
}

func fullAccessEntry(sid *windows.SID, trusteeType windows.TRUSTEE_TYPE, inheritance uint32) windows.EXPLICIT_ACCESS {
	return windows.EXPLICIT_ACCESS{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       inheritance,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  trusteeType,
			TrusteeValue: windows.TrusteeValueFromSID(sid),
		},
	}
}

func CurrentProcessSID() (*windows.SID, error) {
	token := windows.GetCurrentProcessToken()
	user, err := token.GetTokenUser()
	if err != nil {
		return nil, fmt.Errorf("读取当前进程 SID 失败：%w", err)
	}

	return user.User.Sid, nil
}
