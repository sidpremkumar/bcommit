package llm

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/ollama/ollama/api"
	"github.com/sidpremkumar/bcommit/internal/ui"
)

// DefaultModel is the recommended model for commit message generation.
const DefaultModel = "qwen2.5-coder:3b"

// LLM call timeouts. Small local models can stall on pathological inputs;
// these bounds let us fail fast and fall back instead of hanging the CLI.
const (
	commitMessageTimeout = 3 * time.Minute
	branchNameTimeout    = 90 * time.Second
	summarizeTimeout     = 60 * time.Second
)

// Client wraps the Ollama API for commit message generation.
type Client struct {
	client        *api.Client
	model         string
	numCtx        int
	temperature   float64
	numPredict    int
	serverCmd     *exec.Cmd // non-nil only if we started the Ollama server
	startedServer bool      // true if we started the server ourselves
}

// NewClient creates a new LLM client configured for commit message generation.
func NewClient(model string, numCtx int, temperature float64) (*Client, error) {
	c, err := api.ClientFromEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to create Ollama client: %w", err)
	}
	return &Client{
		client:      c,
		model:       model,
		numCtx:      numCtx,
		temperature: temperature,
		numPredict:  256,
	}, nil
}

func ptrBool(b bool) *bool { return &b }

// GenerateCommitMessage sends the diff to the LLM and returns a commit message.
func (c *Client) GenerateCommitMessage(diffContent, diffStat, hint, forceType string) (string, error) {
	userPrompt := BuildUserPrompt(diffContent, diffStat, hint, forceType)

	messages := []api.Message{
		{Role: "system", Content: SystemPrompt},
		{Role: "user", Content: userPrompt},
	}

	req := &api.ChatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   ptrBool(false),
		Options: map[string]any{
			"temperature": c.temperature,
			"num_ctx":     c.numCtx,
			"num_predict": c.numPredict,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), commitMessageTimeout)
	defer cancel()

	var result string
	err := c.client.Chat(ctx, req, func(resp api.ChatResponse) error {
		result = resp.Message.Content
		return nil
	})
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("LLM request timed out after %s — try a smaller diff, a larger model, or use --hint", commitMessageTimeout)
		}
		return "", fmt.Errorf("LLM request failed: %w", err)
	}

	return cleanResponse(result), nil
}

// GenerateBranchName sends the diff to the LLM and returns a branch name.
func (c *Client) GenerateBranchName(diffContent, diffStat, hint string) (string, error) {
	userPrompt := BuildBranchNamePrompt(diffContent, diffStat, hint)

	messages := []api.Message{
		{Role: "system", Content: BranchNameSystemPrompt},
		{Role: "user", Content: userPrompt},
	}

	req := &api.ChatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   ptrBool(false),
		Options: map[string]any{
			"temperature": c.temperature,
			"num_ctx":     c.numCtx,
			"num_predict": 64,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), branchNameTimeout)
	defer cancel()

	var result string
	err := c.client.Chat(ctx, req, func(resp api.ChatResponse) error {
		result = resp.Message.Content
		return nil
	})
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("branch-name generation timed out after %s", branchNameTimeout)
		}
		return "", fmt.Errorf("LLM request failed: %w", err)
	}

	cleaned := cleanBranchName(result)
	if cleaned == "" {
		return "", fmt.Errorf("LLM produced an empty branch name — try providing a --hint")
	}
	return cleaned, nil
}

// cleanBranchName sanitizes LLM output into a valid git branch name.
func cleanBranchName(raw string) string {
	s := strings.TrimSpace(raw)

	// Remove markdown fences
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		var cleaned []string
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				continue
			}
			cleaned = append(cleaned, line)
		}
		s = strings.Join(cleaned, "\n")
		s = strings.TrimSpace(s)
	}

	// Remove surrounding quotes
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		s = s[1 : len(s)-1]
	}

	// Take only the first line
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		s = s[:idx]
	}
	s = strings.TrimSpace(s)

	// Lowercase
	s = strings.ToLower(s)

	// Replace spaces and underscores with hyphens
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")

	// Remove anything that isn't a-z, 0-9, or hyphen
	branchCharRe := regexp.MustCompile(`[^a-z0-9-]`)
	s = branchCharRe.ReplaceAllString(s, "")

	// Collapse consecutive hyphens
	multiHyphen := regexp.MustCompile(`-{2,}`)
	s = multiHyphen.ReplaceAllString(s, "-")

	// Trim leading/trailing hyphens
	s = strings.Trim(s, "-")

	// Truncate to 50 chars at a hyphen boundary
	if len(s) > 50 {
		s = s[:50]
		if idx := strings.LastIndexByte(s, '-'); idx > 0 {
			s = s[:idx]
		}
	}

	return s
}

// SummarizeFileDiff asks the LLM to summarize a single file's changes (Tier 3).
func (c *Client) SummarizeFileDiff(filename, fileDiff string) (string, error) {
	userPrompt := BuildSummarizePrompt(filename, fileDiff)

	messages := []api.Message{
		{Role: "system", Content: SummarizeSystemPrompt},
		{Role: "user", Content: userPrompt},
	}

	req := &api.ChatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   ptrBool(false),
		Options: map[string]any{
			"temperature": 0.2,
			"num_ctx":     c.numCtx,
			"num_predict": 128,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), summarizeTimeout)
	defer cancel()

	var result string
	err := c.client.Chat(ctx, req, func(resp api.ChatResponse) error {
		result = resp.Message.Content
		return nil
	})
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("summarization timed out for %s after %s", filename, summarizeTimeout)
		}
		return "", fmt.Errorf("summarization failed for %s: %w", filename, err)
	}

	return strings.TrimSpace(result), nil
}

