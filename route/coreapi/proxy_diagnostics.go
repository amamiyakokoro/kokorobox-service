package coreapi

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/amamiyakokoro/kokorobox-service/log"
	"github.com/go-chi/render"
)

const connectivityEndpoint = "https://www.gstatic.com/generate_204"
const controllerTimeout = 2500 * time.Millisecond
const listenerTimeout = 1200 * time.Millisecond
const connectivityTimeout = 5 * time.Second

type runtimeCoreState struct {
	Running   *bool  `json:"running"`
	Ready     bool   `json:"ready"`
	ErrorCode string `json:"errorCode,omitempty"`
}
type runtimeProxyEndpoint struct {
	Host string `json:"host"`
	Port *int   `json:"port"`
}
type runtimeListenerState struct {
	Available bool   `json:"available"`
	ErrorCode string `json:"errorCode,omitempty"`
}
type runtimeConnectivityState struct {
	Available bool   `json:"available"`
	Outcome   string `json:"outcome"`
	ErrorCode string `json:"errorCode,omitempty"`
	LatencyMs *int64 `json:"latencyMs,omitempty"`
}
type proxyRuntimeDiagnostics struct {
	Core         runtimeCoreState         `json:"core"`
	Proxy        runtimeProxyEndpoint     `json:"proxy"`
	Listener     runtimeListenerState     `json:"listener"`
	Connectivity runtimeConnectivityState `json:"connectivity"`
}

// direct=true supports the existing Desktop-owned core without accepting arbitrary
// endpoints or a guessed port from the caller. The service reads the live controller.
func coreProxyDiagnostics(w http.ResponseWriter, r *http.Request) {
	direct := r.URL.Query().Get("direct") == "true"
	running, lastError := cm.RuntimeDiagnosticState()
	network, address, err := cm.ControllerEndpoint()
	if direct {
		network, address = directCoreControllerEndpoint()
	}
	if err != nil && !direct {
		network, address = "", ""
	}
	result := collectProxyRuntimeDiagnostics(r.Context(), running, lastError, direct, func(ctx context.Context) (*http.Response, error) {
		if address == "" {
			return nil, errors.New("controller-unavailable")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost/configs", nil)
		if err != nil {
			return nil, err
		}
		// A private per-call transport cannot retain a previous core's IPC connection.
		transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			if direct {
				return dialDirectCoreController(ctx)
			}
			return dialCoreController(ctx, network, address)
		}}
		defer transport.CloseIdleConnections()
		return (&http.Client{Transport: transport, Timeout: controllerTimeout}).Do(req)
	}, probeLocalProxyListener, func(ctx context.Context, endpoint string) runtimeConnectivityState {
		if r.URL.Query().Get("probe") == "false" {
			return runtimeConnectivityState{Outcome: "unreachable", ErrorCode: "not-tested"}
		}
		return probeOutboundProxy(ctx, endpoint, connectivityEndpoint)
	})
	render.JSON(w, r, result)
}

