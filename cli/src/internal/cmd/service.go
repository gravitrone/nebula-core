package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

const (
	apiStateFilename = "api-runtime.json"
	apiPIDFilename   = "api.pid"
	apiLogFilename   = "api.log"
	apiLockFilename  = "api.lock"
)

var waitForAPIHealthProbe = func() (string, error) {
	client := newDefaultClient("", 900*time.Millisecond)
	return client.Health()
}

var startHealthTimeout = 8 * time.Second

type apiRuntimeState struct {
	PID       int       `json:"pid"`
	Port      int       `json:"port"`
	ServerDir string    `json:"server_dir"`
	LogPath   string    `json:"log_path"`
	StartedAt time.Time `json:"started_at"`
}

type apiLockState struct {
	OwnerPID  int       `json:"owner_pid"`
	APIPID    int       `json:"api_pid"`
	CreatedAt time.Time `json:"created_at"`
}

type apiLockHeldError struct {
	PID int
}

// Error handles error.
func (e *apiLockHeldError) Error() string {
	if e == nil || e.PID <= 0 {
		return "api lock is held"
	}
	return fmt.Sprintf("api lock is held by pid %d", e.PID)
}

// StartCmd starts the local Nebula API in background mode.
func StartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start local Nebula API in background mode",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runStartCmd(os.Stdout)
		},
	}
}

// StopCmd stops the local background API process started by `nebula start`.
func StopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop local Nebula API background process",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runStopCmd(os.Stdout)
		},
	}
}

// LogsCmd prints recent logs for local Nebula services.
func LogsCmd() *cobra.Command {
	var apiOnly bool
	var tail int
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Show local Nebula logs",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runLogsCmd(os.Stdout, apiOnly, tail)
		},
	}
	cmd.Flags().BoolVar(&apiOnly, "api", false, "show API logs")
	cmd.Flags().IntVar(&tail, "tail", 120, "number of recent log lines to show")
	return cmd
}

// runStartCmd runs run start cmd.
func runStartCmd(out io.Writer) error {
	if err := os.MkdirAll(runtimeDir(), 0o700); err != nil {
		return fmt.Errorf("create runtime dir: %w", err)
	}

	state, _ := loadAPIState()
	if state != nil && processAlive(state.PID) {
		renderCommandPanel(out, "Nebula API", []components.TableRow{
			{Label: "status", Value: "already running"},
			{Label: "pid", Value: strconv.Itoa(state.PID)},
			{Label: "port", Value: strconv.Itoa(state.Port)},
			{Label: "log", Value: state.LogPath},
		})
		return nil
	}

	if err := acquireAPILock(); err != nil {
		var held *apiLockHeldError
		if errors.As(err, &held) {
			renderCommandPanel(out, "Nebula API", []components.TableRow{
				{Label: "status", Value: "already running"},
				{Label: "pid", Value: strconv.Itoa(held.PID)},
				{Label: "port", Value: strconv.Itoa(api.DefaultAPIPort)},
				{Label: "log", Value: apiLogPath()},
			})
			return nil
		}
		return err
	}

	startSucceeded := false
	startedPID := 0
	defer func() {
		if !startSucceeded {
			stopProcessIfAlive(startedPID)
			_ = cleanupAPIState()
			_ = os.Remove(apiLockPath())
		}
	}()

	serverDir, err := resolveServerDir()
	if err != nil {
		return err
	}

	uvicornPath := filepath.Join(serverDir, ".venv", "bin", "uvicorn")
	if _, err := os.Stat(uvicornPath); err != nil {
		return fmt.Errorf("uvicorn not found at %s (run server env setup first)", uvicornPath)
	}

	logPath := apiLogPath()
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open api log: %w", err)
	}
	defer func() {
		_ = logFile.Close()
	}()

	cmd := exec.Command(
		uvicornPath,
		"nebula_api.app:app",
		"--host", "127.0.0.1",
		"--port", strconv.Itoa(api.DefaultAPIPort),
	)
	cmd.Dir = serverDir
	cmd.Env = append(os.Environ(), "PYTHONPATH=./src")
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start api: %w", err)
	}
	pid := cmd.Process.Pid
	startedPID = pid
	_ = cmd.Process.Release()

	state = &apiRuntimeState{
		PID:       pid,
		Port:      api.DefaultAPIPort,
		ServerDir: serverDir,
		LogPath:   logPath,
		StartedAt: time.Now(),
	}
	if err := saveAPIState(state); err != nil {
		return err
	}
	if err := updateAPILockPID(pid); err != nil {
		return err
	}
	startSucceeded = true

	ready := waitForAPIHealth(startHealthTimeout)
	if !ready {
		portConflict, processExited := detectStartupFailure(logPath, pid, 900*time.Millisecond)
		if portConflict {
			stopProcessIfAlive(pid)
			_ = cleanupAPIState()
			_ = os.Remove(apiLockPath())
			return fmt.Errorf(
				"multiple api instances detected: stop duplicate API processes and restart with `nebula start`",
			)
		}
		if processExited {
			_ = cleanupAPIState()
			_ = os.Remove(apiLockPath())
			return fmt.Errorf("api failed to start; check log at %s", logPath)
		}
	}

	status := "starting"
	if ready {
		status = "running"
	}
	renderCommandPanel(out, "Nebula API", []components.TableRow{
		{Label: "status", Value: status},
		{Label: "pid", Value: strconv.Itoa(pid)},
		{Label: "port", Value: strconv.Itoa(api.DefaultAPIPort)},
		{Label: "url", Value: api.DefaultBaseURL},
		{Label: "log", Value: logPath},
	})
	return nil
}

