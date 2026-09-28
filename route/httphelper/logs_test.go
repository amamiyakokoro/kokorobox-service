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

func TestStatusPollingAndMutationAudit(t *testing.T) {
	for _, path := range []string{"/ping", "/test", "/meta", "/core/status", "/process-router/status", "/process-router/logs"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		if !isPollingRequest(r) || !shouldLogResponse(r, http.StatusForbidden) {
			t.Errorf("polling classification: %s", path)
		}
	}
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		r := httptest.NewRequest(method, "/process-router/rules", nil)
		if isPollingRequest(r) || !shouldLogRequest(r) || !shouldLogResponse(r, http.StatusOK) {
			t.Errorf("mutation must remain audited: %s", method)
		}
	}
}
