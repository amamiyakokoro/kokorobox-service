package sysapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUwpLoopbackRejectsInvalidRequestBeforeMutation(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"id":"S-1-15-2","enabled":true}`,
		`{"id":"0123456789abcdef","enabled":null}`,
		`{"id":"0123456789abcdefg","enabled":true}`,
		`{"id":"0123456789abcdef","enabled":"true"}`,
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/uwp-loopback", strings.NewReader(body))
		Router().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("body %s: got status %d, want 400", body, recorder.Code)
		}
	}
}
