package httphelper

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocaleMiddlewareUsesAcceptLanguage(t *testing.T) {
	handler := LocaleMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SendJSON(w, "success", "Core started successfully")
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "zh-TW")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if got, want := recorder.Header().Get("Content-Language"), "zh-TW"; got != want {
		t.Fatalf("Content-Language = %q, want %q", got, want)
	}
	var response Response
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if got, want := response.Message, "核心啟動成功"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

func TestLocaleMiddlewareDefaultsToEnglish(t *testing.T) {
	handler := LocaleMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SendJSON(w, "success", "Core started successfully")
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	var response Response
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if got, want := response.Message, "Core started successfully"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

func TestLocaleMiddlewareSupportsSimplifiedChinese(t *testing.T) {
	handler := LocaleMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SendJSON(w, "success", "Core started successfully")
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "zh-CN")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if got, want := recorder.Header().Get("Content-Language"), "zh-CN"; got != want {
		t.Fatalf("Content-Language = %q, want %q", got, want)
	}
	var response Response
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if got, want := response.Message, "核心启动成功"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}