func collectProxyRuntimeDiagnostics(ctx context.Context, running bool, lastError string, direct bool,
	readConfig func(context.Context) (*http.Response, error),
	listener func(context.Context, string) runtimeListenerState,
	connectivity func(context.Context, string) runtimeConnectivityState,
) proxyRuntimeDiagnostics {
	result := proxyRuntimeDiagnostics{
		Core:         runtimeCoreState{Running: &running, ErrorCode: lastError},
		Proxy:        runtimeProxyEndpoint{Host: "127.0.0.1"},
		Listener:     runtimeListenerState{ErrorCode: "runtime-port-unavailable"},
		Connectivity: runtimeConnectivityState{Outcome: "unreachable", ErrorCode: "runtime-port-unavailable"},
	}
	if direct {
		result.Core.Running = nil
	}
	configCtx, cancel := context.WithTimeout(ctx, controllerTimeout)
	defer cancel()
	response, err := readConfig(configCtx)
	if err != nil {
		result.Core.ErrorCode = "core-config-unavailable"
		if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
			result.Core.ErrorCode = "timeout"
		}
		if direct && (errors.Is(err, syscall.ECONNREFUSED) || (errors.Is(err, syscall.ENOENT) || errors.Is(err, os.ErrNotExist))) {
			stopped := false
			result.Core.Running = &stopped
		}
		if lastError != "" {
			result.Core.ErrorCode = lastError
		}
		log.Printf("[ProxyRuntime] Configuration unavailable: %s", result.Core.ErrorCode)
		return result
	}
	defer response.Body.Close()
	loaded := true
	result.Core.Running = &loaded
	// Decode only the port fields; do not return or log the full configuration.
	var ports struct {
		MixedPort int `json:"mixed-port"`
		HTTPPort  int `json:"port"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 1024*1024)).Decode(&ports) != nil {
		result.Core.ErrorCode = "core-config-failed"
		return result
	}
	port := ports.MixedPort
	if port <= 0 {
		port = ports.HTTPPort
	}
	if port <= 0 || port > 65535 {
		result.Core.ErrorCode = "runtime-port-unavailable"
		return result
	}
	result.Proxy.Port = &port
	result.Core.Ready = true
	result.Core.ErrorCode = ""
	endpoint := net.JoinHostPort(result.Proxy.Host, strconv.Itoa(port))
	log.Printf("[ProxyRuntime] Core running; runtime endpoint: %s", endpoint)
	result.Listener = listener(ctx, endpoint)
	log.Printf("[ProxyRuntime] Listener reachable: %t", result.Listener.Available)
	// Never issue the public request when the local endpoint is unavailable.
	if !result.Listener.Available {
		result.Connectivity.ErrorCode = result.Listener.ErrorCode
		return result
	}
	result.Connectivity = connectivity(ctx, endpoint)
	if result.Connectivity.Available {
		log.Printf("[ProxyRuntime] Connectivity successful")
	} else {
		log.Printf("[ProxyRuntime] Connectivity test failed: %s", result.Connectivity.ErrorCode)
	}
	return result
}

func probeLocalProxyListener(ctx context.Context, endpoint string) runtimeListenerState {
	dialer := net.Dialer{Timeout: listenerTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", endpoint)
	if err != nil {
		return runtimeListenerState{ErrorCode: networkErrorCode(err)}
	}
	_ = conn.Close()
	return runtimeListenerState{Available: true}
}

func probeOutboundProxy(ctx context.Context, endpoint, target string) runtimeConnectivityState {
	return requestProxyConnectivity(ctx, target, proxyDiagnosticTransport(endpoint))
}

func proxyDiagnosticTransport(endpoint string) *http.Transport {
	// An explicit proxy avoids environment/system proxies and direct fallback.
	proxy := &url.URL{Scheme: "http", Host: endpoint}
	transport := &http.Transport{
		Proxy:                 http.ProxyURL(proxy),
		DialContext:           (&net.Dialer{Timeout: listenerTimeout}).DialContext,
		TLSHandshakeTimeout:   connectivityTimeout,
		ResponseHeaderTimeout: connectivityTimeout,
		OnProxyConnectResponse: func(_ context.Context, _ *url.URL, _ *http.Request, response *http.Response) error {
			if response.StatusCode == http.StatusProxyAuthRequired {
				return errors.New("proxy-authentication")
			}
			if response.StatusCode != http.StatusOK {
				return errors.New("tunnel-rejected")
			}
			return nil
		},
	}
	return transport
}

func requestProxyConnectivity(ctx context.Context, target string, transport *http.Transport) runtimeConnectivityState {
	started := time.Now()
	timeoutCtx, cancel := context.WithTimeout(ctx, connectivityTimeout)
	defer cancel()
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: connectivityTimeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequestWithContext(timeoutCtx, http.MethodGet, target, nil)
	if err != nil {
		return runtimeConnectivityState{Outcome: "outbound-failed", ErrorCode: "network-error"}
	}
	req.Header.Set("User-Agent", "KokoroBox-ProxyDiagnostics")
	response, err := client.Do(req)
	if err != nil {
		code := networkErrorCode(err)
		outcome := "outbound-failed"
		// Only failure dialing the proxy is 'unreachable'; tunnel/outbound failures
		// must not be confused with a closed local port.
		var operation *net.OpError
		if errors.As(err, &operation) && (operation.Op == "dial" || operation.Op == "proxyconnect") {
			outcome = "unreachable"
		}
		return runtimeConnectivityState{Outcome: outcome, ErrorCode: code}
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		return runtimeConnectivityState{Outcome: "outbound-failed", ErrorCode: "unexpected-response"}
	}
	latency := time.Since(started).Milliseconds()
	return runtimeConnectivityState{Available: true, Outcome: "success", LatencyMs: &latency}
}
func isTimeout(err error) bool { var e net.Error; return errors.As(err, &e) && e.Timeout() }
func networkErrorCode(err error) string {
	if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
		return "timeout"
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return "connection-refused"
	}
	// Only match our own fixed errors. Never return transport error text.
	if strings.Contains(err.Error(), "proxy-authentication") {
		return "proxy-authentication"
	}
	if strings.Contains(err.Error(), "tunnel-rejected") {
		return "tunnel-rejected"
	}
	var unknownAuthority x509.UnknownAuthorityError
	var certificateInvalid x509.CertificateInvalidError
	if errors.As(err, &unknownAuthority) || errors.As(err, &certificateInvalid) {
		return "tls-failed"
	}
	return "network-error"
}
