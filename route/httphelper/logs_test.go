package httphelper

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLogPollingDoesNotGenerateMoreLogs(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/service/logs", nil)
	if shouldLogResponse(request, http.StatusOK) {
		t.Fatal("successful polling must not generate its own log entries")
	}
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError} {
		if !shouldLogResponse(request, status) {
			t.Fatalf("failed log access must remain auditable: %d", status)
		}
	}
	if !shouldLogResponse(httptest.NewRequest(http.MethodPost, "/service/restart", nil), http.StatusOK) {
		t.Fatal("service operations must still be logged")
	}
}
