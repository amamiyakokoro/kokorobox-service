package cmd

import (
	"github.com/amamiyakokoro/kokorobox-service/i18n"
	"github.com/amamiyakokoro/kokorobox-service/log"
	"github.com/amamiyakokoro/kokorobox-service/route"

	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: i18n.DefaultText("启动 KokoroBox 服务（测试用）"),
	Run: func(cmd *cobra.Command, args []string) {
		if err := route.StartHTTP("127.0.0.1:10002"); err != nil {
			log.Fatal(err)
		}
	},
}
