package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/amamiyakokoro/kokorobox-service/i18n"
	"github.com/amamiyakokoro/sysproxy-go/v2/sysproxy"
	"github.com/spf13/cobra"
)

var sysproxyCmd = &cobra.Command{
	Use:   "sysproxy",
	Short: i18n.DefaultText("Inspect system proxy settings"),
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Help()
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: i18n.DefaultText("Show current proxy settings"),
	RunE: func(cmd *cobra.Command, args []string) error {
		status, err := sysproxy.QueryProxySettings(&sysproxy.Options{
			Device:           device,
			OnlyActiveDevice: onlyActiveDevice,
			UseRegistry:      useRegistry,
		})
		if err != nil {
			fmt.Fprintln(cmd.ErrOrStderr(), i18n.DefaultText("Failed to query proxy settings: "), i18n.DefaultText(err.Error()))
			return newReportedError(err)
		}
		statusJSON, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			fmt.Fprintln(cmd.ErrOrStderr(), i18n.DefaultText("Failed to format JSON: "), i18n.DefaultText(err.Error()))
			return newReportedError(err)
		}
		fmt.Println(string(statusJSON))
		return nil
	},
}
