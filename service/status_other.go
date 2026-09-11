//go:build !windows

package service

import kservice "github.com/kardianos/service"

func queryServiceStatus() (kservice.Status, error) {
	svc, err := newControlService()
	if err != nil {
		return kservice.StatusUnknown, err
	}
	return svc.Status()
}
