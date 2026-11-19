//go:build windows

package auth

import (
	"fmt"
	"net/http"
	"runtime"

	"github.com/amamiyakokoro/kokorobox-service/route/httphelper"
	"github.com/amamiyakokoro/kokorobox-service/route/pipectx"

	"golang.org/x/sys/windows"
)

var (
	advapi32Principal                 = windows.NewLazySystemDLL("advapi32.dll")
	procImpersonateNamedPipeClientSID = advapi32Principal.NewProc("ImpersonateNamedPipeClient")
)

func getRequestPrincipal(r *http.Request) (string, string, bool, error) {
	handle, ok := pipectx.RequestPipeHandle(r)
	if !ok {
		return "", "", false, nil
	}

	sid, err := getPipeClientSID(handle)
	if err != nil {
		return "", "", false, err
	}

	return "sid", sid, true, nil
}

func getPipeClientSID(pipe windows.Handle) (string, error) {
	var sid string
	err := withPipeClientToken(pipe, func(token windows.Token) error {
		tokenUser, err := token.GetTokenUser()
		if err != nil {
			return fmt.Errorf("Failed to read thread token user: %w", err)
		}
		sid = tokenUser.User.Sid.String()
		return nil
	})
	return sid, err
}

func withPipeClientToken(pipe windows.Handle, useToken func(windows.Token) error) error {
	if pipe == 0 {
		return fmt.Errorf("Invalid named pipe handle")
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if ret, _, callErr := procImpersonateNamedPipeClientSID.Call(uintptr(pipe)); ret == 0 {
		return fmt.Errorf("Failed to impersonate named pipe client: %v", callErr)
	}
	defer windows.RevertToSelf()

	var token windows.Token
	if err := windows.OpenThreadToken(windows.CurrentThread(), windows.TOKEN_QUERY, true, &token); err != nil {
		return fmt.Errorf("Failed to open thread token: %w", err)
	}
	defer token.Close()

	return useToken(token)
}

// The Windows core API can launch a client-selected executable as SYSTEM.
// SID and signing-key authentication also permits standard users for process routing.
func RequireWindowsCoreAdministrator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handle, _ := pipectx.RequestPipeHandle(r)
		var administrator bool
		err := withPipeClientToken(handle, func(token windows.Token) error {
			groups, err := token.GetTokenGroups()
			if err != nil {
				return err
			}
			for _, group := range groups.AllGroups() {
				// A UAC-filtered administrator has the group marked deny-only.
				if group.Sid.IsWellKnown(windows.WinBuiltinAdministratorsSid) &&
					group.Attributes&(windows.SE_GROUP_ENABLED|windows.SE_GROUP_USE_FOR_DENY_ONLY) != 0 {
					administrator = true
					break
				}
			}
			return nil
		})
		if err != nil || !administrator {
			httphelper.SendError(w, httphelper.Forbidden("The Windows service-managed core requires an administrator account"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
