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

// remoteRefExists returns true if a remote-tracking ref (e.g. origin/main) exists.
func remoteRefExists(ref string) bool {
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/remotes/"+ref)
	return cmd.Run() == nil
}

// DetectBaseBranch returns the repository's base branch (e.g. "main" or "master").
//
// It first asks git for the remote's default branch via
// `git symbolic-ref refs/remotes/origin/HEAD` (set by clone / `git remote set-head`).
// If that is unavailable it falls back to the first of main/master that exists
// as a remote-tracking ref or a local branch.
func DetectBaseBranch() (string, error) {
	cmd := exec.Command("git", "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if out, err := cmd.Output(); err == nil {
		ref := strings.TrimSpace(string(out)) // e.g. "origin/main"
		if i := strings.IndexByte(ref, '/'); i >= 0 {
			ref = ref[i+1:]
		}
		if ref != "" {
			return ref, nil
		}
	}

	for _, candidate := range []string{"main", "master"} {
		if remoteRefExists("origin/" + candidate) {
			return candidate, nil
		}
		if exists, _ := BranchExists(candidate); exists {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("could not detect a base branch (no main/master found) — pass --base")
}

// GetDiffBetweenBranches returns the diff that head introduces relative to base.
// It uses the three-dot range (base...head) so the result reflects only head's
// own changes — the same set GitHub shows in a pull request.
func GetDiffBetweenBranches(base, head string, contextLines int) (string, error) {
	cmd := exec.Command("git", "diff", fmt.Sprintf("-U%d", contextLines), base+"..."+head)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to diff %s...%s: %w", base, head, err)
	}
	return string(out), nil
}

// GetDiffStatBetweenBranches returns the --stat summary of base...head.
func GetDiffStatBetweenBranches(base, head string) (string, error) {
	cmd := exec.Command("git", "diff", "--stat", base+"..."+head)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to diff stat %s...%s: %w", base, head, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// GetCommitsBetweenBranches returns the subjects of commits in head but not base
// (the two-dot range base..head), newest first.
func GetCommitsBetweenBranches(base, head string) ([]string, error) {
	cmd := exec.Command("git", "log", base+".."+head, "--pretty=format:%s")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list commits %s..%s: %w", base, head, err)
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}
	return strings.Split(raw, "\n"), nil
}

// GetRemoteURL returns the URL configured for the named remote (e.g. "origin").
func GetRemoteURL(remote string) (string, error) {
	cmd := exec.Command("git", "remote", "get-url", remote)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("no %q remote configured", remote)
	}
	return strings.TrimSpace(string(out)), nil
}

// GetUserName returns the configured git user.name, or "" if it isn't set.
func GetUserName() string {
	cmd := exec.Command("git", "config", "user.name")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// HasUpstream reports whether the current branch has a configured upstream.
func HasUpstream() (bool, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	err := cmd.Run()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}
		return false, fmt.Errorf("failed to check upstream: %w", err)
	}
	return true, nil
}

// PushCurrentBranch pushes the current branch to origin and sets upstream tracking.
// Stdout/stderr are streamed so push progress and errors are visible live.
func PushCurrentBranch() error {
	cmd := exec.Command("git", "push", "-u", "origin", "HEAD")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}
	return nil
}
