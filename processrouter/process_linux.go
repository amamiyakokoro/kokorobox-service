//go:build linux

package processrouter

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/amamiyakokoro/kokorobox-service/log"
)

const (
	linuxCgroupName       = "kokorobox-app-routing"
	linuxCgroupV2Root     = "/sys/fs/cgroup"
	linuxPrivateCgroupDir = "/run/kokorobox/cgroup/net_cls"
	linuxProcessScanDelay = 250 * time.Millisecond
)

type linuxCgroupController struct {
	mode        linuxCgroupMode
	mountPoint  string
	managedRoot string
	mountedV1   bool
}

type linuxNativeProcess struct {
	mu              sync.Mutex
	controller      *linuxCgroupController
	events          chan routerEvent
	done            chan struct{}
	cancel          context.CancelFunc
	alive           bool
	firewallReady   bool
	proxyUDPDNS     bool
	proxyPort       int
	targets         []linuxProcessTarget
	assigned        map[int]string
	originalCgroups map[int]string
}

type linuxProcessSnapshot struct {
	pid            int
	executablePath string
	executableName string
}

type linuxProcessTarget struct {
	matchKind string
	pattern   string
	group     string
}

func hardenProcessRouterPaths(_ string, configDir string) error {
	info, err := os.Lstat(configDir)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("Linux process-router config path must be a real directory: %s", configDir)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 {
		return fmt.Errorf("Linux process-router config directory must be owned by root: %s", configDir)
	}
	if err := os.Chmod(configDir, 0o700); err != nil {
		return fmt.Errorf("harden Linux process-router config directory: %w", err)
	}
	return nil
}

func startNativeProcess(string, string) (nativeProcess, error) {
	if os.Geteuid() != 0 {
		return nil, errors.New("Linux application routing requires the root KokoroBox Service")
	}
	for _, command := range []string{"ip", "iptables", "ip6tables"} {
		if _, err := secureLinuxCommand(command); err != nil {
			return nil, err
		}
	}

	controller, err := createLinuxCgroupController()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	process := &linuxNativeProcess{
		controller:      controller,
		events:          make(chan routerEvent, 16),
		done:            make(chan struct{}),
		cancel:          cancel,
		alive:           true,
		assigned:        make(map[int]string),
		originalCgroups: make(map[int]string),
	}
	go process.monitorProcesses(ctx)
	return process, nil
}

func (p *linuxNativeProcess) PID() int { return os.Getpid() }

func (p *linuxNativeProcess) Alive() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.alive
}

func (p *linuxNativeProcess) FirewallReady() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.firewallReady
}

func (p *linuxNativeProcess) Backend() string {
	return "linux-" + string(p.controller.mode)
}

func (p *linuxNativeProcess) Events() <-chan routerEvent { return p.events }

