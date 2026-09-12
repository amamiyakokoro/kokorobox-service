//go:build linux

package processrouter

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestApplyProcessSnapshotLockedAssignsMatchingProcess(t *testing.T) {
	executablePath, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	tempDir := t.TempDir()
	group := "proxy-tcp"
	targetFile := filepath.Join(tempDir, linuxCgroupName, group, "cgroup.procs")
	if err := os.MkdirAll(filepath.Dir(targetFile), 0o755); err != nil {
		t.Fatal(err)
	}

	process := &linuxNativeProcess{
		controller: &linuxCgroupController{
			mode:        linuxCgroupV2,
			mountPoint:  tempDir,
			managedRoot: filepath.Join(tempDir, linuxCgroupName),
		},
		targets: []linuxProcessTarget{{
			matchKind: matchExecutablePath,
			pattern:   executablePath,
			group:     group,
		}},
		assigned:        make(map[int]string),
		originalCgroups: make(map[int]string),
	}

	pid := os.Getpid()
	if err := process.applyProcessSnapshotLocked([]linuxProcessSnapshot{{
		pid:            pid,
		executablePath: executablePath,
		executableName: filepath.Base(executablePath),
	}}); err != nil {
		t.Fatal(err)
	}

	if process.assigned[pid] != group {
		t.Fatalf("assigned[%d] = %q, want %q", pid, process.assigned[pid], group)
	}
	if process.originalCgroups[pid] == "" {
		t.Fatalf("original cgroup for pid %d was not recorded", pid)
	}

	payload, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(payload)) != strconv.Itoa(pid) {
		t.Fatalf("cgroup target contains %q, want %d", strings.TrimSpace(string(payload)), pid)
	}
}
