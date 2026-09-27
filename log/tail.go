package log

import (
	"fmt"
	"io"
	"os"
	"time"
)

const maxLogTailBytes = 512 * 1024

var logSession = fmt.Sprintf("%d:%d", os.Getpid(), time.Now().UnixNano())

// Snapshot is a bounded, read-only view of the current service log. Offsets are
// byte positions, so clients can clear their view without truncating diagnostics.
type Snapshot struct {
	Session string `json:"session"`
	Offset  int64  `json:"offset"`
	End     int64  `json:"end"`
	Content string `json:"content"`
}

func ReadSnapshot() (Snapshot, error) {
	return readSnapshot(currentLogPath(), logSession)
}

func readSnapshot(path, session string) (Snapshot, error) {
	snapshot := Snapshot{Session: session}
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return snapshot, nil
	}
	if err != nil {
		return snapshot, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return snapshot, err
	}
	snapshot.Offset = max(0, info.Size()-maxLogTailBytes)
	data := make([]byte, info.Size()-snapshot.Offset)
	n, err := file.ReadAt(data, snapshot.Offset)
	if err != nil && err != io.EOF {
		return snapshot, err
	}
	snapshot.End = snapshot.Offset + int64(n)
	start := 0
	for start < n && data[start]&0xc0 == 0x80 {
		start++
	}
	snapshot.Offset += int64(start)
	snapshot.Content = string(data[start:n])
	return snapshot, nil
}
