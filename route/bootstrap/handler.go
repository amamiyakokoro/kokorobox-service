package bootstrap

import (
	"errors"
	"net/http"

	"github.com/amamiyakokoro/kokorobox-service/log"
	"github.com/amamiyakokoro/kokorobox-service/route/auth"
	"github.com/amamiyakokoro/kokorobox-service/route/httphelper"
)

const maxBootstrapBodyBytes = 16 * 1024

type peerVerifier func(*http.Request) (uint32, error)
type socketRestrictor func(uint32) error

type request struct {
	PublicKey string `json:"public_key"`
}

func newHandler(verify peerVerifier, restrict socketRestrictor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, err := verify(r)
		if err != nil {
			log.Printf("Rejected macOS service bootstrap client: %v", err)
			httphelper.SendError(w, httphelper.Forbidden("Bootstrap client verification failed"))
			return
		}

		var payload request
		r.Body = http.MaxBytesReader(w, r.Body, maxBootstrapBodyBytes)
		if err := httphelper.DecodeRequest(r, &payload); err != nil {
			httphelper.SendError(w, httphelper.BadRequest("Invalid bootstrap request"))
			return
		}

		km := auth.GetKeyManager()
		if err := km.BootstrapPublicKeyForUID(payload.PublicKey, uid); err != nil {
			if errors.Is(err, auth.ErrAlreadyInitialized) {
				httphelper.SendError(w, httphelper.Conflict("Service authentication is already initialized"))
				return
			}
			httphelper.SendError(w, httphelper.BadRequest(err.Error()))
			return
		}
		if err := restrict(uid); err != nil {
			log.Printf("Failed to restrict initialized macOS service socket: %v", err)
			httphelper.SendError(w, err)
			return
		}

		httphelper.SendJSON(w, "success", "Service authentication initialized")
	}
}
