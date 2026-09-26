package processrouter

import (
	"strings"
	"sync"
	"time"
)

const maxDiagnosticEntries = 1000

type DiagnosticEntry struct {
	ID      uint64 `json:"id"`
	Time    string `json:"time"`
	Message string `json:"message"`
}

var diagnostics = struct {
	sync.Mutex
	nextID  uint64
	entries []DiagnosticEntry
}{}

func recordDiagnostic(message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	diagnostics.Lock()
	defer diagnostics.Unlock()
	diagnostics.nextID++
	diagnostics.entries = append(diagnostics.entries, DiagnosticEntry{
		ID:      diagnostics.nextID,
		Time:    time.Now().UTC().Format(time.RFC3339Nano),
		Message: message,
	})
	if len(diagnostics.entries) > maxDiagnosticEntries {
		diagnostics.entries = append([]DiagnosticEntry(nil), diagnostics.entries[len(diagnostics.entries)-maxDiagnosticEntries:]...)
	}
}

func DiagnosticLogs() []DiagnosticEntry {
	diagnostics.Lock()
	defer diagnostics.Unlock()
	return append([]DiagnosticEntry{}, diagnostics.entries...)
}

func ClearDiagnosticLogs() {
	diagnostics.Lock()
	defer diagnostics.Unlock()
	diagnostics.entries = nil
}
