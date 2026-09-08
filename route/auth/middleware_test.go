package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuildCanonicalRequestSupportsV2AndV3(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "http://service/rules?b=2&a=last&a=first", nil)

	for _, testCase := range []struct {
		version string
		domain  string
	}{
		{authVersionV2, "SPARKLE-AUTH-V2"},
		{authVersionV3, "KOKOROBOX-AUTH-V3"},
	} {
		t.Run(testCase.version, func(t *testing.T) {
			canonical, err := buildCanonicalRequest(request, "1", "nonce", "key", "hash", testCase.version)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(canonical, testCase.domain+"\n") {
				t.Fatalf("canonical domain = %q, want %q", canonical, testCase.domain)
			}
			if !strings.Contains(canonical, "\n/rules\na=first&a=last&b=2\n") {
				t.Fatalf("canonical query was not normalized: %q", canonical)
			}
		})
	}
}

func TestBuildCanonicalRequestRejectsUnknownVersion(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://service/ping", nil)
	if _, err := buildCanonicalRequest(request, "1", "nonce", "key", "hash", "4"); err == nil {
		t.Fatal("expected unsupported auth version to fail")
	}
}
