package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// HasStagedChanges returns true if there are staged changes in the current repo.
func HasStagedChanges() (bool, error) {
	cmd := exec.Command("git", "diff", "--cached", "--quiet")
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Exit code 1 means there are differences
			if exitErr.ExitCode() == 1 {
				return true, nil
			}
		}
		return false, fmt.Errorf("failed to check staged changes: %w", err)
	}
	// Exit code 0 means no differences
	return false, nil
}

// GetStagedDiff returns the diff of staged changes with the given number of context lines.
func GetStagedDiff(contextLines int) (string, error) {
	cmd := exec.Command("git", "diff", "--cached", fmt.Sprintf("-U%d", contextLines))
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get staged diff: %w", err)
	}
	return string(out), nil
}

// GetDiffStat returns the --stat summary of staged changes.
func GetDiffStat() (string, error) {
	cmd := exec.Command("git", "diff", "--cached", "--stat")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get diff stat: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Commit creates a git commit with the given message.
func Commit(message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}
	return nil
}

// GetRecentCommits returns the last n commit subjects.
func GetRecentCommits(n int) ([]string, error) {
	cmd := exec.Command("git", "log", fmt.Sprintf("-%d", n), "--pretty=format:%s")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get recent commits: %w", err)
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}
	return strings.Split(raw, "\n"), nil
}
