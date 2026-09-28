//go:build windows

package core

import (
	"fmt"
	"time"

	"github.com/amamiyakokoro/kokorobox-service/core/security"
	"github.com/amamiyakokoro/kokorobox-service/listen"
	"github.com/amamiyakokoro/kokorobox-service/listen/namedpipe"
)

func createNativeStartupHook(token string) (*coreStartupHook, error) {
	pipePath := `\\.\pipe\kokorobox\core-notify-` + token
	listener, err := listen.ListenNamedPipe(pipePath, currentProcessPipeSDDL())
	if err != nil {
		return nil, fmt.Errorf("Failed to create core startup notification pipe: %w", err)
	}
	postUpCommand, executable, err := startupNotifyCommand()
	if err != nil {
		_ = listener.Close()
		return nil, err
	}

	waitNotification := func() (bool, error) {
		conn, err := listener.Accept()
		if err != nil {
			return true, err
		}
		return false, readStartupNotification(conn, token)
	}
	notifyEnv := startupNotificationEnv("pipe", pipePath, token)
	notifyEnv[startupNotifyExecutableEnv] = executable
	return newCoreStartupHook(waitNotification, pipePath, postUpCommand, noopShellCommand(), notifyEnv, func() {
		_ = listener.Close()
	}), nil
}

func sendNativeStartupNotification(network string, address string, token string) error {
	if network != "pipe" {
		return fmt.Errorf("Windows startup notifications only support named pipes")
	}
	conn, err := namedpipe.DialTimeout(address, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Write([]byte(token))
	return err
}

func currentProcessPipeSDDL() string {
	sid, err := security.CurrentProcessSID()
	if err != nil {
		return "D:P(A;;GA;;;SY)(A;;GA;;;BA)"
	}
	return fmt.Sprintf("D:P(A;;GA;;;%s)(A;;GA;;;SY)(A;;GA;;;BA)", sid.String())
}
