package route

import (
	"net/http"
	"runtime"

	"github.com/go-chi/render"
)

// ServiceVersion is set by the release build. Development builds report "dev".
var ServiceVersion = "dev"

const ServiceAPIVersion = 1

type serviceCapabilities struct {
	SysproxyPACServer        bool `json:"sysproxyPacServer"`
	CoreDesiredState         bool `json:"coreDesiredState"`
	SysproxyLease            bool `json:"sysproxyLease"`
	SysproxyEvents           bool `json:"sysproxyEvents"`
	SysproxyNetworkReconcile bool `json:"sysproxyNetworkReconcile"`
	DNSLease                 bool `json:"dnsLease"`
	ProcessRouter            bool `json:"processRouter"`
	WindowsUwpLoopback       bool `json:"windowsUwpLoopback"`
	ProxyRuntimeDiagnostics  bool `json:"proxyRuntimeDiagnostics"`
	ServiceLogs              bool `json:"serviceLogs"`
}

type serviceMeta struct {
	ServiceVersion string              `json:"serviceVersion"`
	APIVersion     int                 `json:"apiVersion"`
	Capabilities   serviceCapabilities `json:"capabilities"`
}

func metaStatus(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, serviceMeta{
		ServiceVersion: ServiceVersion,
		APIVersion:     ServiceAPIVersion,
		Capabilities: serviceCapabilities{
			CoreDesiredState:         true,
			SysproxyPACServer:        true,
			SysproxyLease:            true,
			SysproxyEvents:           true,
			SysproxyNetworkReconcile: runtime.GOOS == "darwin",
			DNSLease:                 runtime.GOOS == "darwin",
			ProcessRouter:            runtime.GOOS == "windows" || runtime.GOOS == "linux",
			WindowsUwpLoopback:       runtime.GOOS == "windows",
			ServiceLogs:              true,
			ProxyRuntimeDiagnostics:  true,
		},
	})
}