func (p *linuxNativeProcess) Send(payload []byte) error {
	var command routerCommand
	if err := json.Unmarshal(payload, &command); err != nil {
		return fmt.Errorf("decode Linux process router command: %w", err)
	}
	if command.Version != ProtocolVersion || command.Command != "replace_rules" || !command.FailClosed {
		return errors.New("invalid Linux process router command")
	}
	if command.Proxy.Host != "127.0.0.1" || command.Proxy.Port != LinuxProxyPort {
		return fmt.Errorf("Linux TProxy listener must be 127.0.0.1:%d", LinuxProxyPort)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.alive {
		return errors.New("Linux process router is not running")
	}
	if !p.firewallReady || p.proxyUDPDNS != command.ProxyUDPDNS || p.proxyPort != command.Proxy.Port {
		if err := p.rebuildFirewallLocked(command.Proxy.Port, command.ProxyUDPDNS); err != nil {
			p.emitLocked("error", err.Error())
			return err
		}
	}

	targets := make([]linuxProcessTarget, 0, len(command.Rules))
	for _, rule := range command.Rules {
		if !rule.Enabled {
			continue
		}
		action := strings.ToLower(rule.Action)
		protocol := strings.ToLower(rule.Protocol)
		group := linuxRoutingGroup(action, protocol)
		if err := validateLinuxRoutingGroup(group); err != nil {
			return err
		}
		targets = append(targets, linuxProcessTarget{
			matchKind: rule.MatchKind,
			pattern:   rule.ProcessPattern,
			group:     group,
		})
	}
	p.targets = targets
	if err := p.scanProcessesLocked(); err != nil {
		if command.DiagnosticLogging {
			log.Printf("Linux 应用分流首次进程扫描失败: %v", err)
		}
	}
	p.emitLocked("rules_replaced", "")
	return nil
}

func (p *linuxNativeProcess) Stop(ctx context.Context) error {
	p.mu.Lock()
	if !p.alive {
		p.mu.Unlock()
		return nil
	}
	p.alive = false
	p.cancel()
	p.mu.Unlock()

	var waitErr error
	select {
	case <-p.done:
	case <-ctx.Done():
		waitErr = ctx.Err()
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	restoreErr := p.restoreAllLocked()
	firewallErr := runLinuxPlan(buildLinuxFirewallCleanup(), true)
	p.firewallReady = false
	cgroupErr := p.controller.cleanup()
	close(p.events)
	return errors.Join(waitErr, restoreErr, firewallErr, cgroupErr)
}

func (p *linuxNativeProcess) emitLocked(event, message string) {
	select {
	case p.events <- routerEvent{Version: ProtocolVersion, Event: event, Message: message}:
	default:
	}
}

func (p *linuxNativeProcess) rebuildFirewallLocked(proxyPort int, proxyUDPDNS bool) error {
	cleanup := buildLinuxFirewallCleanup()
	plan := buildLinuxFirewallSetup(p.controller.mode, proxyPort, proxyUDPDNS)
	_ = runLinuxPlan(cleanup, true)
	if err := runLinuxPlan(plan[len(cleanup):], false); err != nil {
		_ = runLinuxPlan(cleanup, true)
		p.firewallReady = false
		return fmt.Errorf("install Linux application-routing firewall: %w", err)
	}
	p.firewallReady = true
	p.proxyPort = proxyPort
	p.proxyUDPDNS = proxyUDPDNS
	return nil
}

func (p *linuxNativeProcess) monitorProcesses(ctx context.Context) {
	defer close(p.done)
	ticker := time.NewTicker(linuxProcessScanDelay)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snapshot, err := snapshotLinuxProcesses()
			if err != nil {
				log.Printf("Linux 应用分流进程扫描失败: %v", err)
				continue
			}
			p.mu.Lock()
			if p.alive {
				if err := p.applyProcessSnapshotLocked(snapshot); err != nil {
					log.Printf("Linux 应用分流进程扫描失败: %v", err)
				}
			}
			p.mu.Unlock()
		}
	}
}

func (p *linuxNativeProcess) scanProcessesLocked() error {
	snapshot, err := snapshotLinuxProcesses()
	if err != nil {
		return err
	}
	return p.applyProcessSnapshotLocked(snapshot)
}

func (p *linuxNativeProcess) applyProcessSnapshotLocked(snapshot []linuxProcessSnapshot) error {
	seen := make(map[int]bool)
	var scanErr error
	for _, process := range snapshot {
		pid := process.pid
		group, matches := p.targetGroup(process)
		if !matches {
			if _, assigned := p.assigned[pid]; assigned {
				scanErr = errors.Join(scanErr, p.restoreProcessLocked(pid))
			}
			continue
		}
		seen[pid] = true
		if p.assigned[pid] == group {
			continue
		}
		if _, tracked := p.originalCgroups[pid]; !tracked {
			original, err := p.controller.processCgroup(pid)
			if err != nil {
				scanErr = errors.Join(scanErr, err)
				continue
			}
			if strings.HasPrefix(original, p.controller.managedRoot+string(os.PathSeparator)) {
				original = p.inheritedOriginalCgroup(pid)
			}
			p.originalCgroups[pid] = original
		}
		if err := p.controller.move(pid, group); err != nil {
			scanErr = errors.Join(scanErr, err)
			continue
		}
		p.assigned[pid] = group
	}
	for pid := range p.assigned {
		if !seen[pid] && !processExists(pid) {
			delete(p.assigned, pid)
			delete(p.originalCgroups, pid)
		}
	}
	return scanErr
}

func (p *linuxNativeProcess) targetGroup(process linuxProcessSnapshot) (string, bool) {
	for _, target := range p.targets {
		if target.matchKind == matchProcessName {
			if target.pattern == process.executableName {
				return target.group, true
			}
			continue
		}
		if target.pattern == process.executablePath {
			return target.group, true
		}
	}
	return "", false
}

