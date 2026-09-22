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
	CoreDesiredState bool `json:"coreDesiredState"`
	SysproxyLease    bool `json:"sysproxyLease"`
	SysproxyEvents   bool `json:"sysproxyEvents"`
	DNSLease         bool `json:"dnsLease"`
	ProcessRouter    bool `json:"processRouter"`
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
			CoreDesiredState: true,
			SysproxyLease:    true,
			SysproxyEvents:   true,
			DNSLease:         runtime.GOOS == "darwin",
			ProcessRouter:    runtime.GOOS == "windows" || runtime.GOOS == "linux",
		},
	})
}
