//go:build !windows && !linux

package processrouter

type unsupportedFirewall struct{}

func newProcessRouterFirewall() firewallController { return unsupportedFirewall{} }

func (unsupportedFirewall) Ensure(string) error { return ErrUnsupported }
func (unsupportedFirewall) Check(string) error  { return ErrUnsupported }
func (unsupportedFirewall) Remove() error       { return nil }
