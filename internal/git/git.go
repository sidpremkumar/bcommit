package git

import (
	"fmt"
	"os"
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
// Stdout/stderr are streamed to the user's terminal so that pre-commit hook
// output (progress, errors, fixups) is visible live.
func Commit(message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("git commit failed (exit code %d) — see output above (likely a pre-commit hook)", exitErr.ExitCode())
		}
		return fmt.Errorf("git commit failed: %w", err)
	}
	return nil
}

// GetCurrentBranch returns the name of the current branch.
func GetCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// BranchExists returns true if a local branch with the given name exists.
func BranchExists(name string) (bool, error) {
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+name)
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("failed to check branch existence: %w", err)
	}
	return true, nil
}

// CreateAndCheckoutBranch creates a new branch and switches to it.
func CreateAndCheckoutBranch(name string) error {
	cmd := exec.Command("git", "checkout", "-b", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create branch: %s", strings.TrimSpace(string(out)))
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
