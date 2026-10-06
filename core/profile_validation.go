package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/amamiyakokoro/kokorobox-service/core/security"
)

const validationOutputLimit = 256 * 1024

type ProfileValidationRequest struct {
	Executable string   `json:"executable"`
	ConfigPath string   `json:"configPath"`
	WorkDir    string   `json:"workDir"`
	SafePaths  []string `json:"safePaths"`
}
type ProfileValidationResult struct {
	Outcome string `json:"outcome"`
	Output  string `json:"output"`
}

func (cm *CoreManager) ValidateProfile(ctx context.Context, request ProfileValidationRequest) (ProfileValidationResult, error) {
	for _, path := range append([]string{request.Executable, request.ConfigPath, request.WorkDir}, request.SafePaths...) {
		if !filepath.IsAbs(path) || strings.ContainsRune(path, 0) {
			return ProfileValidationResult{}, fmt.Errorf("validation paths must be absolute")
		}
	}
	if len(request.SafePaths) > 64 {
		return ProfileValidationResult{}, fmt.Errorf("too many trusted paths")
	}
	executable, err := filepath.EvalSymlinks(request.Executable)
	if err != nil {
		return ProfileValidationResult{}, err
	}
	switch strings.ToLower(filepath.Base(executable)) {
	case "mihomo", "mihomo-alpha", "mihomo.exe", "mihomo-alpha.exe":
	default:
		return ProfileValidationResult{}, fmt.Errorf("validation executable must be Mihomo")
	}
	config, err := os.Stat(request.ConfigPath)
	if err != nil || !config.Mode().IsRegular() {
		return ProfileValidationResult{}, fmt.Errorf("invalid validation config")
	}
	work, err := os.Stat(request.WorkDir)
	if err != nil || !work.IsDir() {
		return ProfileValidationResult{}, fmt.Errorf("invalid validation working directory")
	}
	profile := LaunchProfile{CorePath: executable, Mode: CoreRunModeAuto, SafePaths: request.SafePaths}
	// Keep the same protected executable and runtime mode when validating the
	// currently managed core. Do not mutate persisted launch intent or restart it.
	var staged string
	cm.mutex.Lock()
	if cm.launch != nil {
		source, _ := filepath.EvalSymlinks(cm.launch.sourcePath)
		if source == executable {
			staged = cm.launch.executablePath
			profile.Mode = cm.launch.profile.Mode
		}
	}
	cm.mutex.Unlock()
	if staged == "" {
		staged, err = security.PrepareBinary(executable)
		if err != nil {
			return ProfileValidationResult{}, err
		}
	}
	launch := &launchSession{sourcePath: executable, executablePath: staged, workingDir: request.WorkDir,
		args: []string{"-t", "-f", request.ConfigPath, "-d", request.WorkDir}, env: buildLaunchEnv(profile, nil), profile: profile}
	command, err := newCoreLauncher(launch).Command(launch)
	if err != nil {
		return ProfileValidationResult{}, err
	}
	return runProfileValidation(ctx, command)
}

type validationOutput struct {
	mu       sync.Mutex
	bytes    []byte
	overflow bool
	cancel   context.CancelFunc
}

func (w *validationOutput) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	room := validationOutputLimit - len(w.bytes)
	if len(data) > room {
		w.bytes = append(w.bytes, data[:room]...)
		w.overflow = true
		w.cancel()
	} else {
		w.bytes = append(w.bytes, data...)
	}
	return len(data), nil
}
func runProfileValidation(parent context.Context, command *coreCommand) (ProfileValidationResult, error) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	defer command.cleanupNow()
	output := &validationOutput{cancel: cancel}
	command.cmd.Stdout = output
	command.cmd.Stderr = output
	command.cmd.WaitDelay = time.Second
	if err := ctx.Err(); err != nil {
		return ProfileValidationResult{}, err
	}
	cmd, err := command.start()
	if err != nil {
		return ProfileValidationResult{}, err
	}
	cmd.WaitDelay = time.Second
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err = <-done:
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		err = <-done
	}
	output.mu.Lock()
	defer output.mu.Unlock()
	result := ProfileValidationResult{Outcome: "valid", Output: string(output.bytes)}
	switch {
	case output.overflow:
		result.Outcome = "output-limit"
	case ctx.Err() != nil:
		result.Outcome = "timeout"
	case err != nil:
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			return ProfileValidationResult{}, err
		}
		result.Outcome = "invalid"
	}
	return result, nil
}
