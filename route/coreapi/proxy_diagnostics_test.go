package coreapi

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	corepkg "github.com/amamiyakokoro/kokorobox-service/core"
)

func configResponse(body string) func(context.Context) (*http.Response, error) {
	return func(context.Context) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	}
}
func TestRuntimeDiagnosticsUseLoadedPortAndKeepHealthStatesSeparate(t *testing.T) {
	cases := []struct {
		name, config                 string
		running, listener, connected bool
		wantPort                     int
		wantReady                    bool
	}{
		{"healthy", `{"mixed-port":18327,"port":18328,"secret":"PRIVATE"}`, true, true, true, 18327, true},
		{"http fallback", `{"mixed-port":0,"port":18328}`, true, true, true, 18328, true},
		{"listener unavailable", `{"mixed-port":18329}`, true, false, false, 18329, true},
		{"outbound unavailable", `{"mixed-port":18330}`, true, true, false, 18330, true},
		{"config failed", `invalid`, true, false, false, 0, false},
		{"no HTTP proxy", `{"mixed-port":0,"port":0}`, true, false, false, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			outboundCalls := 0
			result := collectProxyRuntimeDiagnostics(context.Background(), c.running, "", false, configResponse(c.config),
				func(_ context.Context, address string) runtimeListenerState {
					_, port, _ := net.SplitHostPort(address)
					if port == "" {
						t.Fatal("missing runtime port")
					}
					return runtimeListenerState{Available: c.listener, ErrorCode: map[bool]string{false: "connection-refused"}[c.listener]}
				}, func(context.Context, string) runtimeConnectivityState {
					outboundCalls++
					return runtimeConnectivityState{Available: c.connected, Outcome: map[bool]string{true: "success", false: "outbound-failed"}[c.connected], ErrorCode: map[bool]string{false: "timeout"}[c.connected]}
				})
			if result.Core.Ready != c.wantReady || result.Listener.Available != c.listener || result.Connectivity.Available != c.connected {
				t.Fatalf("unexpected health: %+v", result)
			}
			if c.wantPort == 0 {
				if result.Proxy.Port != nil {
					t.Fatal("inferred a static port")
				}
			} else if result.Proxy.Port == nil || *result.Proxy.Port != c.wantPort {
				t.Fatalf("wrong runtime port: %+v", result.Proxy)
			}
			if !c.listener && outboundCalls != 0 {
				t.Fatal("probed public endpoint without listener")
			}
			data, _ := json.Marshal(result)
			if strings.Contains(string(data), "PRIVATE") || strings.Contains(string(data), "secret") {
				t.Fatal("private config leaked")
			}
		})
	}
}
func TestRuntimeDiagnosticsStoppedAndControllerErrorsArePartial(t *testing.T) {
	for _, direct := range []bool{false, true} {
		result := collectProxyRuntimeDiagnostics(context.Background(), false, "core-start-failed", direct,
			func(context.Context) (*http.Response, error) { return nil, syscall.ECONNREFUSED },
			func(context.Context, string) runtimeListenerState {
				t.Fatal("listener called without runtime endpoint")
				return runtimeListenerState{}
			},
			func(context.Context, string) runtimeConnectivityState {
				t.Fatal("outbound called without runtime endpoint")
				return runtimeConnectivityState{}
			})
		if result.Core.Running == nil || *result.Core.Running || result.Proxy.Port != nil || result.Core.ErrorCode != "core-start-failed" {
			t.Fatalf("wrong stopped result: %+v", result)
		}
	}
	result := collectProxyRuntimeDiagnostics(context.Background(), true, "", false,
		func(ctx context.Context) (*http.Response, error) {
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("missing timeout")
			}
			return nil, context.DeadlineExceeded
		}, nil, nil)
	if result.Core.Running == nil || !*result.Core.Running || result.Core.Ready || result.Core.ErrorCode != "timeout" {
		t.Fatalf("configuration timeout discarded running state: %+v", result)
	}
}
func TestProxyDiagnosticsEndpointReturnsStoppedStateWithoutMutation(t *testing.T) {
	previous := cm
	cm = corepkg.NewCoreManager()
	t.Cleanup(func() { cm = previous })
	w := httptest.NewRecorder()
	coreProxyDiagnostics(w, httptest.NewRequest("GET", "/core/proxy-diagnostics", nil))
	var result proxyRuntimeDiagnostics
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || result.Core.Running == nil || *result.Core.Running || result.Proxy.Port != nil {
		t.Fatalf("bad endpoint response: %s", w.Body.String())
	}
}

