package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// ── styles ──────────────────────────────────────────────────────────────────

var (
	purple = lipgloss.Color("#7f57b4")
	teal   = lipgloss.Color("#436b77")
	green  = lipgloss.Color("#3f866b")
	red    = lipgloss.Color("#d1606b")
	muted  = lipgloss.Color("#9ba0bf")
	text   = lipgloss.Color("#d7d9da")
	warm   = lipgloss.Color("#a7754e")

	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(purple)
	subStyle     = lipgloss.NewStyle().Faint(true).Italic(true).Foreground(muted)
	infoStyle    = lipgloss.NewStyle().Foreground(muted)
	successStyle = lipgloss.NewStyle().Bold(true).Foreground(green)
	errorStyle   = lipgloss.NewStyle().Bold(true).Foreground(red)
	warnStyle    = lipgloss.NewStyle().Foreground(warm)
	toolStyle    = lipgloss.NewStyle().Foreground(teal).Bold(true)
	textStyle    = lipgloss.NewStyle().Foreground(text)
	dimStyle     = lipgloss.NewStyle().Faint(true).Foreground(muted)
	headerStyle  = lipgloss.NewStyle().Foreground(teal).Bold(true)
)

// ── NDJSON event types ──────────────────────────────────────────────────────

type baseEvent struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype,omitempty"`
}

type streamEvent struct {
	Type  string `json:"type"`
	Event struct {
		Type         string `json:"type"`
		Index        int    `json:"index"`
		ContentBlock struct {
			Type string `json:"type"`
			Name string `json:"name"`
			ID   string `json:"id"`
		} `json:"content_block"`
		Delta struct {
			Type        string `json:"type"`
			Text        string `json:"text"`
			PartialJSON string `json:"partial_json"`
		} `json:"delta"`
	} `json:"event"`
}

