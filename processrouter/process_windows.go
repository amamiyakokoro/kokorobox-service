//go:build windows

package processrouter

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
	"unsafe"

	"github.com/amamiyakokoro/kokorobox-service/core/security"
	"github.com/amamiyakokoro/kokorobox-service/log"
	"golang.org/x/sys/windows"
)

type windowsNativeProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	events chan routerEvent
	done   chan struct{}
	job    windows.Handle
}

func startNativeProcess(binaryPath, workingDir string) (nativeProcess, error) {
	cmd := exec.Command(binaryPath)
	cmd.Dir = workingDir
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_BREAKAWAY_FROM_JOB | windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW,
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	job, err := attachProcessJob(uint32(cmd.Process.Pid))
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, err
	}
	process := &windowsNativeProcess{
		cmd:    cmd,
		stdin:  stdin,
		events: make(chan routerEvent, 16),
		done:   make(chan struct{}),
		job:    job,
	}
	go process.readEvents(stdout)
	go process.readErrors(stderr)
	go func() {
		_ = cmd.Wait()
		close(process.done)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			_ = process.Stop(context.Background())
			return nil, errors.New("native process router did not become ready")
		case event, ok := <-process.events:
			if !ok {
				return nil, errors.New("native process router exited during startup")
			}
			if event.Version != ProtocolVersion {
				_ = process.Stop(context.Background())
				return nil, fmt.Errorf("unsupported native router protocol version: %d", event.Version)
			}
			if event.Event == "ready" {
				return process, nil
			}
			if event.Event == "error" {
				_ = process.Stop(context.Background())
				return nil, fmt.Errorf("native process router startup failed: %s", event.Message)
			}
		}
	}
}

func (p *windowsNativeProcess) PID() int { return p.cmd.Process.Pid }

func (p *windowsNativeProcess) Alive() bool {
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}

func (p *windowsNativeProcess) Send(payload []byte) error {
	if !p.Alive() {
		return errors.New("native process router is not running")
	}
	_, err := p.stdin.Write(payload)
	return err
}

func (p *windowsNativeProcess) Events() <-chan routerEvent { return p.events }

func (p *windowsNativeProcess) Stop(ctx context.Context) error {
	if !p.Alive() {
		return p.closeJob()
	}
	_ = p.Send([]byte("{\"version\":1,\"command\":\"shutdown\"}\n"))
	select {
	case <-p.done:
	case <-time.After(500 * time.Millisecond):
		if err := p.closeJob(); err != nil {
			return err
		}
		select {
		case <-p.done:
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
			return errors.New("timed out while stopping native process router")
		}
	}
	return p.closeJob()
}

func (p *windowsNativeProcess) closeJob() error {
	if p.job == 0 {
		return nil
	}
	err := windows.CloseHandle(p.job)
	p.job = 0
	return err
}

func (p *windowsNativeProcess) readEvents(reader io.Reader) {
	defer close(p.events)
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 64*1024)
	for scanner.Scan() {
		var event routerEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			log.Printf("忽略无效的应用分流输出: %v", err)
			continue
		}
		p.events <- event
	}
}

func (p *windowsNativeProcess) readErrors(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 64*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) > 1000 {
			line = line[:1000]
		}
		log.Printf("应用分流原生组件: %s", line)
	}
}

func attachProcessJob(pid uint32) (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, fmt.Errorf("create process router job object: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(job)
		return 0, fmt.Errorf("configure process router job object: %w", err)
	}
	processHandle, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION,
		false,
		pid,
	)
	if err != nil {
		_ = windows.CloseHandle(job)
		return 0, fmt.Errorf("open process router handle: %w", err)
	}
	defer windows.CloseHandle(processHandle)
	if err := windows.AssignProcessToJobObject(job, processHandle); err != nil {
		_ = windows.CloseHandle(job)
		return 0, fmt.Errorf("attach process router to job object: %w", err)
	}
	return job, nil
}

func hardenProcessRouterPaths(binaryPath, configDir string) error {
	if err := security.RestrictPath(configDir, windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT); err != nil {
		return err
	}
	if _, err := os.Stat(binaryPath); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return security.SecureBinary(binaryPath)
}
