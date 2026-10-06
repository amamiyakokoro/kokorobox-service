//go:build !windows

package core

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestProfileValidationBoundsAndErrors(t *testing.T) {
	tests := []struct {
		command string
		args    []string
		outcome string
		timeout time.Duration
	}{
		{"/bin/sh", []string{"-c", "echo level=error invalid-profile >&2; exit 1"}, "invalid", time.Second},
		{"/bin/sleep", []string{"30"}, "timeout", 20 * time.Millisecond},
		{"/usr/bin/yes", []string{"diagnostic"}, "output-limit", time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.outcome, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()
			result, err := runProfileValidation(ctx, newCoreCommand(exec.Command(tt.command, tt.args...), nil))
			if err != nil {
				t.Fatal(err)
			}
			if result.Outcome != tt.outcome || len(result.Output) > validationOutputLimit {
				t.Fatal(result.Outcome)
			}
			if tt.outcome == "invalid" && !strings.Contains(result.Output, "invalid-profile") {
				t.Fatal("diagnostic output lost")
			}
		})
	}
}
func TestProfileValidationRejectsArbitraryExecutables(t *testing.T) {
	_, err := NewCoreManager().ValidateProfile(context.Background(), ProfileValidationRequest{Executable: "/bin/sh", ConfigPath: "/tmp/config", WorkDir: "/tmp"})
	if err == nil {
		t.Fatal("arbitrary executable accepted")
	}
}
