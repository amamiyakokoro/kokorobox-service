//go:build linux

package core

import (
	"log"

	"github.com/amamiyakokoro/kokorobox-service/core/sandbox"
)

type linuxAutoLauncher struct{}

func (linuxAutoLauncher) Command(launch *launchSession) (*coreCommand, error) {
	if err := sandbox.Probe(); err != nil {
		log.Printf("Core runtime mode: falling back to direct launch (chroot sandbox unavailable: %v)", err)
		return (linuxDirectLauncher{}).Command(launch)
	}
	log.Printf("Core runtime mode: automatically selected chroot sandbox")
	command, err := (linuxSandboxLauncher{}).Command(launch)
	if err != nil {
		if sandbox.IsConfigError(err) {
			return nil, err
		}
		log.Printf("Core runtime mode: falling back to direct launch (failed to prepare chroot sandbox: %v)", err)
		return (linuxDirectLauncher{}).Command(launch)
	}
	command.startFallback = func(err error) (*coreCommand, error) {
		log.Printf("Core runtime mode: falling back to direct launch (sandbox core launch failed: %v)", err)
		return (linuxDirectLauncher{}).Command(launch)
	}
	return command, nil
}
