package auth

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/amamiyakokoro/kokorobox-service/route/httphelper"
)

const (
	authVersionV2       = "2"
	authVersionV3       = "3"
	maxTimestampDriftV2 = 30 * time.Second
)

type nonceStore struct {
	mu    sync.Mutex
	ttl   time.Duration
	seen  map[string]time.Time
	order []nonceEntry
}

type nonceEntry struct {
	key       string
	expiresAt time.Time
}

func newNonceStore(ttl time.Duration) *nonceStore {
	return &nonceStore{
		ttl:   ttl,
		seen:  make(map[string]time.Time),
		order: make([]nonceEntry, 0),
	}
}

func (s *nonceStore) Remember(key string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.evictExpiredLocked(now)

	if expiresAt, exists := s.seen[key]; exists && expiresAt.After(now) {
		return false
	}

	expiresAt := now.Add(s.ttl)
	s.seen[key] = expiresAt
	s.order = append(s.order, nonceEntry{
		key:       key,
		expiresAt: expiresAt,
	})
	return true
}

func (s *nonceStore) evictExpiredLocked(now time.Time) {
	evicted := 0
	for evicted < len(s.order) {
		entry := s.order[evicted]
		if entry.expiresAt.After(now) {
			break
		}

		if currentExpiresAt, exists := s.seen[entry.key]; exists && currentExpiresAt.Equal(entry.expiresAt) {
			delete(s.seen, entry.key)
		}
		evicted++
	}

	if evicted > 0 {
		s.order = append([]nonceEntry(nil), s.order[evicted:]...)
	}
}

var requestNonceStore = newNonceStore(2 * maxTimestampDriftV2)

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		km := GetKeyManager()
		if !km.IsInitialized() || !km.HasAuthorizedPrincipal() {
			httphelper.SendError(w, httphelper.ServiceUnavailable("Service is not initialized"))
			return
		}

		if err := km.VerifyRequestPrincipal(r); err != nil {
			httphelper.SendError(w, httphelper.Forbidden(fmt.Sprintf("Requestor is not authorized: %v", err)))
			return
		}

		version := r.Header.Get("X-Auth-Version")
		if version != authVersionV2 && version != authVersionV3 {
			httphelper.SendError(w, httphelper.Unauthorized("Only Auth V2/V3 is supported"))
			return
		}
		err := authenticateRequest(r, km, version)
		if err != nil {
			httphelper.SendError(w, err)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func RequireAuth(next http.Handler) http.Handler {
	return AuthMiddleware(next)
}

func authenticateRequest(r *http.Request, km *KeyManager, version string) error {
	timestamp := r.Header.Get("X-Timestamp")
	keyID := r.Header.Get("X-Key-Id")
	nonce := r.Header.Get("X-Nonce")
	contentHash := strings.ToLower(r.Header.Get("X-Content-SHA256"))
	signature := r.Header.Get("X-Signature")

	if timestamp == "" || keyID == "" || nonce == "" || contentHash == "" || signature == "" {
		return httphelper.Unauthorized("Missing authentication information")
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return httphelper.BadRequest("Invalid timestamp format")
	}

	requestTime := time.UnixMilli(ts)
	now := time.Now()
	timeDiff := now.Sub(requestTime)

	if timeDiff < -maxTimestampDriftV2 || timeDiff > maxTimestampDriftV2 {
		return httphelper.Unauthorized("Request expired or timestamp is invalid")
	}

	bodyHash, err := hashRequestBody(r)
	if err != nil {
		return err
	}
	if bodyHash != contentHash {
		return httphelper.Unauthorized("Request body digest does not match")
	}

	canonical, err := buildCanonicalRequest(r, timestamp, nonce, keyID, bodyHash, version)
	if err != nil {
		return httphelper.BadRequest(err.Error())
	}

	if err := km.VerifySignature(keyID, canonical, signature); err != nil {
		return httphelper.Unauthorized(err.Error())
	}

	nonceKey := keyID + ":" + timestamp + ":" + nonce
	if !requestNonceStore.Remember(nonceKey, now) {
		return httphelper.Conflict("Request has already been replayed")
	}

	return nil
}

func hashRequestBody(r *http.Request) (string, error) {
	var body []byte

	if r.Body != nil {
		rawBody, err := io.ReadAll(r.Body)
		if err != nil {
			return "", fmt.Errorf("Failed to read request body:  %w", err)
		}
		body = rawBody
		r.Body = io.NopCloser(bytes.NewReader(rawBody))
	}

	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func buildCanonicalRequest(r *http.Request, timestamp string, nonce string, keyID string, bodyHash string, version string) (string, error) {
	domain, err := canonicalDomain(version)
	if err != nil {
		return "", err
	}

	query, err := canonicalizeQuery(r.URL.RawQuery)
	if err != nil {
		return "", fmt.Errorf("Failed to normalize request parameters:  %w", err)
	}

	path := r.URL.EscapedPath()
	if path == "" {
		path = "/"
	}

	return strings.Join([]string{
		domain,
		timestamp,
		nonce,
		keyID,
		strings.ToUpper(r.Method),
		path,
		query,
		bodyHash,
	}, "\n"), nil
}

func canonicalDomain(version string) (string, error) {
	switch version {
	case authVersionV2:
		return "SPARKLE-AUTH-V2", nil
	case authVersionV3:
		return "KOKOROBOX-AUTH-V3", nil
	default:
		return "", fmt.Errorf("Unsupported authentication version: %s", version)
	}
}

func canonicalizeQuery(rawQuery string) (string, error) {
	if rawQuery == "" {
		return "", nil
	}

	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return "", err
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0)
	for _, key := range keys {
		vals := append([]string(nil), values[key]...)
		sort.Strings(vals)
		for _, value := range vals {
			parts = append(parts, url.QueryEscape(key)+"="+url.QueryEscape(value))
		}
	}

	return strings.Join(parts, "&"), nil
}