func snapshotLinuxProcesses() ([]linuxProcessSnapshot, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	snapshot := make([]linuxProcessSnapshot, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 1 || pid == os.Getpid() {
			continue
		}
		executablePath, err := os.Readlink(filepath.Join("/proc", entry.Name(), "exe"))
		if err != nil || strings.HasSuffix(executablePath, " (deleted)") {
			continue
		}
		snapshot = append(snapshot, linuxProcessSnapshot{
			pid:            pid,
			executablePath: executablePath,
			executableName: filepath.Base(executablePath),
		})
	}
	return snapshot, nil
}

func (p *linuxNativeProcess) inheritedOriginalCgroup(pid int) string {
	status, err := os.Open(filepath.Join("/proc", strconv.Itoa(pid), "status"))
	if err == nil {
		defer status.Close()
		scanner := bufio.NewScanner(status)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) == 2 && fields[0] == "PPid:" {
				parentPID, parseErr := strconv.Atoi(fields[1])
				if parseErr == nil && p.originalCgroups[parentPID] != "" {
					return p.originalCgroups[parentPID]
				}
				break
			}
		}
	}
	return filepath.Join(p.controller.mountPoint, "cgroup.procs")
}

func (p *linuxNativeProcess) restoreProcessLocked(pid int) error {
	target := p.originalCgroups[pid]
	delete(p.assigned, pid)
	delete(p.originalCgroups, pid)
	if target == "" || !processExists(pid) {
		return nil
	}
	if err := os.WriteFile(target, []byte(strconv.Itoa(pid)), 0o600); err != nil {
		return fmt.Errorf("restore process %d to %s: %w", pid, target, err)
	}
	return nil
}

func (p *linuxNativeProcess) restoreAllLocked() error {
	var restoreErr error
	for pid := range p.assigned {
		restoreErr = errors.Join(restoreErr, p.restoreProcessLocked(pid))
	}
	return restoreErr
}

func processExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func createLinuxCgroupController() (*linuxCgroupController, error) {
	var v2Err error
	if _, err := os.Stat(filepath.Join(linuxCgroupV2Root, "cgroup.controllers")); err == nil {
		if supportsLinuxCgroupMatcher("--path") {
			controller := &linuxCgroupController{
				mode:        linuxCgroupV2,
				mountPoint:  linuxCgroupV2Root,
				managedRoot: filepath.Join(linuxCgroupV2Root, linuxCgroupName),
			}
			if err := controller.setup(); err == nil {
				return controller, nil
			} else {
				v2Err = err
				_ = controller.cleanup()
			}
		} else {
			v2Err = errors.New("xt_cgroup does not support cgroup v2 path matching")
		}
	}

	controller, err := createLinuxCgroupV1Controller()
	if err != nil {
		return nil, fmt.Errorf("cgroup v2 unavailable (%v); cgroup v1 net_cls fallback failed: %w", v2Err, err)
	}
	return controller, nil
}

func createLinuxCgroupV1Controller() (*linuxCgroupController, error) {
	mountPoint, err := findNetClsMount()
	mounted := false
	if err != nil {
		return nil, err
	}
	if mountPoint == "" {
		if err := os.MkdirAll(linuxPrivateCgroupDir, 0o755); err != nil {
			return nil, err
		}
		if err := runLinuxCommand("mount", "-t", "cgroup", "-o", "net_cls", "net_cls", linuxPrivateCgroupDir); err != nil {
			return nil, fmt.Errorf("mount cgroup v1 net_cls: %w", err)
		}
		mountPoint = linuxPrivateCgroupDir
		mounted = true
	}
	if !supportsLinuxCgroupMatcher("--cgroup") {
		if mounted {
			_ = runLinuxCommand("umount", mountPoint)
		}
		return nil, errors.New("xt_cgroup does not support cgroup v1 classid matching")
	}
	controller := &linuxCgroupController{
		mode:        linuxCgroupV1,
		mountPoint:  mountPoint,
		managedRoot: filepath.Join(mountPoint, linuxCgroupName),
		mountedV1:   mounted,
	}
	if err := controller.setup(); err != nil {
		_ = controller.cleanup()
		return nil, err
	}
	return controller, nil
}

func (c *linuxCgroupController) setup() error {
	if err := os.MkdirAll(c.managedRoot, 0o755); err != nil {
		return fmt.Errorf("create Linux application-routing cgroup: %w", err)
	}
	for _, group := range linuxRoutingGroups {
		groupPath := filepath.Join(c.managedRoot, group)
		if err := os.MkdirAll(groupPath, 0o755); err != nil {
			return fmt.Errorf("create Linux application-routing cgroup %s: %w", group, err)
		}
		if c.mode == linuxCgroupV1 {
			if err := os.WriteFile(filepath.Join(groupPath, "net_cls.classid"), []byte(linuxNetClsClassIDs[group]), 0o600); err != nil {
				return fmt.Errorf("set Linux net_cls classid for %s: %w", group, err)
			}
		}
	}
	return nil
}

