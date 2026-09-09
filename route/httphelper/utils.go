package httphelper

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/amamiyakokoro/kokorobox-service/i18n"
	"github.com/amamiyakokoro/kokorobox-service/log"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
)

type Response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type HTTPError struct {
	StatusCode int
	Message    string
}

// LocaleMiddleware selects the response language for one request. The
// process-wide KOKOROBOX_LOCALE value is used when Accept-Language is absent.
func LocaleMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Language", string(i18n.FromAcceptLanguage(r.Header.Get("Accept-Language"))))
		next.ServeHTTP(w, r)
	})
}

func (e *HTTPError) Error() string {
	return e.Message
}

func NewError(statusCode int, message string) error {
	return &HTTPError{
		StatusCode: statusCode,
		Message:    message,
	}
}

func BadRequest(message string) error {
	return NewError(http.StatusBadRequest, message)
}

func Unauthorized(message string) error {
	return NewError(http.StatusUnauthorized, message)
}

func Forbidden(message string) error {
	return NewError(http.StatusForbidden, message)
}

func Conflict(message string) error {
	return NewError(http.StatusConflict, message)
}

func ServiceUnavailable(message string) error {
	return NewError(http.StatusServiceUnavailable, message)
}

func DecodeRequest(r *http.Request, v any) error {
	_, err := DecodeOptionalRequest(r, v)
	return err
}

func DecodeOptionalRequest(r *http.Request, v any) (bool, error) {
	if r.Body == nil || r.Body == http.NoBody || r.ContentLength == 0 {
		return false, nil
	}
	if err := render.DecodeJSON(r.Body, v); err != nil {
		if errors.Is(err, io.EOF) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !shouldLogRequest(r) {
			next.ServeHTTP(w, r)
			return
		}

		startedAt := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		if strings.HasPrefix(ww.Header().Get("Content-Type"), "text/event-stream") {
			return
		}

		duration := time.Since(startedAt)
		status := ww.Status()
		if status == 0 {
			status = http.StatusOK
		}
		fields := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"bytes", ww.BytesWritten(),
			"duration", duration.String(),
			"duration_ms", float64(duration.Nanoseconds()) / float64(time.Millisecond),
		}
		if routePattern := chi.RouteContext(r.Context()).RoutePattern(); routePattern != "" {
			fields = append(fields, "route", routePattern)
		}

		switch {
		case status >= http.StatusInternalServerError:
			log.S().Errorw("HTTP request completed", fields...)
		case status >= http.StatusBadRequest:
			log.S().Warnw("HTTP request completed", fields...)
		default:
			log.S().Infow("HTTP request completed", fields...)
		}
	})
}

func shouldLogRequest(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		return false
	}
	if isCoreControllerRequest(r.URL.Path) {
		return false
	}
	if r.Header.Get("Upgrade") != "" || headerContainsToken(r.Header, "Connection", "upgrade") {
		return false
	}
	return !headerContainsToken(r.Header, "Accept", "text/event-stream")
}

func isCoreControllerRequest(path string) bool {
	return path == "/core/controller" || strings.HasPrefix(path, "/core/controller/")
}

func SendJSONWithStatus(w http.ResponseWriter, statusCode int, status string, message string) {
	w.Header().Set("Content-Type", "application/json")
	locale := i18n.Default()
	if value := w.Header().Get("Content-Language"); value != "" {
		locale = i18n.Normalize(value)
	}
	w.Header().Set("Content-Language", string(locale))
	w.WriteHeader(statusCode)
	resp := Response{
		Status:  status,
		Message: i18n.Text(locale, message),
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Failed to encode HTTP JSON response: %v", err)
	}
}

func SendJSON(w http.ResponseWriter, status string, message string) {
	SendJSONWithStatus(w, http.StatusOK, status, message)
}

func SendError(w http.ResponseWriter, err error) {
	if httpErr, ok := errors.AsType[*HTTPError](err); ok {
		SendJSONWithStatus(w, httpErr.StatusCode, "error", httpErr.Message)
		return
	}

	SendJSONWithStatus(w, http.StatusInternalServerError, "error", err.Error())
}
