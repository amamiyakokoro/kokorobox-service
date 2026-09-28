//go:build windows

package core

import (
	"context"
	"errors"
	"fmt"
	"github.com/amamiyakokoro/kokorobox-service/listen"
	"github.com/amamiyakokoro/kokorobox-service/listen/namedpipe"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
)

const trafficMonitorPipeAddress = `\\.\pipe\KokoroBox\mihomo`

func startTrafficMonitorProxy(launch *launchSession, sddl string) (func(), error) {
	if launch == nil || launch.controllerNet != "pipe" || launch.controllerAddr == "" {
		return nil, nil
	}

	listener, err := listen.ListenNamedPipe(trafficMonitorPipeAddress, sddl)
	if err != nil {
		return nil, fmt.Errorf("Failed to listen on %s: %w", trafficMonitorPipeAddress, err)
	}

	proxy := newTrafficMonitorReverseProxy(launch.controllerAddr)
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/traffic" {
				http.NotFound(w, r)
				return
			}
			proxy.ServeHTTP(w, r)
		}),
	}

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("TrafficMonitor compatibility pipe server exited unexpectedly: %v", err)
		}
	}()
	log.Printf("TrafficMonitor compatibility pipe listen address: %s", listener.Addr().String())

	return func() {
		if err := server.Close(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("Failed to close TrafficMonitor compatibility pipe: %v", err)
		}
	}, nil
}

func newTrafficMonitorReverseProxy(controllerAddr string) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		FlushInterval: -1,
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = "mihomo.local"
			req.URL.Path = "/traffic"
			req.URL.RawPath = ""
			req.URL.RawQuery = ""
			req.Host = "mihomo.local"
		},
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return namedpipe.DialContext(ctx, controllerAddr)
			},
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			http.Error(w, fmt.Sprintf("Failed to forward TrafficMonitor traffic request: %v", err), http.StatusBadGateway)
		},
	}
}
