//go:build windows

package sysproxyapi

import (
	"fmt"
	"strings"

	"github.com/UruhaLushia/sysproxy-go/sysproxy"
	"github.com/amamiyakokoro/kokorobox-service/route/auth"
)

func recoverySysproxyOptions(opts *sysproxy.Options, runner sysproxyGuardRunner) (*sysproxy.Options, error) {
	if opts.Device != "" {
		return nil, fmt.Errorf("managed system proxy recovery does not support a Windows device")
	}
	tokenRunner, ok := runner.(*windowsTokenSysproxyGuardRunner)
	if !ok {
		return nil, fmt.Errorf("managed system proxy has no Windows user token")
	}
	user, err := tokenRunner.token.GetTokenUser()
	if err != nil {
		return nil, fmt.Errorf("read managed system proxy user SID: %w", err)
	}
	recovery := cloneSysproxyOptions(opts)
	recovery.UserSID = user.User.Sid.String()
	recovery.UseRegistry = true
	return recovery, nil
}

func validateManagedProxyRecoveryOptions(opts managedProxyOptions) error {
	sid, ok := auth.GetKeyManager().GetAuthorizedSID()
	if !ok || sid == "" || !strings.EqualFold(sid, opts.UserSID) || opts.Device != "" {
		return fmt.Errorf("managed system proxy recovery user does not match the authorized Windows user")
	}
	return nil
}
