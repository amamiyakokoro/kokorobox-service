//go:build windows

package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/process"
	"github.com/spf13/cobra"
)

// This command is launched from an unprivileged copy of the bundled service
// executable outside the application directory. It waits until Desktop has
// exited, extracts one verified update archive, and starts KokoroBox again.
var portableUpdateCmd = &cobra.Command{
	Use:    "portable-update",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE:   runPortableUpdate,
}

var portableParentPID int32
var portableArchive string
var portableExtractor string
var portableApplication string

func init() {
	portableUpdateCmd.Flags().Int32Var(&portableParentPID, "parent-pid", 0, "Desktop process ID")
	portableUpdateCmd.Flags().StringVar(&portableArchive, "archive", "", "verified update archive")
	portableUpdateCmd.Flags().StringVar(&portableExtractor, "extractor", "", "bundled 7-Zip executable")
	portableUpdateCmd.Flags().StringVar(&portableApplication, "application", "", "KokoroBox executable")
	MainCmd.AddCommand(portableUpdateCmd)
}

func runPortableUpdate(_ *cobra.Command, _ []string) error {
	if portableParentPID <= 0 {
		return errors.New("invalid Desktop process ID")
	}
	for _, value := range []string{portableArchive, portableExtractor, portableApplication} {
		if !filepath.IsAbs(value) || strings.ContainsAny(value, "\x00\r\n") {
			return errors.New("portable update requires absolute local paths")
		}
	}
	if !strings.EqualFold(filepath.Ext(portableArchive), ".7z") ||
		!strings.EqualFold(filepath.Base(portableExtractor), "7za.exe") ||
		!strings.EqualFold(filepath.Base(portableApplication), "KokoroBox.exe") ||
		!strings.EqualFold(filepath.Dir(portableArchive), filepath.Dir(portableExtractor)) {
		return errors.New("invalid portable update paths")
	}
	for _, file := range []string{portableArchive, portableExtractor, portableApplication} {
		info, err := os.Stat(file)
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("portable update input is unavailable: %s", file)
		}
	}
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		alive, err := process.PidExists(portableParentPID)
		if err != nil {
			return fmt.Errorf("query Desktop process: %w", err)
		}
		if !alive {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if alive, _ := process.PidExists(portableParentPID); alive {
		return errors.New("Desktop did not exit before portable update")
	}
	targetDir := filepath.Dir(portableApplication)
	command := exec.Command(portableExtractor, "x", "-o"+targetDir, "-y", portableArchive)
	command.Dir = targetDir
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("extract portable update: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return exec.Command(portableApplication).Start()
}
