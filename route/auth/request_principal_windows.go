//go:build windows

package auth

import (
	"fmt"
	"net/http"
	"runtime"

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
	if pipe == 0 {
		return "", fmt.Errorf("Invalid named pipe handle")
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if ret, _, callErr := procImpersonateNamedPipeClientSID.Call(uintptr(pipe)); ret == 0 {
		return "", fmt.Errorf("Failed to impersonate named pipe client: %v", callErr)
	}
	defer windows.RevertToSelf()

	var token windows.Token
	if err := windows.OpenThreadToken(windows.CurrentThread(), windows.TOKEN_QUERY, true, &token); err != nil {
		return "", fmt.Errorf("Failed to open thread token: %w", err)
	}
	defer token.Close()

	tokenUser, err := token.GetTokenUser()
	if err != nil {
		return "", fmt.Errorf("Failed to read thread token user: %w", err)
	}

	return tokenUser.User.Sid.String(), nil
}
