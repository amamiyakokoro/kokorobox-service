package processrouter

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/amamiyakokoro/kokorobox-service/log"
)

func probeMihomo(port int, requireSOCKS bool) error {
	endpoint := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	connection, err := net.DialTimeout("tcp", endpoint, probeTimeout)
	if err != nil {
		return fmt.Errorf("connect to proxy: %w", err)
	}
	defer connection.Close()
	if !requireSOCKS {
		return nil
	}
	if err := connection.SetDeadline(time.Now().Add(probeTimeout)); err != nil {
		return fmt.Errorf("set SOCKS5 handshake deadline: %w", err)
	}
	greeting := []byte{0x05, 0x01, 0x00}
	written, err := connection.Write(greeting)
	if err != nil {
		return fmt.Errorf("write SOCKS5 greeting: %w", err)
	}
	if written != len(greeting) {
		return fmt.Errorf("write SOCKS5 greeting: %w", io.ErrShortWrite)
	}
	response := make([]byte, 2)
	if _, err := io.ReadFull(connection, response); err != nil {
		return fmt.Errorf("read SOCKS5 greeting response: %w", err)
	}
	if response[0] != 0x05 || response[1] != 0x00 {
		return fmt.Errorf("SOCKS5 greeting rejected: received %02x %02x; expected 05 00 (no authentication)", response[0], response[1])
	}
	return nil
}

// Report transitions once rather than flooding both log views every three seconds.
// Keep the latest failure in status even when subsequent probes remain blocked.
func (m *Manager) updateProxyHealthLocked(requiresProxy bool, probeErr error) {
	wasBlocked := m.state == StateBlocked
	m.mihomoReady = requiresProxy && probeErr == nil
	m.lastError = ""
	if requiresProxy && probeErr != nil {
		m.state = StateBlocked
		m.lastError = fmt.Sprintf("Proxy health check failed at 127.0.0.1:%d: %v", m.rules.ProxyPort, probeErr)
		if !wasBlocked {
			message := m.lastError + "; proxy rules are blocked to prevent direct fallback"
			recordDiagnosticAt("warning", message)
			log.Warnf("%s", message)
		}
		return
	}
	m.state = StateRunning
	if wasBlocked && requiresProxy {
		message := fmt.Sprintf("Proxy health check recovered at 127.0.0.1:%d; proxy rules restored", m.rules.ProxyPort)
		recordDiagnosticAt("info", message)
		log.Printf("%s", message)
	}
}
