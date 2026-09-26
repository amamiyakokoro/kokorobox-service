//go:build !windows

package firewall

func Ensure(string) error { return nil }
