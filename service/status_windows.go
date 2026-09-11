//go:build windows

package service

import (
	"errors"

	"github.com/amamiyakokoro/kokorobox-service/identity"
	kservice "github.com/kardianos/service"
	"golang.org/x/sys/windows"
)

// queryServiceStatus deliberately requests query-only SCM access. The generic
// service library also requests SERVICE_START and SERVICE_STOP while checking
// status, which makes a read-only status check fail for a standard user.
func queryServiceStatus() (kservice.Status, error) {
	manager, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return kservice.StatusUnknown, err
	}
	defer windows.CloseServiceHandle(manager)

	serviceName, err := windows.UTF16PtrFromString(identity.ServiceName)
	if err != nil {
		return kservice.StatusUnknown, err
	}
	serviceHandle, err := windows.OpenService(manager, serviceName, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return kservice.StatusUnknown, kservice.ErrNotInstalled
		}
		return kservice.StatusUnknown, err
	}
	defer windows.CloseServiceHandle(serviceHandle)

	var status windows.SERVICE_STATUS
	if err := windows.QueryServiceStatus(serviceHandle, &status); err != nil {
		return kservice.StatusUnknown, err
	}

	switch status.CurrentState {
	case windows.SERVICE_START_PENDING, windows.SERVICE_RUNNING:
		return kservice.StatusRunning, nil
	case windows.SERVICE_PAUSE_PENDING,
		windows.SERVICE_PAUSED,
		windows.SERVICE_CONTINUE_PENDING,
		windows.SERVICE_STOP_PENDING,
		windows.SERVICE_STOPPED:
		return kservice.StatusStopped, nil
	default:
		return kservice.StatusUnknown, nil
	}
}
