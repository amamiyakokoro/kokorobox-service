package cmd

import (
	"fmt"

	"github.com/amamiyakokoro/kokorobox-service/processrouter"
	"github.com/spf13/cobra"
)

var processRouterCmd = &cobra.Command{
	Use:   "process-router",
	Short: "Manage the privileged application-routing helper",
	Args:  cobra.NoArgs,
}

var processRouterFirewallCmd = &cobra.Command{
	Use:   "firewall",
	Short: "Manage application-routing firewall rules",
	Args:  cobra.NoArgs,
}

var processRouterFirewallEnsureCmd = &cobra.Command{
	Use:   "ensure",
	Short: "Create or repair application-routing firewall rules",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := processrouter.EnsureFirewallRules(processrouter.DefaultBinaryPath()); err != nil {
			return fmt.Errorf("ensure application-routing firewall rules: %w", err)
		}
		return nil
	},
}

var processRouterFirewallCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Verify application-routing firewall rules",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := processrouter.CheckFirewallRules(processrouter.DefaultBinaryPath()); err != nil {
			return fmt.Errorf("check application-routing firewall rules: %w", err)
		}
		return nil
	},
}

var processRouterFirewallRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove application-routing firewall rules",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := processrouter.RemoveFirewallRules(); err != nil {
			return fmt.Errorf("remove application-routing firewall rules: %w", err)
		}
		return nil
	},
}
