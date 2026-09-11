//go:build !darwin

package service

func startService() error {
	svc, err := newControlService()
	if err != nil {
		return err
	}
	return svc.Start()
}

func stopService() error {
	svc, err := newControlService()
	if err != nil {
		return err
	}
	return svc.Stop()
}

func restartService() error {
	svc, err := newControlService()
	if err != nil {
		return err
	}
	return svc.Restart()
}
