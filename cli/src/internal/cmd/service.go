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
)

type apiRuntimeState struct {
	PID       int       `json:"pid"`
	Port      int       `json:"port"`
	ServerDir string    `json:"server_dir"`
	LogPath   string    `json:"log_path"`
	StartedAt time.Time `json:"started_at"`
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

	ready := waitForAPIHealth(8 * time.Second)
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
	state, _ := loadAPIState()
	pid := 0
	if state != nil {
		pid = state.PID
	}
	if pid <= 0 {
		if pidFromFile, ok := readPIDFile(); ok {
			pid = pidFromFile
		}
	}
	if pid <= 0 {
		renderCommandMessage(out, "Nebula API", "API is not running.")
		return nil
	}

	if !processAlive(pid) {
		_ = cleanupAPIState()
		renderCommandMessage(out, "Nebula API", "API was not running. cleaned stale runtime files.")
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

// readPIDFile handles read pidfile.
func readPIDFile() (int, bool) {
	raw, err := os.ReadFile(apiPIDPath())
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
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

// waitForAPIHealth handles wait for apihealth.
func waitForAPIHealth(timeout time.Duration) bool {
	if timeout <= 0 {
		timeout = 6 * time.Second
	}
	client := api.NewDefaultClient("", 900*time.Millisecond)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status, err := client.Health()
		if err == nil && strings.TrimSpace(status) != "" {
			return true
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
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
			filepath.Join(root, "00-The-Void", "Nebula", "nebula-core", "server"),
			filepath.Join(root, "nebula-core", "server"),
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
	if _, err := os.Stat(appPath); err != nil {
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
	if len(clean) <= tail {
		return clean
	}
	return clean[len(clean)-tail:]
}