func (c *linuxCgroupController) move(pid int, group string) error {
	if err := validateLinuxRoutingGroup(group); err != nil {
		return err
	}
	target := filepath.Join(c.managedRoot, group, "cgroup.procs")
	if err := os.WriteFile(target, []byte(strconv.Itoa(pid)), 0o600); err != nil {
		return fmt.Errorf("move process %d into %s: %w", pid, group, err)
	}
	return nil
}

func (c *linuxCgroupController) processCgroup(pid int) (string, error) {
	file, err := os.Open(filepath.Join("/proc", strconv.Itoa(pid), "cgroup"))
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), ":", 3)
		if len(parts) != 3 {
			continue
		}
		if c.mode == linuxCgroupV2 && parts[0] == "0" && parts[1] == "" {
			return filepath.Join(c.mountPoint, strings.TrimPrefix(parts[2], "/"), "cgroup.procs"), nil
		}
		if c.mode == linuxCgroupV1 && containsController(parts[1], "net_cls") {
			return filepath.Join(c.mountPoint, strings.TrimPrefix(parts[2], "/"), "cgroup.procs"), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("process %d has no %s membership", pid, c.mode)
}

func (c *linuxCgroupController) cleanup() error {
	var cleanupErr error
	for index := len(linuxRoutingGroups) - 1; index >= 0; index-- {
		groupPath := filepath.Join(c.managedRoot, linuxRoutingGroups[index])
		cleanupErr = errors.Join(cleanupErr, restoreUntrackedProcesses(groupPath, c.mountPoint))
		if err := os.Remove(groupPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	if err := os.Remove(c.managedRoot); err != nil && !errors.Is(err, os.ErrNotExist) {
		cleanupErr = errors.Join(cleanupErr, err)
	}
	if c.mountedV1 {
		if err := runLinuxCommand("umount", c.mountPoint); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	return cleanupErr
}

func restoreUntrackedProcesses(groupPath, mountPoint string) error {
	payload, err := os.ReadFile(filepath.Join(groupPath, "cgroup.procs"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	target := filepath.Join(mountPoint, "cgroup.procs")
	var restoreErr error
	for _, line := range strings.Fields(string(payload)) {
		if err := os.WriteFile(target, []byte(line), 0o600); err != nil {
			restoreErr = errors.Join(restoreErr, err)
		}
	}
	return restoreErr
}

func findNetClsMount() (string, error) {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 || fields[2] != "cgroup" || !containsController(fields[3], "net_cls") {
			continue
		}
		return strings.ReplaceAll(fields[1], `\040`, " "), nil
	}
	return "", scanner.Err()
}

func containsController(list, target string) bool {
	for _, value := range strings.Split(list, ",") {
		if value == target {
			return true
		}
	}
	return false
}

func supportsLinuxCgroupMatcher(option string) bool {
	path, err := secureLinuxCommand("iptables")
	if err != nil {
		return false
	}
	command := exec.Command(path, "-m", "cgroup", "--help")
	command.Args[0] = "iptables"
	output, _ := command.CombinedOutput()
	return strings.Contains(string(output), option)
}

func runLinuxPlan(plan []linuxCommand, ignoreErrors bool) error {
	var planErr error
	for _, command := range plan {
		if err := runLinuxCommand(command.Name, command.Args...); err != nil {
			if ignoreErrors {
				continue
			}
			planErr = errors.Join(planErr, err)
			break
		}
	}
	return planErr
}

func runLinuxCommand(name string, args ...string) error {
	path, err := secureLinuxCommand(name)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, path, args...)
	// Preserve the requested tool name for alternatives and multicall binaries
	// such as openSUSE's /usr/bin/alts. The executable path remains the fully
	// resolved and validated path returned by secureLinuxCommand.
	command.Args[0] = name
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func secureLinuxCommand(name string) (string, error) {
	commandPath, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("required Linux command %s was not found", name)
	}
	commandPath, err = filepath.EvalSymlinks(commandPath)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(commandPath)
	if err != nil {
		return "", err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 || info.Mode().Perm()&0o022 != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("refusing unsafe Linux command path: %s", commandPath)
	}
	return commandPath, nil
}