// runStopCmd runs run stop cmd.
func runStopCmd(out io.Writer) error {
	lock, lockErr := loadAPILock()
	if lockErr != nil && !os.IsNotExist(lockErr) {
		return fmt.Errorf("read api lock: %w", lockErr)
	}
	state, _ := loadAPIState()

	if lock == nil {
		if state == nil || state.PID <= 0 || !processAlive(state.PID) {
			_ = cleanupAPIState()
			renderCommandMessage(out, "Nebula API", "API is not running.")
			return nil
		}
		renderCommandMessage(
			out,
			"Nebula API",
			"refusing to stop unmanaged process (missing lock). clean stale state and run `nebula start`.",
		)
		return nil
	}

	pid := lock.APIPID
	if state != nil && state.PID > 0 {
		switch {
		case pid <= 0:
			pid = state.PID
		case pid == state.PID:
			// lock and runtime state agree
		case !processAlive(pid):
			// dead lock pid should not override live runtime state
			pid = state.PID
		case processAlive(state.PID):
			// conflicting live pids: prefer runtime state that we actively manage
			pid = state.PID
		}
	}
	if pid <= 0 {
		_ = cleanupAPIState()
		_ = os.Remove(apiLockPath())
		renderCommandMessage(out, "Nebula API", "stale lock cleaned.")
		return nil
	}

	if !processAlive(pid) {
		_ = cleanupAPIState()
		_ = os.Remove(apiLockPath())
		renderCommandMessage(
			out,
			"Nebula API",
			"API was not running. cleaned stale runtime files.",
		)
		return nil
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find api process: %w", err)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("stop api: %w", err)
	}

	deadline := time.Now().Add(6 * time.Second)
	for processAlive(pid) && time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
	}
	if processAlive(pid) {
		_ = proc.Signal(syscall.SIGKILL)
		time.Sleep(200 * time.Millisecond)
	}

	if err := cleanupAPIState(); err != nil {
		return err
	}
	_ = os.Remove(apiLockPath())

	renderCommandPanel(out, "Nebula API", []components.TableRow{
		{Label: "status", Value: "stopped"},
		{Label: "pid", Value: strconv.Itoa(pid)},
	})
	return nil
}

