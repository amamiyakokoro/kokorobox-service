package processrouterapi

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/amamiyakokoro/kokorobox-service/processrouter"
	"github.com/amamiyakokoro/kokorobox-service/route/httphelper"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type testStatusSource struct{ renewals atomic.Int32 }

func (s *testStatusSource) RenewLease() { s.renewals.Add(1) }
func (s *testStatusSource) Status() processrouter.Status {
	return processrouter.Status{Version: 1, State: processrouter.StateStopped}
}
func readServerFrame(r io.Reader) (byte, []byte, error) {
	var h [2]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return 0, nil, err
	}
	size := int(h[1] & 127)
	if size == 126 {
		var n [2]byte
		if _, err := io.ReadFull(r, n[:]); err != nil {
			return 0, nil, err
		}
		size = int(binary.BigEndian.Uint16(n[:]))
	}
	if size > 4096 || size == 127 {
		return 0, nil, fmt.Errorf("invalid frame length")
	}
	data := make([]byte, size)
	_, err := io.ReadFull(r, data)
	return h[0] & 15, data, err
}
func TestStatusEventsSendSnapshotAndRequireClientPing(t *testing.T) {
	source := &testStatusSource{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { serveStatusEvents(w, r, source) }))
	defer server.Close()
	conn, err := net.Dial("tcp", strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	_, err = fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: localhost\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n")
	if err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, nil)
	if err != nil || response.StatusCode != 101 {
		t.Fatalf("upgrade failed: %v", err)
	}
	opcode, data, err := readServerFrame(reader)
	if err != nil || opcode != httphelper.WebSocketOpText {
		t.Fatalf("no initial status: %v", err)
	}
	var status processrouter.Status
	if json.Unmarshal(data, &status) != nil || status.State != processrouter.StateStopped {
		t.Fatal("invalid initial snapshot")
	}
	if source.renewals.Load() != 1 {
		t.Fatal("initial lease was not renewed")
	}
	// A masked empty ping is the only continuing proof of client liveness.
	if _, err = conn.Write([]byte{0x89, 0x80, 0, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	opcode, _, err = readServerFrame(reader)
	if err != nil || opcode != httphelper.WebSocketOpPong {
		t.Fatalf("no pong: %v", err)
	}
	if source.renewals.Load() != 2 {
		t.Fatal("client ping did not renew the lease")
	}
	time.Sleep(3100 * time.Millisecond)
	if source.renewals.Load() != 2 {
		t.Fatal("server status polling renewed the client lease")
	}
}
