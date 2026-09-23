//go:build windows

package sysapi

import (
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"unsafe"

	"github.com/amamiyakokoro/kokorobox-service/route/auth"
	"github.com/amamiyakokoro/kokorobox-service/route/httphelper"
	"golang.org/x/sys/windows"
)

type sidAndAttributes struct {
	SID        *windows.SID
	Attributes uint32
}

type appContainerDetails struct {
	AppContainerSID *windows.SID
	UserSID         *windows.SID
	Name            *uint16
	DisplayName     *uint16
	Description     *uint16
	Capabilities    struct {
		Count uint32
		Items *sidAndAttributes
	}
	Binaries struct {
		Count uint32
		Items **uint16
	}
	WorkingDirectory *uint16
	PackageFullName  *uint16
}

var (
	uwpLoopbackMu             = sync.Mutex{}
	firewallDLL               = windows.NewLazySystemDLL("Firewallapi.dll")
	procEnumAppContainers     = firewallDLL.NewProc("NetworkIsolationEnumAppContainers")
	procFreeAppContainers     = firewallDLL.NewProc("NetworkIsolationFreeAppContainers")
	procGetAppContainerConfig = firewallDLL.NewProc("NetworkIsolationGetAppContainerConfig")
	procSetAppContainerConfig = firewallDLL.NewProc("NetworkIsolationSetAppContainerConfig")
	kernelDLL                 = windows.NewLazySystemDLL("kernel32.dll")
	procGetProcessHeap        = kernelDLL.NewProc("GetProcessHeap")
	procHeapFree              = kernelDLL.NewProc("HeapFree")
)

func sidHex(sid *windows.SID) string {
	if sid == nil || sid.String() == "" {
		return ""
	}
	length := sid.Len()
	if length < 8 || length > 68 {
		return ""
	}
	return hex.EncodeToString(unsafe.Slice((*byte)(unsafe.Pointer(sid)), length))
}

func freeAppContainerConfig(entries *sidAndAttributes, count uint32) {
	if entries == nil {
		return
	}
	heap, _, _ := procGetProcessHeap.Call()
	if heap == 0 {
		return
	}
	for _, entry := range unsafe.Slice(entries, count) {
		if entry.SID != nil {
			procHeapFree.Call(heap, 0, uintptr(unsafe.Pointer(entry.SID)))
		}
	}
	procHeapFree.Call(heap, 0, uintptr(unsafe.Pointer(entries)))
}

func updateUwpLoopback(id string, enabled bool) error {
	ownerSID, ok := auth.GetKeyManager().GetAuthorizedSID()
	if !ok {
		return httphelper.Forbidden("No authorized Windows user")
	}

	// The API replaces the complete exemption set; serialize requests and preserve all other entries.
	uwpLoopbackMu.Lock()
	defer uwpLoopbackMu.Unlock()

	var containerCount uint32
	var containers *appContainerDetails
	code, _, _ := procEnumAppContainers.Call(0, uintptr(unsafe.Pointer(&containerCount)), uintptr(unsafe.Pointer(&containers)))
	if code != 0 {
		return fmt.Errorf("NetworkIsolationEnumAppContainers failed: %d", code)
	}
	if containers != nil {
		defer procFreeAppContainers.Call(uintptr(unsafe.Pointer(containers)))
	}
	if containerCount > 65536 || (containerCount != 0 && containers == nil) {
		return fmt.Errorf("Windows returned an invalid app-container list")
	}
	var target *windows.SID
	for _, container := range unsafe.Slice(containers, containerCount) {
		if sidHex(container.AppContainerSID) != strings.ToLower(id) {
			continue
		}
		if container.UserSID == nil || !strings.EqualFold(container.UserSID.String(), ownerSID) {
			return httphelper.Forbidden("App container does not belong to the authorized user")
		}
		target = container.AppContainerSID
		break
	}
	if target == nil {
		return httphelper.BadRequest("UWP app container is no longer installed")
	}

	var configCount uint32
	var config *sidAndAttributes
	code, _, _ = procGetAppContainerConfig.Call(uintptr(unsafe.Pointer(&configCount)), uintptr(unsafe.Pointer(&config)))
	if code != 0 {
		return fmt.Errorf("NetworkIsolationGetAppContainerConfig failed: %d", code)
	}
	if configCount > 65536 || (configCount != 0 && config == nil) {
		return fmt.Errorf("Windows returned an invalid loopback configuration")
	}
	defer freeAppContainerConfig(config, configCount)
	entries := append([]sidAndAttributes(nil), unsafe.Slice(config, configCount)...)
	existing := false
	for _, entry := range entries {
		if sidHex(entry.SID) == strings.ToLower(id) {
			existing = true
			break
		}
	}
	if existing == enabled {
		return nil
	}
	if enabled {
		entries = append(entries, sidAndAttributes{SID: target})
	} else {
		kept := entries[:0]
		for _, entry := range entries {
			if sidHex(entry.SID) != strings.ToLower(id) {
				kept = append(kept, entry)
			}
		}
		entries = kept
	}
	var ptr uintptr
	if len(entries) != 0 {
		ptr = uintptr(unsafe.Pointer(&entries[0]))
	}
	code, _, _ = procSetAppContainerConfig.Call(uintptr(len(entries)), ptr)
	if code != 0 {
		return fmt.Errorf("NetworkIsolationSetAppContainerConfig failed: %d", code)
	}
	return nil
}
