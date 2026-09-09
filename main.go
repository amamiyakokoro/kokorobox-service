package main

import (
	"fmt"
	"github.com/amamiyakokoro/kokorobox-service/cmd"
	"github.com/amamiyakokoro/kokorobox-service/i18n"
	"os"
)

func main() {
	if err := cmd.MainCmd.Execute(); err != nil {
		if !cmd.IsReportedError(err) {
			fmt.Println(i18n.DefaultText(err.Error()))
		}
		os.Exit(1)
	}
}
