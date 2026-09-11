//go:build darwin

package service

import (
	"errors"
	"testing"

	kservice "github.com/kardianos/service"
)

func TestParseLaunchctlPrint(t *testing.T) {
	tests := []struct {
		name   string
		output string
		err    error
		want   kservice.Status
		noJob  bool
	}{
		{name: "running state", output: "state = running", want: kservice.StatusRunning},
		{name: "running pid", output: "service = {\n\tpid = 42\n}", want: kservice.StatusRunning},
		{name: "registered but stopped", output: "state = exited", want: kservice.StatusStopped},
		{name: "not registered", output: "Could not find service", err: errors.New("exit status 113"), want: kservice.StatusUnknown, noJob: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseLaunchctlPrint(test.output, test.err)
			if got != test.want {
				t.Fatalf("status = %v, want %v", got, test.want)
			}
			if test.noJob != errors.Is(err, kservice.ErrNotInstalled) {
				t.Fatalf("ErrNotInstalled = %v, want %v", errors.Is(err, kservice.ErrNotInstalled), test.noJob)
			}
		})
	}
}
