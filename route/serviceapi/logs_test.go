package serviceapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amamiyakokoro/kokorobox-service/log"
)

func TestServiceLogsEndpointReturnsBoundedSnapshot(t *testing.T) {
	w := httptest.NewRecorder()
	Router().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/logs", nil))
	if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("invalid response: status=%d cache=%q", w.Code, w.Header().Get("Cache-Control"))
	}
	var snapshot log.Snapshot
	if err := json.Unmarshal(w.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Session == "" || snapshot.End < snapshot.Offset || len(snapshot.Content) > 512*1024 {
		t.Fatal("invalid log snapshot")
	}
}
