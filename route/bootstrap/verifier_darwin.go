//go:build darwin

package bootstrap

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/amamiyakokoro/kokorobox-service/route/pipectx"
	"github.com/ebitengine/purego"
)

const (
	expectedBundleIdentifier = "com.amamiyakokoro.app"
	expectedTeamIdentifier   = "755TNLRN92"
	kCFStringEncodingUTF8    = 0x08000100
	kSecCSStrictValidate     = 1 << 4
)

type securityFunctions struct {
	cfDataCreate                        func(uintptr, unsafe.Pointer, int64) uintptr
	cfDictionaryCreate                  func(uintptr, unsafe.Pointer, unsafe.Pointer, int64, uintptr, uintptr) uintptr
	cfStringCreateWithCString           func(uintptr, *byte, uint32) uintptr
	cfRelease                           func(uintptr)
	secCodeCopyGuestWithAttributes      func(uintptr, uintptr, uint32, *uintptr) int32
	secRequirementCreateWithString      func(uintptr, uint32, *uintptr) int32
	secCodeCheckValidity                func(uintptr, uint32, uintptr) int32
	copyMemory                          func(unsafe.Pointer, uintptr, uintptr) unsafe.Pointer
	secGuestAttributeAuditDictionaryKey uintptr
}

var (
	loadSecurityOnce sync.Once
	loadedSecurity   securityFunctions
	loadSecurityErr  error
)

func loadSecurityFunctions() (securityFunctions, error) {
	loadSecurityOnce.Do(func() {
		coreFoundation, err := purego.Dlopen(
			"/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation",
			purego.RTLD_NOW|purego.RTLD_LOCAL,
		)
		if err != nil {
			loadSecurityErr = fmt.Errorf("load CoreFoundation: %w", err)
			return
		}
		security, err := purego.Dlopen(
			"/System/Library/Frameworks/Security.framework/Security",
			purego.RTLD_NOW|purego.RTLD_LOCAL,
		)
		if err != nil {
			loadSecurityErr = fmt.Errorf("load Security.framework: %w", err)
			return
		}

		purego.RegisterLibFunc(&loadedSecurity.cfDataCreate, coreFoundation, "CFDataCreate")
		purego.RegisterLibFunc(&loadedSecurity.cfDictionaryCreate, coreFoundation, "CFDictionaryCreate")
		purego.RegisterLibFunc(&loadedSecurity.cfStringCreateWithCString, coreFoundation, "CFStringCreateWithCString")
		purego.RegisterLibFunc(&loadedSecurity.cfRelease, coreFoundation, "CFRelease")
		purego.RegisterLibFunc(&loadedSecurity.secCodeCopyGuestWithAttributes, security, "SecCodeCopyGuestWithAttributes")
		purego.RegisterLibFunc(&loadedSecurity.secRequirementCreateWithString, security, "SecRequirementCreateWithString")
		purego.RegisterLibFunc(&loadedSecurity.secCodeCheckValidity, security, "SecCodeCheckValidity")
		purego.RegisterLibFunc(&loadedSecurity.copyMemory, purego.RTLD_DEFAULT, "memcpy")

		auditKeySymbol, err := purego.Dlsym(security, "kSecGuestAttributeAudit")
		if err != nil {
			loadSecurityErr = fmt.Errorf("load kSecGuestAttributeAudit: %w", err)
			return
		}
		loadedSecurity.copyMemory(
			unsafe.Pointer(&loadedSecurity.secGuestAttributeAuditDictionaryKey),
			auditKeySymbol,
			unsafe.Sizeof(loadedSecurity.secGuestAttributeAuditDictionaryKey),
		)
		if loadedSecurity.secGuestAttributeAuditDictionaryKey == 0 {
			loadSecurityErr = fmt.Errorf("kSecGuestAttributeAudit is unavailable")
		}
	})
	return loadedSecurity, loadSecurityErr
}

func verifyKokoroBoxProcess(token pipectx.AuditToken) error {
	security, err := loadSecurityFunctions()
	if err != nil {
		return err
	}

	auditData := security.cfDataCreate(0, unsafe.Pointer(&token[0]), int64(unsafe.Sizeof(token)))
	if auditData == 0 {
		return fmt.Errorf("create bootstrap audit token data")
	}
	defer security.cfRelease(auditData)

	key := security.secGuestAttributeAuditDictionaryKey
	value := auditData
	attributes := security.cfDictionaryCreate(
		0,
		unsafe.Pointer(&key),
		unsafe.Pointer(&value),
		1,
		0,
		0,
	)
	if attributes == 0 {
		return fmt.Errorf("create bootstrap code attributes")
	}
	defer security.cfRelease(attributes)

	var guest uintptr
	if status := security.secCodeCopyGuestWithAttributes(0, attributes, 0, &guest); status != 0 {
		return fmt.Errorf("resolve bootstrap client code: OSStatus %d", status)
	}
	defer security.cfRelease(guest)

	requirementText := fmt.Sprintf(
		`anchor apple generic and identifier %q and certificate leaf[subject.OU] = %q and ! entitlement["com.apple.security.get-task-allow"] exists`,
		expectedBundleIdentifier,
		expectedTeamIdentifier,
	)
	cRequirement := append([]byte(requirementText), 0)
	requirementString := security.cfStringCreateWithCString(0, &cRequirement[0], kCFStringEncodingUTF8)
	if requirementString == 0 {
		return fmt.Errorf("create bootstrap code requirement")
	}
	defer security.cfRelease(requirementString)

	var requirement uintptr
	if status := security.secRequirementCreateWithString(requirementString, 0, &requirement); status != 0 {
		return fmt.Errorf("compile bootstrap code requirement: OSStatus %d", status)
	}
	defer security.cfRelease(requirement)

	if status := security.secCodeCheckValidity(guest, kSecCSStrictValidate, requirement); status != 0 {
		return fmt.Errorf("bootstrap client signature rejected: OSStatus %d", status)
	}
	return nil
}
