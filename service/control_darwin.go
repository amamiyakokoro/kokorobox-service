//go:build darwin

package service

import (
	"fmt"
	"os/exec"

	"github.com/amamiyakokoro/kokorobox-service/identity"
)

func launchctlServiceTarget() string {
	return "system/" + identity.ServiceName
}

func runLaunchctl(arguments ...string) error {
	output, err := exec.Command("/bin/launchctl", arguments...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl %s failed: %s: %w", arguments[0], string(output), err)
	}
	return nil
}

func startService() error {
	return runLaunchctl("kickstart", launchctlServiceTarget())
}

func stopService() error {
	return runLaunchctl("kill", "SIGTERM", launchctlServiceTarget())
}

func restartService() error {
	return runLaunchctl("kickstart", "-k", launchctlServiceTarget())
}
