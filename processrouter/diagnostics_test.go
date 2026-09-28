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
		{"diagnostic engine=Added rule ID: 5 for process 'Failed to.exe' (Action: 2)", "debug"},
		{"diagnostic engine=Deleted rule ID: 5", "debug"},
		{"diagnostic engine=Updated rule ID: 5 (ProxyConfigId: 0)", "debug"},
		{"diagnostic engine=Enabled rule ID: 5", "debug"},
		{"diagnostic engine=Disabled rule ID: 5", "debug"},
		{"diagnostic engine=Moved rule ID 5 to position 1", "debug"},
		{"diagnostic engine=Rule: game.exe -> BLOCK", "debug"},
		{"diagnostic engine=level=debug Pending rule: game.exe -> BLOCK", "debug"},
		{"diagnostic engine=level=debug Temporary BLOCK guard installed for rule replacement", "debug"},
		{"diagnostic engine=Active routing rules committed: applications=1 exclusions=2", "info"},
		{"diagnostic engine=Active application rule: priority=1 process=error.exe action=BLOCK protocol=BOTH", "info"},
		{"diagnostic engine=Active exclusion: local destinations -> DIRECT (hosts=127.*.*.*)", "info"},
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
