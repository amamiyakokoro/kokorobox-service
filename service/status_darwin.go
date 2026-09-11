//go:build darwin

package service

import (
	"errors"
	"os/exec"
	"strings"

	"github.com/amamiyakokoro/kokorobox-service/identity"
	kservice "github.com/kardianos/service"
)

func parseLaunchctlPrint(output string, err error) (kservice.Status, error) {
	if err == nil {
		if strings.Contains(output, "state = running") || strings.Contains(output, "\n\tpid = ") {
			return kservice.StatusRunning, nil
		}
		return kservice.StatusStopped, nil
	}

	lower := strings.ToLower(output + " " + err.Error())
	if strings.Contains(lower, "could not find service") || strings.Contains(lower, "service not found") {
		return kservice.StatusUnknown, kservice.ErrNotInstalled
	}
	return kservice.StatusUnknown, err
}

func queryServiceStatus() (kservice.Status, error) {
	target := "system/" + identity.ServiceName
	output, err := exec.Command("/bin/launchctl", "print", target).CombinedOutput()
	if err != nil {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			return kservice.StatusUnknown, err
		}
	}
	return parseLaunchctlPrint(string(output), err)
}
