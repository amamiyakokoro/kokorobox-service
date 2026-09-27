package log

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogSnapshotBoundsAndAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "service.log")
	content := strings.Repeat("a", maxLogTailBytes+100)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := readSnapshot(path, "session")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Offset != 100 || snapshot.End != int64(len(content)) || len(snapshot.Content) != maxLogTailBytes || snapshot.Session != "session" {
		t.Fatalf("invalid bounded snapshot: offset=%d end=%d bytes=%d", snapshot.Offset, snapshot.End, len(snapshot.Content))
	}
	if err := os.WriteFile(path, []byte("new log\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err = readSnapshot(path, "new-session")
	if err != nil || snapshot.Offset != 0 || snapshot.End != 8 || snapshot.Content != "new log\n" {
		t.Fatalf("invalid restarted snapshot: %+v, %v", snapshot, err)
	}
	if actual, _ := os.ReadFile(path); string(actual) != "new log\n" {
		t.Fatal("reading changed the log file")
	}
}

func TestMissingLogSnapshotIsEmpty(t *testing.T) {
	snapshot, err := readSnapshot(filepath.Join(t.TempDir(), "missing.log"), "session")
	if err != nil || snapshot.Content != "" || snapshot.End != 0 {
		t.Fatalf("missing log: %+v, %v", snapshot, err)
	}
}

func TestLogSnapshotStartsOnUTF8Boundary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "service.log")
	content := strings.Repeat("中", 200_000) + "\n{\"msg\":\"last\"}\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := readSnapshot(path, "session")
	if err != nil || strings.Contains(snapshot.Content, "\uFFFD") || snapshot.Offset+int64(len(snapshot.Content)) != snapshot.End {
		t.Fatalf("invalid UTF-8 tail: offset=%d end=%d error=%v", snapshot.Offset, snapshot.End, err)
	}
}
