//go:build darwin

package bootstrap

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amamiyakokoro/kokorobox-service/route/pipectx"
)

func TestAuditTokenResolvesLiveUnsignedClient(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "kb-bootstrap-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socketPath := filepath.Join(dir, "service.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	result := make(chan string, 1)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer, ok := pipectx.RequestDarwinPeerInfo(r)
		if !ok {
			result <- "missing peer audit token"
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		verificationErr := verifyKokoroBoxProcess(peer.AuditToken)
		if verificationErr == nil {
			result <- "unsigned Go test client unexpectedly passed the release requirement"
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		result <- verificationErr.Error()
		w.WriteHeader(http.StatusNoContent)
	})}
	pipectx.ConfigureServer(server)
	go func() { _ = server.Serve(listener) }()
	defer server.Close()

	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
	}}
	response, err := (&http.Client{Transport: transport}).Get("http://localhost/")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("unexpected response status %d: %s", response.StatusCode, <-result)
	}
	if message := <-result; !strings.Contains(message, "signature rejected") {
		t.Fatalf("audit token did not reach code requirement validation: %s", message)
	}
}
