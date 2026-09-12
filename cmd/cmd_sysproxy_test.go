package cmd

import "testing"

func TestSysproxyCommandsReportFailures(t *testing.T) {
	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "proxy",
			run: func() error {
				restoreServer, restoreBypass := server, bypass
				defer func() {
					server, bypass = restoreServer, restoreBypass
				}()
				server = "127.0.0.1:7890"
				bypass = "localhost"
				return proxyCmd.RunE(proxyCmd, nil)
			},
		},
		{
			name: "pac",
			run: func() error {
				restore := pacUrl
				defer func() { pacUrl = restore }()
				pacUrl = "http://127.0.0.1:10000/pac"
				return pacCmd.RunE(pacCmd, nil)
			},
		},
		{name: "disable", run: func() error { return disableCmd.RunE(disableCmd, nil) }},
		{name: "status", run: func() error { return statusCmd.RunE(statusCmd, nil) }},
	}

	t.Setenv("XDG_CURRENT_DESKTOP", "unsupported-test-desktop")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if err == nil {
				t.Fatal("command returned nil for a sysproxy failure")
			}
			if !IsReportedError(err) {
				t.Fatalf("command error = %v, want reportedError", err)
			}
		})
	}
}