// runLogsCmd runs run logs cmd.
func runLogsCmd(out io.Writer, _ bool, tail int) error {
	if tail <= 0 {
		tail = 120
	}
	data, err := os.ReadFile(apiLogPath())
	if err != nil {
		if os.IsNotExist(err) {
			renderCommandMessage(out, "Nebula Logs", "No API logs yet. Run `nebula start` first.")
			return nil
		}
		return fmt.Errorf("read api log: %w", err)
	}

	lines := tailLines(strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n"), tail)
	width := commandWidth(out)
	contentWidth := components.BoxContentWidth(width)
	if contentWidth < 20 {
		contentWidth = 20
	}
	for i, line := range lines {
		lines[i] = components.ClampTextWidthEllipsis(components.SanitizeText(line), contentWidth)
	}
	renderCommandMessage(out, "Nebula API Logs", strings.Join(lines, "\n"))
	return nil
}

// runtimeDir runs runtime dir.
func runtimeDir() string {
	return filepath.Dir(config.Path())
}

// apiStatePath handles api state path.
func apiStatePath() string {
	return filepath.Join(runtimeDir(), apiStateFilename)
}

// apiPIDPath handles api pidpath.
func apiPIDPath() string {
	return filepath.Join(runtimeDir(), apiPIDFilename)
}

// apiLogPath handles api log path.
func apiLogPath() string {
	return filepath.Join(runtimeDir(), apiLogFilename)
}

// apiLockPath handles api lock path.
func apiLockPath() string {
	return filepath.Join(runtimeDir(), apiLockFilename)
}

// saveAPIState handles save apistate.
func saveAPIState(state *apiRuntimeState) error {
	if state == nil {
		return fmt.Errorf("api runtime state is nil")
	}
	if err := os.MkdirAll(runtimeDir(), 0o700); err != nil {
		return fmt.Errorf("create runtime dir: %w", err)
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal runtime state: %w", err)
	}
	if err := os.WriteFile(apiStatePath(), raw, 0o600); err != nil {
		return fmt.Errorf("write runtime state: %w", err)
	}
	return os.WriteFile(apiPIDPath(), []byte(strconv.Itoa(state.PID)), 0o600)
}

// loadAPIState loads load apistate.
func loadAPIState() (*apiRuntimeState, error) {
	raw, err := os.ReadFile(apiStatePath())
	if err != nil {
		return nil, err
	}
	var state apiRuntimeState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, fmt.Errorf("parse runtime state: %w", err)
	}
	return &state, nil
}

// loadAPILock loads api lock.
func loadAPILock() (*apiLockState, error) {
	raw, err := os.ReadFile(apiLockPath())
	if err != nil {
		return nil, err
	}
	var lock apiLockState
	if err := json.Unmarshal(raw, &lock); err != nil {
		return nil, fmt.Errorf("parse api lock: %w", err)
	}
	return &lock, nil
}

// acquireAPILock handles acquire api lock.
func acquireAPILock() error {
	if err := os.MkdirAll(runtimeDir(), 0o700); err != nil {
		return fmt.Errorf("create runtime dir: %w", err)
	}

	for attempt := 0; attempt < 2; attempt++ {
		fd, err := os.OpenFile(apiLockPath(), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			lock := apiLockState{
				OwnerPID:  os.Getpid(),
				CreatedAt: time.Now(),
			}
			raw, marshalErr := json.Marshal(lock)
			if marshalErr != nil {
				_ = fd.Close()
				return fmt.Errorf("marshal api lock: %w", marshalErr)
			}
			if _, writeErr := fd.Write(raw); writeErr != nil {
				_ = fd.Close()
				return fmt.Errorf("write api lock: %w", writeErr)
			}
			if closeErr := fd.Close(); closeErr != nil {
				return fmt.Errorf("close api lock: %w", closeErr)
			}
			return nil
		}
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("create api lock: %w", err)
		}

		lock, lockErr := loadAPILock()
		pid := 0
		if lockErr == nil && lock != nil {
			pid = lock.APIPID
		}
		// If lock metadata points to a dead process, fall back to runtime state.
		if pid > 0 && !processAlive(pid) {
			pid = 0
		}
		if pid <= 0 {
			state, stateErr := loadAPIState()
			if stateErr == nil && state != nil {
				pid = state.PID
			}
		}
		if pid > 0 && processAlive(pid) {
			return &apiLockHeldError{PID: pid}
		}

		_ = cleanupAPIState()
		_ = os.Remove(apiLockPath())
	}

	return fmt.Errorf("failed to acquire api lock")
}

// updateAPILockPID handles update api lock pid.
func updateAPILockPID(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid api pid: %d", pid)
	}
	lock := apiLockState{
		OwnerPID:  os.Getpid(),
		APIPID:    pid,
		CreatedAt: time.Now(),
	}
	raw, err := json.Marshal(lock)
	if err != nil {
		return fmt.Errorf("marshal api lock: %w", err)
	}
	if err := os.WriteFile(apiLockPath(), raw, 0o600); err != nil {
		return fmt.Errorf("write api lock: %w", err)
	}
	return nil
}

