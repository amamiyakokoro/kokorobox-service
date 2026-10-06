package processrouterapi

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/amamiyakokoro/kokorobox-service/processrouter"
	"github.com/amamiyakokoro/kokorobox-service/route/httphelper"
)

// The Manager owns reconciliation. Clients receive snapshots and renew the
// existing 20-second lease with explicit pings, never with server-side traffic.
type routerStatusSource interface {
	Status() processrouter.Status
	RenewLease()
}

func statusEvents(w http.ResponseWriter, r *http.Request) { serveStatusEvents(w, r, manager) }
func serveStatusEvents(w http.ResponseWriter, r *http.Request, source routerStatusSource) {
	conn, _, err := httphelper.AcceptWebSocket(w, r)
	if err != nil {
		httphelper.SendError(w, err)
		return
	}
	defer conn.Close()
	source.RenewLease()
	var writes sync.Mutex
	write := func(op byte, data []byte) error {
		writes.Lock()
		defer writes.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		return httphelper.WriteWebSocketFrame(conn, op, data)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
			opcode, data, err := httphelper.ReadWebSocketFrame(conn)
			if err != nil {
				return
			}
			switch opcode {
			case httphelper.WebSocketOpPing:
				source.RenewLease()
				if write(httphelper.WebSocketOpPong, data) != nil {
					return
				}
			case httphelper.WebSocketOpClose:
				return
			}
		}
	}()
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	var previous string
	for {
		data, err := json.Marshal(source.Status())
		if err != nil {
			return
		}
		if string(data) != previous {
			if write(httphelper.WebSocketOpText, data) != nil {
				return
			}
			previous = string(data)
		}
		select {
		case <-done:
			return
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}
