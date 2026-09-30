//go:build linux

package sysproxyapi

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/user"
	"strconv"
	"strings"

	"github.com/amamiyakokoro/kokorobox-service/route/pipectx"

	"github.com/amamiyakokoro/sysproxy-go/v2/sysproxy"
)

type linuxSysproxyGuardRunner struct {
	env []string
	uid uint32
	gid uint32
}

func captureSysproxyGuardRunner(r *http.Request) (sysproxyGuardRunner, error) {
	peer, ok := pipectx.RequestUnixPeerInfo(r)
	if !ok {
		return nil, fmt.Errorf("system proxy request has no Linux user identity")
	}
	return captureLinuxSysproxyGuardRunner(peer)
}

func captureLinuxSysproxyGuardRunner(peer pipectx.UnixPeerInfo) (*linuxSysproxyGuardRunner, error) {
	peerEnv, err := readLinuxProcessEnv(peer.PID)
	if err != nil {
		return nil, fmt.Errorf("Failed to read connected process environment: %w", err)
	}
	// The proxy backend can resolve an empty launcher environment by UID.
	// Capture that resolved session too, so both the live lease and its recovery
	// record remain usable after the connected process exits.
	recoverable := false
	for _, item := range peerEnv {
		key, value, ok := strings.Cut(item, "=")
		if ok && value != "" && recoveryLinuxEnvKeys[key] {
			recoverable = true
			break
		}
	}
	if !recoverable {
		account, err := user.LookupId(strconv.FormatUint(uint64(peer.UID), 10))
		if err != nil {
			return nil, fmt.Errorf("resolve connected Linux user: %w", err)
		}
		opts, err := sysproxy.OptionsForUser(account.Username)
		if err != nil {
			return nil, fmt.Errorf("resolve connected Linux user session: %w", err)
		}
		if opts.PeerUID != peer.UID {
			return nil, fmt.Errorf("resolved Linux session does not match its socket identity")
		}
		peerEnv = opts.Environment
	}

	return &linuxSysproxyGuardRunner{
		env: peerEnv,
		uid: peer.UID,
		gid: peer.GID,
	}, nil
}

func (r *linuxSysproxyGuardRunner) Query(opts *sysproxy.Options) (*sysproxy.ProxyConfig, error) {
	return querySysproxyGuardSettings(r.sessionOptions(opts))
}

func (r *linuxSysproxyGuardRunner) Apply(mode sysproxyGuardMode, opts *sysproxy.Options) error {
	return applySysproxyGuardSettings(mode, r.sessionOptions(opts))
}

func (r *linuxSysproxyGuardRunner) Disable(opts *sysproxy.Options) error {
	return sysproxy.DisableProxy(r.sessionOptions(opts))
}

func (r *linuxSysproxyGuardRunner) WaitChange(ctx context.Context, opts *sysproxy.Options) error {
	return sysproxy.WaitProxySettingsChange(ctx, r.sessionOptions(opts))
}

func (r *linuxSysproxyGuardRunner) WaitChangeReady(ctx context.Context, opts *sysproxy.Options, ready func()) error {
	if ready != nil {
		ready()
	}
	return sysproxy.WaitProxySettingsChange(ctx, r.sessionOptions(opts))
}

func (r *linuxSysproxyGuardRunner) Close() error {
	return nil
}

func (r *linuxSysproxyGuardRunner) sessionOptions(opts *sysproxy.Options) *sysproxy.Options {
	sessionOpts := cloneSysproxyOptions(opts)
	sessionOpts.Environment = append([]string(nil), r.env...)
	sessionOpts.PeerUID = r.uid
	sessionOpts.PeerGID = r.gid
	return sessionOpts
}

func readLinuxProcessEnv(pid int) ([]string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/environ", pid))
	if err != nil {
		return nil, err
	}

	env := []string{}
	for item := range strings.SplitSeq(string(data), "\x00") {
		if item != "" && strings.Contains(item, "=") {
			env = append(env, item)
		}
	}
	return env, nil
}
