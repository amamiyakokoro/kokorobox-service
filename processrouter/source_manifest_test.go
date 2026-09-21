package processrouter

import (
	"encoding/json"
	"os"
	"regexp"
	"testing"
)

func TestWindowsRuntimeSourceManifestPinsImmutableInputs(t *testing.T) {
	contents, err := os.ReadFile("native/source-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		ProxyBridgeRepository  string `json:"proxyBridgeRepository"`
		ProxyBridgeRevision    string `json:"proxyBridgeRevision"`
		WinDivertVersion       string `json:"winDivertVersion"`
		WinDivertURL           string `json:"winDivertUrl"`
		WinDivertArchiveSHA256 string `json:"winDivertArchiveSha256"`
	}
	if err := json.Unmarshal(contents, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.ProxyBridgeRepository != "https://github.com/amamiyakokoro/ProxyBridge.git" {
		t.Fatalf("unexpected ProxyBridge repository: %q", manifest.ProxyBridgeRepository)
	}
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(manifest.ProxyBridgeRevision) {
		t.Fatalf("ProxyBridge revision is not immutable: %q", manifest.ProxyBridgeRevision)
	}
	if manifest.WinDivertVersion == "" || manifest.WinDivertURL == "" {
		t.Fatal("WinDivert version and download URL must be pinned")
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(manifest.WinDivertArchiveSHA256) {
		t.Fatalf("WinDivert archive hash is invalid: %q", manifest.WinDivertArchiveSHA256)
	}
}
