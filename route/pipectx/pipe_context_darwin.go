//go:build darwin

package pipectx

import (
	"context"
	"net"
	"net/http"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

type darwinPeerContextKey struct{}

type AuditToken [8]uint32

type DarwinPeerInfo struct {
	PID        int
	UID        uint32
	GID        uint32
	HasGID     bool
	AuditToken AuditToken
}

type syscallConn interface {
	SyscallConn() (syscall.RawConn, error)
}

func ConfigureServer(server *http.Server) {
	server.ConnContext = func(ctx context.Context, conn net.Conn) context.Context {
		info, ok := getDarwinPeerInfo(conn)
		if !ok {
			return ctx
		}
		return context.WithValue(ctx, darwinPeerContextKey{}, info)
	}
}

func getDarwinPeerInfo(conn net.Conn) (DarwinPeerInfo, bool) {
	sysConn, ok := conn.(syscallConn)
	if !ok {
		return DarwinPeerInfo{}, false
	}

	rawConn, err := sysConn.SyscallConn()
	if err != nil {
		return DarwinPeerInfo{}, false
	}

	var (
		info DarwinPeerInfo
		okay bool
	)
	if err := rawConn.Control(func(fd uintptr) {
		cred, sockErr := unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
		if sockErr != nil || cred == nil {
			return
		}
		pid, pidErr := unix.GetsockoptInt(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERPID)
		if pidErr != nil || pid <= 0 {
			return
		}
		var token AuditToken
		tokenSize := uint32(unsafe.Sizeof(token))
		_, _, tokenErr := unix.Syscall6(
			unix.SYS_GETSOCKOPT,
			fd,
			uintptr(unix.SOL_LOCAL),
			uintptr(unix.LOCAL_PEERTOKEN),
			uintptr(unsafe.Pointer(&token[0])),
			uintptr(unsafe.Pointer(&tokenSize)),
			0,
		)
		if tokenErr != 0 || tokenSize != uint32(unsafe.Sizeof(token)) {
			return
		}
		info = DarwinPeerInfo{PID: pid, UID: cred.Uid, AuditToken: token}
		if cred.Ngroups > 0 {
			info.GID = cred.Groups[0]
			info.HasGID = true
		}
		okay = true
	}); err != nil {
		return DarwinPeerInfo{}, false
	}

	return info, okay && info.PID > 0
}

func RequestDarwinPeerInfo(r *http.Request) (DarwinPeerInfo, bool) {
	info, ok := r.Context().Value(darwinPeerContextKey{}).(DarwinPeerInfo)
	return info, ok && info.PID > 0
}
