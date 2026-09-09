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
	Short: i18n.DefaultText("管理系统代理设置"),
}

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: i18n.DefaultText("设置系统代理"),
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
			fmt.Println(i18n.DefaultText("设置代理失败："), i18n.DefaultText(err.Error()))
			return
		}
		fmt.Println(i18n.DefaultText("代理设置成功，耗时："), time.Since(t))
	},
}

var pacCmd = &cobra.Command{
	Use:   "pac",
	Short: i18n.DefaultText("设置 PAC 代理"),
	Run: func(cmd *cobra.Command, args []string) {
		t := time.Now()
		err := sysproxy.SetPac(&sysproxy.Options{
			PACURL:           pacUrl,
			Device:           device,
			OnlyActiveDevice: onlyActiveDevice,
			UseRegistry:      useRegistry,
		})
		if err != nil {
			fmt.Println(i18n.DefaultText("设置 PAC 代理失败："), i18n.DefaultText(err.Error()))
			return
		}
		fmt.Println(i18n.DefaultText("PAC 代理设置成功，耗时："), time.Since(t))
	},
}

var disableCmd = &cobra.Command{
	Use:   "disable",
	Short: i18n.DefaultText("取消代理设置"),
	Run: func(cmd *cobra.Command, args []string) {
		t := time.Now()
		err := sysproxy.DisableProxy(&sysproxy.Options{
			Device:           device,
			OnlyActiveDevice: onlyActiveDevice,
			UseRegistry:      useRegistry,
		})
		if err != nil {
			fmt.Println(i18n.DefaultText("取消代理设置失败："), i18n.DefaultText(err.Error()))
			return
		}
		fmt.Println(i18n.DefaultText("代理设置已取消，耗时："), time.Since(t))
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: i18n.DefaultText("查看当前代理设置"),
	Run: func(cmd *cobra.Command, args []string) {
		status, err := sysproxy.QueryProxySettings(&sysproxy.Options{
			Device:           device,
			OnlyActiveDevice: onlyActiveDevice,
			UseRegistry:      useRegistry,
		})
		if err != nil {
			fmt.Println(i18n.DefaultText("查询代理设置失败："), i18n.DefaultText(err.Error()))
			return
		}
		statusJSON, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			fmt.Println(i18n.DefaultText("格式化 JSON 失败："), i18n.DefaultText(err.Error()))
			return
		}
		fmt.Println(string(statusJSON))
	},
}
