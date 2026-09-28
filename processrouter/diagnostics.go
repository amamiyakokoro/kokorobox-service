package processrouter

import (
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/amamiyakokoro/kokorobox-service/log"
)

const maxDiagnosticEntries = 1000

type DiagnosticEntry struct {
	ID      uint64 `json:"id"`
	Time    string `json:"time"`
	Message string `json:"message"`
	Level   string `json:"level"`
}

var diagnostics = struct {
	sync.Mutex
	nextID  uint64
	entries []DiagnosticEntry
}{}

func recordDiagnostic(message string) {
	recordDiagnosticAt("info", message)
}

func recordDiagnosticAt(level, message string) {
	message = strings.ToValidUTF8(strings.TrimSpace(message), "\uFFFD")
	if message == "" {
		return
	}
	if len(message) > 4096 {
		message = message[:4096]
		for !utf8.ValidString(message) {
			message = message[:len(message)-1]
		}
	}
	diagnostics.Lock()
	defer diagnostics.Unlock()
	diagnostics.nextID++
	diagnostics.entries = append(diagnostics.entries, DiagnosticEntry{
		ID:      diagnostics.nextID,
		Time:    time.Now().UTC().Format(time.RFC3339Nano),
		Message: message,
		Level:   level,
	})
	if len(diagnostics.entries) > maxDiagnosticEntries {
		diagnostics.entries = append([]DiagnosticEntry(nil), diagnostics.entries[len(diagnostics.entries)-maxDiagnosticEntries:]...)
	}
}

// ProxyBridge's legacy callback has no level argument. Prefer an explicit
// prefix when available; classify known diagnostics without inspecting paths,
// destinations, or process names for incidental words such as "error".
func nativeDiagnosticLevel(message string) string {
	text := strings.ToLower(strings.TrimSpace(message))
	if strings.HasPrefix(text, "diagnostic route ") {
		return "info"
	}
	text = strings.TrimPrefix(text, "diagnostic engine=")
	for _, prefix := range []string{"level=error", "[error]", "error:", "fatal:", "[udp relay error]"} {
		if strings.HasPrefix(text, prefix) {
			return "error"
		}
	}
	for _, prefix := range []string{"level=warn", "[warn]", "[warning]", "warning:", "warn:"} {
		if strings.HasPrefix(text, prefix) {
			return "warning"
		}
	}
	if strings.HasPrefix(text, "[pid]") || strings.HasPrefix(text, "[packet]") || strings.HasPrefix(text, "level=debug") {
		return "debug"
	}
	// These describe edits or the pre-commit startup dump, not the final policy.
	// Active-rule summaries have their own prefix and remain visible at info.
	for _, prefix := range []string{"added rule id:", "deleted rule id:", "updated rule id:", "enabled rule id:", "disabled rule id:", "moved rule id ", "rule: "} {
		if strings.HasPrefix(text, prefix) {
			return "debug"
		}
	}
	if strings.HasPrefix(text, "failed to ") || strings.HasPrefix(text, "invalid ") {
		return "error"
	}
	// An optional subsystem prefix (HTTP:, [OWNER], etc.) precedes the status.
	status := text
	if strings.HasPrefix(status, "[") {
		if end := strings.IndexByte(status, ']'); end >= 0 {
			status = strings.TrimSpace(status[end+1:])
		}
	} else if end := strings.Index(status, ": "); end >= 0 {
		status = status[end+2:]
	}
	if strings.HasPrefix(status, "failed to ") || strings.HasPrefix(status, "invalid ") ||
		strings.HasPrefix(status, "connect failed") || strings.HasPrefix(status, "no-auth negotiation failed") {
		return "error"
	}
	for _, prefix := range []string{"wsastartup failed", "socket creation failed", "bind failed", "listen failed", "memory allocation failed", "ipv6 listen failed"} {
		if strings.HasPrefix(text, prefix) {
			return "error"
		}
	}
	return "info"
}

func recordNativeDiagnostic(message string) {
	if strings.TrimSpace(message) == "" {
		return
	}
	level := nativeDiagnosticLevel(message)
	recordDiagnosticAt(level, message)
	switch level {
	case "error":
		log.Errorf("Native process router: %s", message)
	case "warning":
		log.Warnf("Native process router: %s", message)
	case "debug":
		log.Debugf("Native process router: %s", message)
	default:
		log.Printf("Native process router: %s", message)
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
