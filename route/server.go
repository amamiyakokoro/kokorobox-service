package route

import (
	"context"
	"errors"
	"fmt"
	"github.com/amamiyakokoro/kokorobox-service/identity"
	"github.com/amamiyakokoro/kokorobox-service/log"
	"github.com/amamiyakokoro/kokorobox-service/route/auth"
	"github.com/amamiyakokoro/kokorobox-service/route/coreapi"
	"github.com/amamiyakokoro/kokorobox-service/route/dnsapi"
	"github.com/amamiyakokoro/kokorobox-service/route/pipectx"
	"github.com/amamiyakokoro/kokorobox-service/route/processrouterapi"
	"github.com/amamiyakokoro/kokorobox-service/route/sysproxyapi"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/amamiyakokoro/kokorobox-service/listen"
)

const serverShutdownTimeout = 3 * time.Second

var (
	unixServer *http.Server
	pipeServer *http.Server
	serverMu   sync.Mutex
)

func GetConfigDir() string {
	if dir := identity.ConfigDirectoryOverride(); dir != "" {
		return dir
	}

	switch runtime.GOOS {
	case "windows":
		return `C:\ProgramData`
	case "darwin":
		return filepath.Join("/var/root", "Library", "Application Support")
	default:
		return filepath.Join("/root", ".config")
	}
}

func GetServiceDataDir() (string, error) {
	return identity.DataDirectory(GetConfigDir())
}

func Start(addr string) error {
	dataDir, err := GetServiceDataDir()
	if err != nil {
		return fmt.Errorf("prepare service data directory: %w", err)
	}
	keyDir := filepath.Join(dataDir, "keys")

	if err := auth.InitKeyManager(keyDir); err != nil {
		log.Printf("Warning: failed to initialize key manager: %v", err)
	}

	km := auth.GetKeyManager()
	if km.IsInitialized() {
		log.Println("Key manager is initialized")
	} else {
		log.Println("Warning: key manager is not initialized")
	}
	if km.HasAuthorizedPrincipal() {
		log.Println("Requestor identity binding is enabled")
	} else {
		log.Println("Warning: requestor identity binding is not enabled")
	}
	if err := sysproxyapi.ConfigureManagedProxyRecovery(dataDir); err != nil {
		log.Printf("Failed to recover service-owned system proxy: %v", err)
	}
	if err := dnsapi.Configure(dataDir); err != nil {
		log.Printf("Failed to recover service-owned DNS: %v", err)
	}
	if err := coreapi.ConfigureDesiredCore(dataDir); err != nil {
		return fmt.Errorf("recover desired core state: %w", err)
	}

	if runtime.GOOS == "windows" {
		err = startServer(addr, StartPipe)
	} else {
		err = startServer(addr, StartUnix)
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func Stop() error {
	var errs []error
	if err := sysproxyapi.StopManagedProxy(); err != nil {
		errs = append(errs, fmt.Errorf("Failed to clean up system proxy settings: %w", err))
	}
	if err := dnsapi.Stop(); err != nil {
		errs = append(errs, fmt.Errorf("Failed to clean up DNS settings: %w", err))
	}
	if err := processrouterapi.Stop(); err != nil {
		errs = append(errs, fmt.Errorf("Failed to stop process router: %w", err))
	}
	if err := coreapi.Stop(); err != nil {
		errs = append(errs, fmt.Errorf("Failed to stop core: %w", err))
	}
	if err := closeServers(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func startServer(addr string, startFunc func(string) error) error {
	if err := closeServers(); err != nil {
		return err
	}

	if len(addr) > 0 {
		if runtime.GOOS != "windows" {
			dir := filepath.Dir(addr)
			if err := ensureDirExists(dir); err != nil {
				return err
			}

			if err := syscall.Unlink(addr); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("Unlink error: %w", err)
			}
		}

		if err := startFunc(addr); err != nil {
			return err
		}
	}
	return nil
}

func closeServers() error {
	serverMu.Lock()
	servers := []*http.Server{unixServer, pipeServer}
	unixServer = nil
	pipeServer = nil
	serverMu.Unlock()

	var errs []error
	for _, server := range servers {
		if server == nil {
			continue
		}
		if err := shutdownServer(server); err != nil {
			errs = append(errs, fmt.Errorf("Failed to close service listener: %w", err))
		}
	}
	return errors.Join(errs...)
}

func shutdownServer(server *http.Server) error {
	ctx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		if closeErr := server.Close(); closeErr != nil && !errors.Is(closeErr, http.ErrServerClosed) {
			return errors.Join(err, closeErr)
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			return err
		}
	}
	return nil
}

func ensureDirExists(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("Failed to create directory: %w", err)
		}
	}
	return nil
}

func StartHTTP(addr string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("HTTP listen error: %w", err)
	}
	log.Printf("http Listen address: %s", addr)
	server := &http.Server{
		Handler: router(""),
	}
	return server.Serve(l)
}

func StartUnix(addr string) error {
	l, err := net.Listen("unix", addr)
	if err != nil {
		return fmt.Errorf("Unix socket listen error: %w", err)
	}
	if uid, ok := auth.GetKeyManager().GetAuthorizedUID(); ok {
		if err := os.Chown(addr, int(uid), -1); err != nil {
			_ = l.Close()
			return fmt.Errorf("Failed to set Unix socket owner: %w", err)
		}
		if err := os.Chmod(addr, 0o600); err != nil {
			_ = l.Close()
			return fmt.Errorf("Failed to set Unix socket permissions: %w", err)
		}
	} else if runtime.GOOS == "darwin" {
		// Before authentication exists, the signed Desktop process reaches the
		// one-time bootstrap route through this socket. The route verifies the
		// kernel peer PID and Developer ID signature before accepting a key, then
		// immediately changes the socket to owner-only mode.
		if err := os.Chmod(addr, 0o666); err != nil {
			_ = l.Close()
			return fmt.Errorf("Failed to set bootstrap Unix socket permissions: %w", err)
		}
	} else if err := os.Chmod(addr, 0o600); err != nil {
		_ = l.Close()
		return fmt.Errorf("Failed to set Unix socket permissions: %w", err)
	}
	log.Printf("unix Listen address: %s", l.Addr().String())

	server := &http.Server{
		Handler: router(addr),
	}
	pipectx.ConfigureServer(server)
	serverMu.Lock()
	unixServer = server
	serverMu.Unlock()
	return server.Serve(l)
}

func StartPipe(addr string) error {
	pipeSDDL := ""
	if sid, ok := auth.GetKeyManager().GetAuthorizedSID(); ok {
		pipeSDDL = fmt.Sprintf("D:PAI(A;OICI;GWGR;;;%s)(A;OICI;GWGR;;;SY)", sid)
	}

	l, err := listen.ListenNamedPipe(addr, pipeSDDL)
	if err != nil {
		return fmt.Errorf("Pipe listen error: %w", err)
	}
	log.Printf("pipe Listen address: %s", l.Addr().String())

	server := &http.Server{
		Handler: router(""),
	}
	pipectx.ConfigureServer(server)
	serverMu.Lock()
	pipeServer = server
	serverMu.Unlock()
	return server.Serve(l)
}
