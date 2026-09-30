package core

// RuntimeDiagnosticState reuses the manager's state and last lifecycle event.
// Never return startup log text: it can include private configuration values.
func (cm *CoreManager) RuntimeDiagnosticState() (bool, string) {
	running := cm.isRunning.Load() && cm.pid.Load() > 0
	cm.eventHub.mutex.Lock()
	event := cm.eventHub.last
	cm.eventHub.mutex.Unlock()
	switch event.Type {
	case CoreEventFailed:
		return running, "core-start-failed"
	case CoreEventRestartFailed:
		return running, "core-restart-failed"
	case CoreEventExited:
		return running, "core-exited"
	}
	return running, ""
}
