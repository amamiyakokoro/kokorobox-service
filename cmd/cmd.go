package cmd

import (
	"errors"
	"runtime"

	"github.com/amamiyakokoro/kokorobox-service/i18n"
	"github.com/amamiyakokoro/kokorobox-service/identity"
	"github.com/spf13/cobra"
)

var (
	server string
	bypass string
	pacUrl string

	device           string
	onlyActiveDevice bool
	useRegistry      bool

	listen      string
	defaultAddr string
	locale      string
)

var MainCmd = &cobra.Command{
	Use:           identity.ServiceExecutable,
	Short:         identity.ServiceDisplayName,
	SilenceErrors: true,
	SilenceUsage:  true,
}

type reportedError struct {
	err error
}

func (e reportedError) Error() string {
	return e.err.Error()
}

func (e reportedError) Unwrap() error {
	return e.err
}

func newReportedError(err error) error {
	return reportedError{err: err}
}

func IsReportedError(err error) bool {
	_, ok := errors.AsType[reportedError](err)
	return ok
}

func init() {
	if runtime.GOOS == "windows" {
		defaultAddr = identity.WindowsServicePipe
	} else {
		defaultAddr = identity.UnixServiceSocket
	}

	MainCmd.AddCommand(sysproxyCmd)
	MainCmd.AddCommand(serverCmd)
	MainCmd.AddCommand(serviceCmd)
	MainCmd.AddCommand(processRouterCmd)

	processRouterCmd.AddCommand(processRouterFirewallCmd)
	processRouterFirewallCmd.AddCommand(processRouterFirewallEnsureCmd)
	processRouterFirewallCmd.AddCommand(processRouterFirewallCheckCmd)
	processRouterFirewallCmd.AddCommand(processRouterFirewallRemoveCmd)

	sysproxyCmd.AddCommand(proxyCmd)
	sysproxyCmd.AddCommand(pacCmd)
	sysproxyCmd.AddCommand(disableCmd)
	sysproxyCmd.AddCommand(statusCmd)

	MainCmd.PersistentFlags().BoolVarP(&onlyActiveDevice, "only-active-device", "a", false, i18n.DefaultText("Apply only to active network devices"))
	MainCmd.PersistentFlags().BoolVarP(&useRegistry, "use-registry", "r", false, i18n.DefaultText("Use registry settings"))
	MainCmd.PersistentFlags().StringVarP(&device, "device", "d", "", i18n.DefaultText("Specify network device"))
	MainCmd.PersistentFlags().StringVarP(&listen, "listen", "l", defaultAddr, i18n.DefaultText("Listen address"))
	MainCmd.PersistentFlags().StringVar(&locale, "locale", string(i18n.Default()), i18n.DefaultText("Output language: en, zh-CN, or zh-TW"))
	MainCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		i18n.SetDefault(locale)
	}

	proxyCmd.Flags().StringVarP(&server, "server", "s", "", i18n.DefaultText("Proxy server address"))
	proxyCmd.Flags().StringVarP(&bypass, "bypass", "b", "", i18n.DefaultText("Bypass addresses"))

	pacCmd.Flags().StringVarP(&pacUrl, "url", "u", "", i18n.DefaultText("PAC URL"))
}
