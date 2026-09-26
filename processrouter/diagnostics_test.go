package processrouter

import "testing"

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
