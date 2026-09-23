//go:build !darwin

package sysproxyapi

func sysproxyNetworkSignature() (string, error) {
	return "", nil
}
