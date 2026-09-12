//go:build linux

package processrouter

// Linux firewall rules depend on the active cgroup mode and proxy policy, so
// linuxNativeProcess owns their lifecycle. The manager-level controller only
// applies to the standalone Windows process-router binary.
type linuxFirewall struct{}

func newProcessRouterFirewall() firewallController { return linuxFirewall{} }

func (linuxFirewall) Ensure(string) error { return nil }
func (linuxFirewall) Check(string) error  { return nil }
func (linuxFirewall) Remove() error       { return nil }
