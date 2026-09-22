//go:build linux

package sysproxyapi

import (
	"fmt"
	"strings"

	"github.com/amamiyakokoro/kokorobox-service/route/auth"
	"github.com/amamiyakokoro/sysproxy-go/v2/sysproxy"
)

var recoveryLinuxEnvKeys = map[string]bool{
	"HOME": true, "PATH": true, "XDG_CURRENT_DESKTOP": true,
	"XDG_RUNTIME_DIR": true, "XDG_CONFIG_HOME": true,
	"XDG_DATA_HOME": true, "DBUS_SESSION_BUS_ADDRESS": true,
	"KDE_SESSION_VERSION": true,
}

func recoverySysproxyOptions(opts *sysproxy.Options, runner sysproxyGuardRunner) (*sysproxy.Options, error) {
	linuxRunner, ok := runner.(*linuxSysproxyGuardRunner)
	if !ok {
		return nil, fmt.Errorf("managed system proxy has no Linux user session")
	}
	recovery := cloneSysproxyOptions(opts)
	recovery.PeerPID = 0
	recovery.PeerUID = linuxRunner.uid
	recovery.PeerGID = linuxRunner.gid
	for _, item := range linuxRunner.env {
		key, _, ok := strings.Cut(item, "=")
		if ok && recoveryLinuxEnvKeys[key] {
			recovery.Environment = append(recovery.Environment, item)
		}
	}
	if len(recovery.Environment) == 0 || recovery.PeerUID == 0 {
		return nil, fmt.Errorf("managed system proxy has no recoverable Linux user session")
	}
	return recovery, nil
}

func validateManagedProxyRecoveryOptions(opts managedProxyOptions) error {
	uid, ok := auth.GetKeyManager().GetAuthorizedUID()
	if !ok || uid == 0 || uid != opts.PeerUID || len(opts.Environment) == 0 {
		return fmt.Errorf("managed system proxy recovery user does not match the authorized Linux user")
	}
	return nil
}
