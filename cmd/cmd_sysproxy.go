package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/UruhaLushia/sysproxy-go/sysproxy"
	"github.com/amamiyakokoro/kokorobox-service/i18n"
	"github.com/spf13/cobra"
)

var sysproxyCmd = &cobra.Command{
	Use:   "sysproxy",
	Short: i18n.DefaultText("Manage system proxy settings"),
}

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: i18n.DefaultText("Set system proxy"),
	Run: func(cmd *cobra.Command, args []string) {
		t := time.Now()
		err := sysproxy.SetProxy(&sysproxy.Options{
			Proxy:            server,
			Bypass:           bypass,
			Device:           device,
			OnlyActiveDevice: onlyActiveDevice,
			UseRegistry:      useRegistry,
		})
		if err != nil {
			fmt.Println(i18n.DefaultText("Failed to update proxy settings: "), i18n.DefaultText(err.Error()))
			return
		}
		fmt.Println(i18n.DefaultText("Proxy settings updated in: "), time.Since(t))
	},
}

var pacCmd = &cobra.Command{
	Use:   "pac",
	Short: i18n.DefaultText("Set PAC proxy"),
	Run: func(cmd *cobra.Command, args []string) {
		t := time.Now()
		err := sysproxy.SetPac(&sysproxy.Options{
			PACURL:           pacUrl,
			Device:           device,
			OnlyActiveDevice: onlyActiveDevice,
			UseRegistry:      useRegistry,
		})
		if err != nil {
			fmt.Println(i18n.DefaultText("Failed to update PAC proxy settings: "), i18n.DefaultText(err.Error()))
			return
		}
		fmt.Println(i18n.DefaultText("PAC proxy settings updated in: "), time.Since(t))
	},
}

var disableCmd = &cobra.Command{
	Use:   "disable",
	Short: i18n.DefaultText("Disable proxy settings"),
	Run: func(cmd *cobra.Command, args []string) {
		t := time.Now()
		err := sysproxy.DisableProxy(&sysproxy.Options{
			Device:           device,
			OnlyActiveDevice: onlyActiveDevice,
			UseRegistry:      useRegistry,
		})
		if err != nil {
			fmt.Println(i18n.DefaultText("Failed to disable proxy settings: "), i18n.DefaultText(err.Error()))
			return
		}
		fmt.Println(i18n.DefaultText("Proxy settings disabled in: "), time.Since(t))
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: i18n.DefaultText("Show current proxy settings"),
	Run: func(cmd *cobra.Command, args []string) {
		status, err := sysproxy.QueryProxySettings(&sysproxy.Options{
			Device:           device,
			OnlyActiveDevice: onlyActiveDevice,
			UseRegistry:      useRegistry,
		})
		if err != nil {
			fmt.Println(i18n.DefaultText("Failed to query proxy settings: "), i18n.DefaultText(err.Error()))
			return
		}
		statusJSON, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			fmt.Println(i18n.DefaultText("Failed to format JSON: "), i18n.DefaultText(err.Error()))
			return
		}
		fmt.Println(string(statusJSON))
	},
}
