package processrouter

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestDiagnosticLogsBoundedAndClearable(t *testing.T) {
	ClearDiagnosticLogs()
	defer ClearDiagnosticLogs()
	recordDiagnostic("first")
	firstID := DiagnosticLogs()[0].ID
	for i := 0; i < maxDiagnosticEntries+2; i++ {
		recordDiagnostic("route")
	}
	entries := DiagnosticLogs()
	if len(entries) != maxDiagnosticEntries {
		t.Fatalf("got %d entries, want %d", len(entries), maxDiagnosticEntries)
	}
	if entries[0].ID != firstID+3 || entries[len(entries)-1].ID != firstID+maxDiagnosticEntries+2 {
		t.Fatalf("unexpected sequence range: %d..%d", entries[0].ID, entries[len(entries)-1].ID)
	}
	entries[0].Message = "changed"
	if DiagnosticLogs()[0].Message != "route" {
		t.Fatal("returned entries alias the diagnostic store")
	}
	ClearDiagnosticLogs()
	if len(DiagnosticLogs()) != 0 {
		t.Fatal("clear left diagnostic entries behind")
	}
	recordDiagnostic("fresh")
	if DiagnosticLogs()[0].ID != firstID+maxDiagnosticEntries+3 {
		t.Fatal("sequence ID reused after clear")
	}
}

func TestDiagnosticLevelsAndUTF8Bounds(t *testing.T) {
	for _, test := range []struct{ message, level string }{
		{"diagnostic engine=[PID] Owner unresolved after retry: TCP", "debug"},
		{"diagnostic engine=[PACKET] Loopback redirect", "debug"},
		{"diagnostic engine=Failed to open WinDivert (5): Access denied", "error"},
		{"diagnostic engine=SOCKS5: CONNECT failed (reply=5)", "error"},
		{"diagnostic engine=[OWNER] Failed to open WinDivert FLOW layer", "error"},
		{"diagnostic engine=[UDP RELAY ERROR] sendto proxy failed", "error"},
		{"diagnostic engine=Warning: No proxy configs configured", "warning"},
		{"diagnostic route process=error.exe pid=1 destination=example.com:443 result=DIRECT", "info"},
		{"diagnostic engine=Added rule for process 'Failed to.exe'", "info"},
		{"diagnostic engine=ProxyBridge started", "info"},
	} {
		if got := nativeDiagnosticLevel(test.message); got != test.level {
			t.Errorf("%s: got %s, want %s", test.message, got, test.level)
		}
	}
	ClearDiagnosticLogs()
	defer ClearDiagnosticLogs()
	recordDiagnosticAt("error", strings.Repeat("測", 2000))
	entry := DiagnosticLogs()[0]
	if entry.Level != "error" || len(entry.Message) > 4096 || !utf8.ValidString(entry.Message) {
		t.Fatalf("invalid bounded entry: %+v", entry)
	}
	encoded, err := json.Marshal(entry)
	if err != nil || !strings.Contains(string(encoded), `"level":"error"`) {
		t.Fatalf("missing wire level: %s, %v", encoded, err)
	}
}