type resultEvent struct {
	Type         string  `json:"type"`
	Subtype      string  `json:"subtype"`
	IsError      bool    `json:"is_error"`
	Result       string  `json:"result"`
	SessionID    string  `json:"session_id"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	DurationMs   int     `json:"duration_ms"`
	NumTurns     int     `json:"num_turns"`
}

type systemEvent struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	SessionID string `json:"session_id"`
	Model     string `json:"model"`
}

// ── helpers ─────────────────────────────────────────────────────────────────

func info(msg string)    { fmt.Println(infoStyle.Render("  " + msg)) }
func success(msg string) { fmt.Println(successStyle.Render("  " + msg)) }
func warn(msg string)    { fmt.Println(warnStyle.Render("  " + msg)) }
func fail(msg string)    { fmt.Println(errorStyle.Render("  " + msg)) }

func run(dir string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func runSilent(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

// ── capture TUI via tmux ────────────────────────────────────────────────────

func captureTUI(capturesDir, nebulaBin string) ([]string, error) {
	os.MkdirAll(capturesDir, 0o755)
	session := fmt.Sprintf("nebula-capture-%d", os.Getpid())

	// Start nebula in tmux
	if err := runSilent(".", "tmux", "new-session", "-d", "-s", session, "-x", "120", "-y", "40", nebulaBin); err != nil {
		return nil, fmt.Errorf("tmux start failed: %w", err)
	}
	defer func() {
		runSilent(".", "tmux", "kill-session", "-t", session)
	}()
	time.Sleep(3 * time.Second)

	tabs := []struct {
		key  string
		name string
	}{
		{"", "startup"},
		{"2", "entities"},
		{"3", "relationships"},
		{"4", "context"},
		{"5", "jobs"},
		{"6", "logs"},
		{"7", "files"},
		{"8", "protocols"},
		{"9", "history"},
		{"0", "settings"},
	}

	var captured []string

	for _, tab := range tabs {
		if tab.key != "" {
			runSilent(".", "tmux", "send-keys", "-t", session, tab.key)
			time.Sleep(1 * time.Second)
		}
		outPath := filepath.Join(capturesDir, tab.name+".ans")
		out, err := run(".", "tmux", "capture-pane", "-t", session, "-p", "-e")
		if err == nil {
			os.WriteFile(outPath, []byte(out), 0o644)
			captured = append(captured, outPath)
		}
	}

	// Command palette
	runSilent(".", "tmux", "send-keys", "-t", session, "/")
	time.Sleep(1 * time.Second)
	out, _ := run(".", "tmux", "capture-pane", "-t", session, "-p", "-e")
	palettePath := filepath.Join(capturesDir, "command_palette.ans")
	os.WriteFile(palettePath, []byte(out), 0o644)
	captured = append(captured, palettePath)

	// Help overlay
	runSilent(".", "tmux", "send-keys", "-t", session, "Escape")
	time.Sleep(500 * time.Millisecond)
	runSilent(".", "tmux", "send-keys", "-t", session, "?")
	time.Sleep(1 * time.Second)
	out, _ = run(".", "tmux", "capture-pane", "-t", session, "-p", "-e")
	helpPath := filepath.Join(capturesDir, "help.ans")
	os.WriteFile(helpPath, []byte(out), 0o644)
	captured = append(captured, helpPath)

	// Quit
	runSilent(".", "tmux", "send-keys", "-t", session, "q")

	return captured, nil
}

// ── claude headless ─────────────────────────────────────────────────────────

func spawnClaude(prompt, cwd, model string, maxTurns int) error {
	args := []string{
		"-p",
		"--input-format", "stream-json",
		"--output-format", "stream-json",
		"--verbose",
		"--include-partial-messages",
		"--dangerously-skip-permissions",
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	if maxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", maxTurns))
	}

	cmd := exec.Command("claude", args...)
	cmd.Dir = cwd

	// Clean env - remove CLAUDECODE to avoid subprocess conflicts
	env := os.Environ()
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, "CLAUDECODE=") {
			filtered = append(filtered, e)
		}
	}
	cmd.Env = filtered

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start claude: %w", err)
	}

	// Send prompt as stream-json
	msg, _ := json.Marshal(map[string]any{
		"type": "user",
		"message": map[string]any{
			"role":    "user",
			"content": []map[string]string{{"type": "text", "text": prompt}},
		},
	})
	fmt.Fprintf(stdin, "%s\n", msg)

	// Read NDJSON stream from claude stdout
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	activeTools := make(map[int]string) // index -> tool name
	lineCount := 0

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var base baseEvent
		json.Unmarshal(line, &base)

		switch base.Type {
		case "system":
			var sys systemEvent
			json.Unmarshal(line, &sys)
			if sys.Subtype == "init" {
				fmt.Println(dimStyle.Render(fmt.Sprintf("  session: %s  model: %s", sys.SessionID, sys.Model)))
			}

		case "stream_event":
			var se streamEvent
			json.Unmarshal(line, &se)

			switch se.Event.Type {
			case "content_block_start":
				if se.Event.ContentBlock.Type == "tool_use" {
					activeTools[se.Event.Index] = se.Event.ContentBlock.Name
					fmt.Print(toolStyle.Render(fmt.Sprintf("\n  [%s] ", se.Event.ContentBlock.Name)))
				}

			case "content_block_delta":
				if se.Event.Delta.Type == "text_delta" {
					fmt.Print(se.Event.Delta.Text)
					lineCount++
				} else if se.Event.Delta.Type == "input_json_delta" {
					// Tool input streaming - show abbreviated
					if len(se.Event.Delta.PartialJSON) > 0 && lineCount%20 == 0 {
						fmt.Print(dimStyle.Render("."))
					}
					lineCount++
				}

			case "content_block_stop":
				if name, ok := activeTools[se.Event.Index]; ok {
					fmt.Println(dimStyle.Render(fmt.Sprintf(" [/%s]", name)))
					delete(activeTools, se.Event.Index)
				}
			}

		case "result":
			var res resultEvent
			json.Unmarshal(line, &res)
			fmt.Println()
			if res.IsError {
				fail(fmt.Sprintf("error: %s", res.Result))
			} else {
				fmt.Println(dimStyle.Render(fmt.Sprintf("  cost: $%.4f  turns: %d  duration: %ds",
					res.TotalCostUSD, res.NumTurns, res.DurationMs/1000)))
			}
			stdin.Close()
			break
		}
	}

	return cmd.Wait()
}

// ── main ────────────────────────────────────────────────────────────────────

func main() {
	iterations := flag.Int("iterations", 5, "number of iterations")
	branch := flag.String("branch", "feat/charm-migration", "target branch")
	model := flag.String("model", "opus", "claude model for analysis")
	maxTurns := flag.Int("max-turns", 25, "max turns per iteration")
	flag.Parse()

	root, _ := os.Getwd()
	// Walk up to find repo root (has .git)
	for {
		if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			fail("could not find git repo root")
			os.Exit(1)
		}
		root = parent
	}

	scriptDir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	programMD := filepath.Join(scriptDir, "program.md")
	if _, err := os.Stat(programMD); err != nil {
		// Try relative to cwd
		programMD = filepath.Join("experiments", "autoresearch", "program.md")
	}

	worktreeDir := filepath.Join(root, ".autoresearch-work")

	// ── banner ────────────────────────────────────────────────────────────
	fmt.Println()
	lipgloss.Println(titleStyle.Render("Nebula Autoresearch"))
	lipgloss.Println(subStyle.Render("Autonomous Visual Bug Hunter"))
	fmt.Println()
	fmt.Println(infoStyle.Render(fmt.Sprintf("  Iterations   %d", *iterations)))
	fmt.Println(infoStyle.Render(fmt.Sprintf("  Branch       %s", *branch)))
	fmt.Println(infoStyle.Render(fmt.Sprintf("  Model        %s", *model)))
	fmt.Println(infoStyle.Render("  Isolation    Git Worktrees"))
	fmt.Println()

	totalFixes := 0

	for i := 1; i <= *iterations; i++ {
		fmt.Println()
		fmt.Println(headerStyle.Render(fmt.Sprintf("  iteration %d/%d", i, *iterations)))
		fmt.Println()

		iterBranch := fmt.Sprintf("autoresearch/iter-%d", i)
		iterDir := filepath.Join(worktreeDir, fmt.Sprintf("iter-%d", i))

		// ── cleanup leftover ──────────────────────────────────────────────
		if _, err := os.Stat(iterDir); err == nil {
			runSilent(root, "git", "worktree", "remove", "--force", iterDir)
			runSilent(root, "git", "branch", "-D", iterBranch)
		}

		// ── create worktree ───────────────────────────────────────────────
		runSilent(root, "git", "branch", "-D", iterBranch)
		if _, err := run(root, "git", "branch", iterBranch, *branch); err != nil {
			fail("failed to create branch: " + iterBranch)
			continue
		}
		if _, err := run(root, "git", "worktree", "add", iterDir, iterBranch); err != nil {
			fail("failed to create worktree")
			runSilent(root, "git", "branch", "-D", iterBranch)
			continue
		}
		info("worktree: " + iterDir)

		// ── run tests ─────────────────────────────────────────────────────
		info("running tests...")
		testOut, testErr := run(iterDir, "make", "test-cli")
		if testErr != nil {
			warn("test issues")
		} else {
			info("tests: all passing")
		}

		// ── build ─────────────────────────────────────────────────────────
		info("building nebula...")
		if _, err := run(iterDir, "make", "build"); err != nil {
			fail("build failed")
			runSilent(root, "git", "worktree", "remove", "--force", iterDir)
			runSilent(root, "git", "branch", "-D", iterBranch)
			continue
		}

		// ── capture TUI ───────────────────────────────────────────────────
		info("capturing TUI...")
		capturesDir := filepath.Join(iterDir, "captures")
		nebulaBin := filepath.Join(iterDir, "cli", "src", "build", "nebula")
		captured, err := captureTUI(capturesDir, nebulaBin)
		if err != nil {
			warn("capture failed: " + err.Error())
		} else {
			info(fmt.Sprintf("captured %d snapshots", len(captured)))
		}

		// ── build prompt ──────────────────────────────────────────────────
		programBytes, _ := os.ReadFile(programMD)
		captureInstructions := ""
		for _, path := range captured {
			name := strings.TrimSuffix(filepath.Base(path), ".ans")
			captureInstructions += fmt.Sprintf("\n- Read %s to see the %s tab", path, name)
		}

		suiteStatus := "unknown"
		lines := strings.Split(testOut, "\n")
		if len(lines) > 0 {
			suiteStatus = lines[len(lines)-1]
		}

		prompt := fmt.Sprintf(`%s

## Current State (iteration %d)
- Full suite: %s

## CRITICAL: Read the captured terminal output before making changes

Terminal snapshots captured at: %s/
%s

## Task
1. Read each .ans capture file - these show ACTUAL rendered TUI output
2. Analyze for visual bugs (broken borders, misaligned columns, overflow, spacing)
3. Fix bugs in cli/src/internal/ui/
4. Run: make test-cli to verify
5. Use /commit-forge to commit changes`,
			string(programBytes), i, suiteStatus, capturesDir, captureInstructions)

		// ── launch claude ─────────────────────────────────────────────────
		fmt.Println()
		fmt.Println(headerStyle.Render("  claude analyzing..."))
		fmt.Println()

		if err := spawnClaude(prompt, iterDir, *model, *maxTurns); err != nil {
			warn("claude session ended: " + err.Error())
		}

		// ── check results ─────────────────────────────────────────────────
		fmt.Println()
		commitsAhead, _ := run(iterDir, "git", "log", *branch+".."+iterBranch, "--oneline")
		commitCount := 0
		if strings.TrimSpace(commitsAhead) != "" {
			commitCount = len(strings.Split(strings.TrimSpace(commitsAhead), "\n"))
		}

		if commitCount == 0 {
			// Check for uncommitted changes
			changed, _ := run(iterDir, "git", "diff", "--name-only")
			if strings.TrimSpace(changed) != "" {
				info("saving uncommitted changes...")
				runSilent(iterDir, "git", "add", "-A")
				runSilent(iterDir, "git", "commit", "-m", fmt.Sprintf("fix(cli): autoresearch visual fixes (iteration %d)", i))
				commitCount = 1
			} else {
				info("no changes this iteration")
				runSilent(root, "git", "worktree", "remove", "--force", iterDir)
				runSilent(root, "git", "branch", "-D", iterBranch)
				continue
			}
		}

		info(fmt.Sprintf("%d commit(s) from claude", commitCount))

		// ── verify tests ──────────────────────────────────────────────────
		info("verifying tests...")
		if err := runSilent(iterDir, "make", "test-cli"); err != nil {
			fail("tests failed, keeping branch: " + iterBranch)
			runSilent(root, "git", "worktree", "remove", "--force", iterDir)
			continue
		}

		// ── merge ─────────────────────────────────────────────────────────
		runSilent(root, "git", "checkout", *branch)
		if _, err := run(root, "git", "merge", iterBranch, "--no-edit"); err != nil {
			fail("merge conflict, keeping branch: " + iterBranch)
			runSilent(root, "git", "merge", "--abort")
			runSilent(root, "git", "worktree", "remove", "--force", iterDir)
			continue
		}

		success(fmt.Sprintf("merged iteration %d into %s", i, *branch))
		totalFixes++

		// ── cleanup ───────────────────────────────────────────────────────
		runSilent(root, "git", "worktree", "remove", "--force", iterDir)
		runSilent(root, "git", "branch", "-D", iterBranch)
	}

	// ── summary ─────────────────────────────────────────────────────────────
	fmt.Println()
	lipgloss.Println(successStyle.Render("Autoresearch Complete"))
	fmt.Println()
	fmt.Println(infoStyle.Render(fmt.Sprintf("  Iterations    %d", *iterations)))
	fmt.Println(infoStyle.Render(fmt.Sprintf("  Fixes Merged  %d", totalFixes)))
	fmt.Println(infoStyle.Render(fmt.Sprintf("  Branch        %s", *branch)))
	fmt.Println()

	os.RemoveAll(worktreeDir)
}