// cleanupAPIState handles cleanup apistate.
func cleanupAPIState() error {
	_ = os.Remove(apiPIDPath())
	_ = os.Remove(apiStatePath())
	return nil
}

// processAlive handles process alive.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	return errors.Is(err, syscall.EPERM)
}

// stopProcessIfAlive terminates a process when it is still alive.
func stopProcessIfAlive(pid int) {
	if pid <= 0 || !processAlive(pid) {
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = proc.Signal(syscall.SIGTERM)
	deadline := time.Now().Add(1500 * time.Millisecond)
	for processAlive(pid) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if processAlive(pid) {
		_ = proc.Signal(syscall.SIGKILL)
		time.Sleep(100 * time.Millisecond)
	}
}

// waitForAPIHealth handles wait for apihealth.
func waitForAPIHealth(timeout time.Duration) bool {
	if timeout <= 0 {
		timeout = 6 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status, err := waitForAPIHealthProbe()
		if err == nil && strings.TrimSpace(status) != "" {
			return true
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}

// isPortConflictLog handles is port conflict log.
func isPortConflictLog(logText string) bool {
	lower := strings.ToLower(logText)
	return strings.Contains(lower, "address already in use") ||
		strings.Contains(lower, "errno 98") ||
		strings.Contains(lower, "errno 48") ||
		strings.Contains(lower, "eaddrinuse")
}

// detectStartupFailure handles post-start checks for delayed conflict logs and exit races.
func detectStartupFailure(logPath string, pid int, timeout time.Duration) (bool, bool) {
	if timeout <= 0 {
		timeout = 900 * time.Millisecond
	}
	deadline := time.Now().Add(timeout)
	for {
		logBytes, _ := os.ReadFile(logPath)
		if isPortConflictLog(string(logBytes)) {
			return true, true
		}
		if !processAlive(pid) {
			return false, true
		}
		if time.Now().After(deadline) {
			return false, false
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// resolveServerDir handles resolve server dir.
func resolveServerDir() (string, error) {
	if env := strings.TrimSpace(os.Getenv("NEBULA_SERVER_DIR")); env != "" {
		if dir, ok := normalizeServerDirCandidate(env); ok {
			return dir, nil
		}
		return "", fmt.Errorf("NEBULA_SERVER_DIR does not point to a Nebula server dir: %s", env)
	}

	roots := []string{}
	if cwd, err := os.Getwd(); err == nil && cwd != "" {
		roots = append(roots, cwd)
		dir := cwd
		for i := 0; i < 6; i++ {
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			roots = append(roots, parent)
			dir = parent
		}
	}
	if exe, err := os.Executable(); err == nil && exe != "" {
		dir := filepath.Dir(exe)
		roots = append(roots, dir)
		for i := 0; i < 4; i++ {
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			roots = append(roots, parent)
			dir = parent
		}
	}

	seen := map[string]bool{}
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		candidates := []string{
			root,
			filepath.Join(root, "server"),
			filepath.Join(root, "nebula-core", "server"),
		}
		for _, pattern := range []string{
			filepath.Join(root, "*", "nebula-core", "server"),
			filepath.Join(root, "*", "*", "nebula-core", "server"),
		} {
			if matches, err := filepath.Glob(pattern); err == nil {
				candidates = append(candidates, matches...)
			}
		}
		for _, candidate := range candidates {
			if seen[candidate] {
				continue
			}
			seen[candidate] = true
			if dir, ok := normalizeServerDirCandidate(candidate); ok {
				return dir, nil
			}
		}
	}

	return "", fmt.Errorf("could not locate server dir; set NEBULA_SERVER_DIR to <repo>/server")
}

// normalizeServerDirCandidate handles normalize server dir candidate.
func normalizeServerDirCandidate(candidate string) (string, bool) {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return "", false
	}
	abs, err := filepath.Abs(candidate)
	if err != nil {
		return "", false
	}
	appPath := filepath.Join(abs, "src", "nebula_api", "app.py")
	info, err := os.Stat(appPath)
	if err != nil || info.IsDir() {
		return "", false
	}
	return abs, true
}

// tailLines handles tail lines.
func tailLines(lines []string, tail int) []string {
	clean := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		clean = append(clean, line)
	}
	if tail <= 0 {
		return clean
	}
	if len(clean) <= tail {
		return clean
	}
	return clean[len(clean)-tail:]
}
