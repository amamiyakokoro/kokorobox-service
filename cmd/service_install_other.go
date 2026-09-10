//go:build !windows

package cmd

import kservice "github.com/kardianos/service"

func prepareServiceInstallExecutable(source string) (string, error) {
	return source, nil
}

func installServiceRegistration(s kservice.Service) error {
	return s.Install()
}
