// Package gh wraps the GitHub `gh` CLI for pull request creation. It shells out
// to gh (reusing the user's existing authentication) rather than talking to the
// GitHub API directly, matching how the rest of bcommit invokes git and ollama.
package gh

import (
	"fmt"
	"os/exec"
	"strings"
)

// Available reports whether the gh binary is on PATH.
func Available() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

// Authenticated returns nil if gh has a logged-in account, otherwise an error
// instructing the user to authenticate.
func Authenticated() error {
	cmd := exec.Command("gh", "auth", "status")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh is not authenticated — run: gh auth login")
	}
	return nil
}

// CreatePROptions carries the arguments for `gh pr create`.
type CreatePROptions struct {
	Base      string
	Title     string
	Body      string
	Draft     bool
	Reviewers []string
}

// ErrPRExists indicates a pull request already exists for the current branch.
type ErrPRExists struct {
	Detail string
}

func (e *ErrPRExists) Error() string {
	if e.Detail != "" {
		return e.Detail
	}
	return "a pull request already exists for this branch"
}

// CreatePR runs `gh pr create` and returns the URL of the created PR.
// The body is passed as a single argv element (no shell), so no escaping is
// needed. If a PR already exists for the branch, it returns *ErrPRExists.
func CreatePR(opts CreatePROptions) (string, error) {
	args := []string{"pr", "create", "--base", opts.Base, "--title", opts.Title, "--body", opts.Body}
	if opts.Draft {
		args = append(args, "--draft")
	}
	if len(opts.Reviewers) > 0 {
		args = append(args, "--reviewer", strings.Join(opts.Reviewers, ","))
	}

	cmd := exec.Command("gh", args...)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		if strings.Contains(output, "already exists") {
			return "", &ErrPRExists{Detail: output}
		}
		if output != "" {
			return "", fmt.Errorf("gh pr create failed: %s", output)
		}
		return "", fmt.Errorf("gh pr create failed: %w", err)
	}

	// gh prints the PR URL on success; return the last URL-looking line.
	return extractURL(output), nil
}

// ViewPRURL returns the URL of the existing PR for the current branch, if any.
func ViewPRURL() (string, error) {
	cmd := exec.Command("gh", "pr", "view", "--json", "url", "--jq", ".url")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// extractURL returns the last line that looks like a URL, falling back to the
// trimmed input.
func extractURL(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			return line
		}
	}
	return strings.TrimSpace(s)
}
