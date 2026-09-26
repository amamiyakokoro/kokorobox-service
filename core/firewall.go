package core

import (
	"fmt"
	"log"
	"runtime"

	"github.com/amamiyakokoro/kokorobox-service/core/firewall"
)

func (cm *CoreManager) ensureLaunchFirewall(launch *launchSession) error {
	ensure := cm.ensureFirewall
	if ensure == nil {
		ensure = firewall.Ensure
	}
	return ensure(launch.executablePath)
}

// Firewall failure must not disable otherwise functional local proxy connections.
func (cm *CoreManager) configureLaunchFirewall(launch *launchSession) {
	if err := cm.ensureLaunchFirewall(launch); err != nil {
		message := fmt.Sprintf("Core LAN firewall setup failed: %v", err)
		log.Print(message)
		line := fmt.Sprintf("level=warning msg=%q\n", message)
		if launch.logWriter != nil {
			_, _ = launch.logWriter.Write([]byte(line))
		}
		cm.emitCoreEvent(CoreEventLog, line, err)
	}
}

// RepairCoreFirewall uses the manager-owned launch path under the same lock as restart.
func (cm *CoreManager) RepairCoreFirewall() error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("core firewall repair is only supported on Windows")
	}
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	if cm.launch == nil || !cm.isRunning.Load() {
		return fmt.Errorf("start the service-managed core before repairing its firewall")
	}
	return cm.ensureLaunchFirewall(cm.launch)
}
