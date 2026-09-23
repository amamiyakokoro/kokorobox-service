package route

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
)

func TestMetaReportsAvailablePlatformCapabilities(t *testing.T) {
	w := httptest.NewRecorder()
	metaStatus(w, httptest.NewRequest(http.MethodGet, "/meta", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", w.Code)
	}
	var meta serviceMeta
	if err := json.Unmarshal(w.Body.Bytes(), &meta); err != nil {
		t.Fatal(err)
	}
	if meta.APIVersion != ServiceAPIVersion || meta.ServiceVersion != ServiceVersion {
		t.Fatalf("invalid service identity: %+v", meta)
	}
	if !meta.Capabilities.CoreDesiredState || !meta.Capabilities.SysproxyLease || !meta.Capabilities.SysproxyEvents {
		t.Fatalf("missing service capabilities: %+v", meta.Capabilities)
	}
	if meta.Capabilities.SysproxyNetworkReconcile != (runtime.GOOS == "darwin") ||
		meta.Capabilities.DNSLease != (runtime.GOOS == "darwin") ||
		meta.Capabilities.ProcessRouter != (runtime.GOOS == "windows" || runtime.GOOS == "linux") {
		t.Fatalf("incorrect platform capabilities: %+v", meta.Capabilities)
	}
}
