package cmd

import (
	"runtime"
	"testing"
)

func TestSysproxyCLIExposesOnlyStatus(t *testing.T) {
	commands := sysproxyCmd.Commands()
	if len(commands) != 1 || commands[0].Name() != "status" {
		t.Fatalf("sysproxy subcommands = %v, want status only", commands)
	}
	if sysproxyCmd.RunE == nil || sysproxyCmd.Args(sysproxyCmd, []string{"proxy"}) == nil {
		t.Fatal("removed proxy command must be rejected")
	}
}

func TestSysproxyStatusReportsFailure(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("unsupported XDG desktop failure injection is Linux-specific")
	}

	t.Setenv("XDG_CURRENT_DESKTOP", "unsupported-test-desktop")
	err := statusCmd.RunE(statusCmd, nil)
	if err == nil {
		t.Fatal("command returned nil for a sysproxy failure")
	}
	if !IsReportedError(err) {
		t.Fatalf("command error = %v, want reportedError", err)
	}
}