func TestRealListenerAndCONNECTErrors(t *testing.T) {
	for _, c := range []struct{ name, response, code string }{
		{"rejected", "HTTP/1.1 502 Bad Gateway\r\n\r\n", "tunnel-rejected"},
		{"authentication", "HTTP/1.1 407 Proxy Authentication Required\r\n\r\n", "proxy-authentication"},
		{"timeout", "", "timeout"},
	} {
		t.Run(c.name, func(t *testing.T) {
			proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "CONNECT" || r.Host != "www.gstatic.com:443" || r.Header.Get("Proxy-Authorization") != "" {
					t.Errorf("wrong proxy request: %s %s", r.Method, r.Host)
				}
				conn, _, err := w.(http.Hijacker).Hijack()
				if err != nil {
					return
				}
				defer conn.Close()
				if c.response != "" {
					_, _ = io.WriteString(conn, c.response)
				} else {
					time.Sleep(80 * time.Millisecond)
				}
			}))
			defer proxy.Close()
			endpoint := strings.TrimPrefix(proxy.URL, "http://")
			if !probeLocalProxyListener(context.Background(), endpoint).Available {
				t.Fatal("listener should be available")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
			defer cancel()
			result := probeOutboundProxy(ctx, endpoint, connectivityEndpoint)
			if result.Outcome != "outbound-failed" || result.Available || result.ErrorCode != c.code {
				t.Fatalf("wrong result: %+v", result)
			}
		})
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := listener.Addr().String()
	_ = listener.Close()
	if result := probeLocalProxyListener(context.Background(), endpoint); result.Available || result.ErrorCode != "connection-refused" {
		t.Fatalf("closed port accepted: %+v", result)
	}
	if result := probeOutboundProxy(context.Background(), endpoint, connectivityEndpoint); result.Outcome != "unreachable" || result.ErrorCode != "connection-refused" {
		t.Fatalf("proxy dial failure confused with outbound: %+v", result)
	}
}

func TestCONNECTStillRequiresVerifiedHTTPS204(t *testing.T) {
	status := http.StatusNoContent
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
	defer target.Close()
	var mu sync.Mutex
	sockets := map[net.Conn]bool{}
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "CONNECT" {
			t.Error("request bypassed CONNECT")
			return
		}
		upstream, err := net.Dial("tcp", r.Host)
		if err != nil {
			t.Error(err)
			return
		}
		conn, buffer, err := w.(http.Hijacker).Hijack()
		if err != nil {
			upstream.Close()
			return
		}
		mu.Lock()
		sockets[conn] = true
		sockets[upstream] = true
		mu.Unlock()
		_, _ = io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
		go func() { _, _ = io.Copy(upstream, buffer); upstream.Close() }()
		_, _ = io.Copy(conn, upstream)
		conn.Close()
	}))
	defer func() {
		mu.Lock()
		for conn := range sockets {
			conn.Close()
		}
		mu.Unlock()
		proxy.Close()
	}()
	endpoint := strings.TrimPrefix(proxy.URL, "http://")
	untrusted := probeOutboundProxy(context.Background(), endpoint, target.URL)
	if untrusted.Available || untrusted.ErrorCode != "tls-failed" {
		t.Fatalf("unverified certificate accepted: %+v", untrusted)
	}
	roots := x509.NewCertPool()
	roots.AddCert(target.Certificate())
	for _, code := range []int{204, 302, 500} {
		status = code
		transport := proxyDiagnosticTransport(endpoint)
		transport.TLSClientConfig = &tls.Config{RootCAs: roots}
		result := requestProxyConnectivity(context.Background(), target.URL, transport)
		if result.Available != (code == 204) {
			t.Fatalf("HTTP %d: %+v", code, result)
		}
		if code == 204 && result.LatencyMs == nil {
			t.Fatal("missing latency")
		}
		if code != 204 && result.ErrorCode != "unexpected-response" {
			t.Fatal(result.ErrorCode)
		}
	}
}
func TestNetworkErrorsAreStableAndDoNotIncludeSecrets(t *testing.T) {
	if networkErrorCode(errors.New("PRIVATE URL token=secret")) != "network-error" {
		t.Fatal("error text leaked")
	}
}
