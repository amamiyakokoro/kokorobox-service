//go:build linux

package core

import (
	"log"
	"strings"

	"github.com/amamiyakokoro/kokorobox-service/identity"
)

const (
	disableLinuxSandboxEnv       = "KOKOROBOX_CORE_DISABLE_LINUX_SANDBOX"
	legacyDisableLinuxSandboxEnv = "SPARKLE_CORE_DISABLE_LINUX_SANDBOX"
)

func newCoreLauncher(launch *launchSession) coreLauncher {
	if sandboxDisabled() {
		log.Printf("Core runtime mode: direct launch (%s is enabled)", disableLinuxSandboxEnv)
		return linuxDirectLauncher{}
	}

	mode := CoreRunModeAuto
	if launch != nil && launch.profile.Mode != "" {
		mode = launch.profile.Mode
	}
	switch mode {
	case CoreRunModeDirect:
		log.Printf("Core runtime mode: direct launch")
		return linuxDirectLauncher{}
	case CoreRunModeSandbox:
		log.Printf("Core runtime mode: forced chroot sandbox")
		return linuxSandboxLauncher{}
	case CoreRunModeAuto:
		return linuxAutoLauncher{}
	default:
		log.Printf("Invalid core runtime mode %q; using auto", mode)
		return linuxAutoLauncher{}
	}
}

func sandboxDisabled() bool {
	value := strings.TrimSpace(identity.Environment(disableLinuxSandboxEnv, legacyDisableLinuxSandboxEnv))
	return value == "1" || strings.EqualFold(value, "true") || strings.EqualFold(value, "yes")
}
