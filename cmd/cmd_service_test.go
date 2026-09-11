package cmd

import (
	"testing"

	appservice "github.com/amamiyakokoro/kokorobox-service/service"
)

func TestServiceInitNextTransition(t *testing.T) {
	tests := []struct {
		name          string
		status        appservice.Status
		changed       bool
		ensureRunning bool
		want          serviceInitTransition
	}{
		{name: "running unchanged", status: appservice.StatusRunning, ensureRunning: true, want: serviceInitNoAction},
		{name: "running changed", status: appservice.StatusRunning, changed: true, ensureRunning: true, want: serviceInitRestart},
		{name: "stopped legacy init", status: appservice.StatusStopped, changed: true, want: serviceInitNoAction},
		{name: "stopped bootstrap", status: appservice.StatusStopped, changed: true, ensureRunning: true, want: serviceInitStart},
		{name: "unknown bootstrap", status: appservice.StatusUnknown, ensureRunning: true, want: serviceInitStart},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := serviceInitNextTransition(tt.status, tt.changed, tt.ensureRunning); got != tt.want {
				t.Fatalf("serviceInitNextTransition() = %v, want %v", got, tt.want)
			}
		})
	}
}
