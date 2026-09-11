//go:build darwin

package bootstrap

import (
	"fmt"
	"net/http"
	"os"

	"github.com/amamiyakokoro/kokorobox-service/route/pipectx"
	"github.com/go-chi/chi/v5"
)

func Register(r chi.Router, socketPath string) {
	if socketPath == "" {
		return
	}
	r.Post("/bootstrap", newHandler(verifyDarwinPeer, func(uid uint32) error {
		if err := os.Chown(socketPath, int(uid), -1); err != nil {
			return fmt.Errorf("restrict bootstrap socket owner: %w", err)
		}
		if err := os.Chmod(socketPath, 0o600); err != nil {
			return fmt.Errorf("restrict bootstrap socket permissions: %w", err)
		}
		return nil
	}))
}

func verifyDarwinPeer(r *http.Request) (uint32, error) {
	peer, ok := pipectx.RequestDarwinPeerInfo(r)
	if !ok {
		return 0, fmt.Errorf("bootstrap request has no kernel peer identity")
	}
	if err := verifyKokoroBoxProcess(peer.AuditToken); err != nil {
		return 0, err
	}
	return peer.UID, nil
}