// EnsureReady performs a 3-step bootstrap: check Ollama installed, start server if needed, pull model if needed.
func (c *Client) EnsureReady(ctx context.Context) error {
	// Step 1: Is Ollama installed?
	if _, err := exec.LookPath("ollama"); err != nil {
		ui.PrintError("Ollama is not installed.")
		fmt.Println("  Install it from https://ollama.com or run: brew install ollama")
		return fmt.Errorf("ollama not found in PATH")
	}

	// Step 2: Is the server running?
	if !c.isServerRunning(ctx) {
		ui.PrintStatus("Ollama server is not running. Starting it...")

		cmd := exec.Command("ollama", "serve")
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Start(); err != nil {
			ui.PrintError("Failed to start Ollama server.")
			fmt.Println("  Try running 'ollama serve' manually in another terminal.")
			return fmt.Errorf("failed to start ollama serve: %w", err)
		}

		// Poll until server is ready (up to 10 seconds)
		if !c.waitForServer(ctx, 10*time.Second) {
			cmd.Process.Kill()
			ui.PrintError("Ollama server failed to start within 10 seconds.")
			fmt.Println("  Try running 'ollama serve' manually in another terminal.")
			return fmt.Errorf("ollama server did not become ready")
		}

		// Track that we started this server so we can shut it down later
		c.serverCmd = cmd
		c.startedServer = true
		ui.PrintSuccess("Ollama server started (will shut down on exit).")
	}

	// Step 3: Is the model available?
	if !c.isModelAvailable(ctx) {
		question := fmt.Sprintf("Model '%s' is not downloaded yet. Pull it now?", c.model)
		if !ui.PromptYesNo(question) {
			fmt.Printf("  You can pull it later with: ollama pull %s\n", c.model)
			return fmt.Errorf("model not available")
		}

		if err := c.pullModel(ctx); err != nil {
			ui.PrintError(fmt.Sprintf("Failed to pull model: %v", err))
			fmt.Printf("  Try manually: ollama pull %s\n", c.model)
			return err
		}
		fmt.Println() // newline after progress
		ui.PrintSuccess("Model ready.")
	}

	return nil
}

// isServerRunning checks if the Ollama server is reachable.
func (c *Client) isServerRunning(ctx context.Context) bool {
	_, err := c.client.List(ctx)
	return err == nil
}

// waitForServer polls the Ollama server until it responds or the timeout expires.
func (c *Client) waitForServer(ctx context.Context, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if c.isServerRunning(ctx) {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

// isModelAvailable checks if the configured model is available locally.
func (c *Client) isModelAvailable(ctx context.Context) bool {
	resp, err := c.client.List(ctx)
	if err != nil {
		return false
	}
	for _, m := range resp.Models {
		if strings.HasPrefix(m.Name, c.model) {
			return true
		}
	}
	return false
}

// pullModel downloads the configured model with progress output.
func (c *Client) pullModel(ctx context.Context) error {
	req := &api.PullRequest{
		Model: c.model,
	}

	return c.client.Pull(ctx, req, func(resp api.ProgressResponse) error {
		if resp.Total > 0 {
			pct := float64(resp.Completed) / float64(resp.Total) * 100
			completedMB := resp.Completed / (1024 * 1024)
			totalMB := resp.Total / (1024 * 1024)
			ui.PrintProgress(fmt.Sprintf("Pulling %s... %.0f%% (%dMB/%dMB)  ", c.model, pct, completedMB, totalMB))
		} else if resp.Status != "" {
			ui.PrintProgress(fmt.Sprintf("Pulling %s... %s  ", c.model, resp.Status))
		}
		return nil
	})
}

// Shutdown stops the Ollama server if this client started it.
// It sends SIGTERM first, then escalates to SIGKILL after 5 seconds.
func (c *Client) Shutdown() {
	if !c.startedServer || c.serverCmd == nil || c.serverCmd.Process == nil {
		return
	}

	ui.PrintStatus("Shutting down Ollama server (we started it)...")

	if err := c.serverCmd.Process.Signal(syscall.SIGTERM); err != nil {
		// Process may have already exited
		ui.PrintStatus("Ollama server already stopped.")
		return
	}

	// Wait up to 5 seconds for a clean exit
	done := make(chan error, 1)
	go func() {
		done <- c.serverCmd.Wait()
	}()

	select {
	case <-done:
		ui.PrintSuccess("Ollama server stopped.")
	case <-time.After(5 * time.Second):
		c.serverCmd.Process.Kill()
		<-done // reap the process
		ui.PrintSuccess("Ollama server stopped (forced).")
	}

	c.startedServer = false
	c.serverCmd = nil
}

// conventionalCommitRe matches a valid conventional commit first line.
var conventionalCommitRe = regexp.MustCompile(`^(feat|fix|refactor|docs|style|test|chore|perf|ci|build)(\(.+\))?!?: .+`)

// cleanResponse strips common LLM artifacts from the response.
func cleanResponse(raw string) string {
	s := strings.TrimSpace(raw)

	// Remove markdown code fences
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		var cleaned []string
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				continue
			}
			cleaned = append(cleaned, line)
		}
		s = strings.Join(cleaned, "\n")
		s = strings.TrimSpace(s)
	}

	// Remove "Commit message:" prefix
	for _, prefix := range []string{"Commit message:", "commit message:", "Commit Message:"} {
		s = strings.TrimPrefix(s, prefix)
		s = strings.TrimSpace(s)
	}

	// Remove surrounding quotes
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}

	return s
}
