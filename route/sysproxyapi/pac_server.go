package sysproxyapi

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

const maxPACScriptBytes = 1024 * 1024

// The service only serves PAC text. It never evaluates JavaScript.
type managedPACServer struct {
	server *http.Server
	url    string
}

func startManagedPACServer(script string) (*managedPACServer, error) {
	if len(script) == 0 || len(script) > maxPACScriptBytes {
		return nil, fmt.Errorf("PAC script must contain 1 to %d bytes", maxPACScriptBytes)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	server := &http.Server{
		ReadHeaderTimeout: 3 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       5 * time.Second,
		MaxHeaderBytes:    16 * 1024,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/pac" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/x-ns-proxy-autoconfig")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write([]byte(script))
		}),
	}
	pac := &managedPACServer{server: server, url: "http://" + listener.Addr().String() + "/pac"}
	go func() { _ = server.Serve(listener) }()
	return pac, nil
}

func (p *managedPACServer) close() {
	if p == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if p.server.Shutdown(ctx) != nil {
		_ = p.server.Close()
	}
}
