// Package contextstore manages per-repository custom context used to steer
// PR generation. Context files are stored centrally (not committed to the repo)
// under <config dir>/context/, keyed by a normalized "org/repo" derived from the
// git remote URL.
package contextstore

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sidpremkumar/bcommit/internal/config"
)

// nonKeyChars matches characters not allowed in a repo-key path segment.
var nonKeyChars = regexp.MustCompile(`[^a-zA-Z0-9._/-]`)

// RepoKey normalizes a git remote URL to an "org/repo" identifier.
//
//	git@github.com:org/repo.git      -> org/repo
//	https://github.com/org/repo.git  -> org/repo
//	ssh://git@github.com/org/repo     -> org/repo
//
// Unparseable remotes fall back to a sanitized slug of the whole URL so the
// caller never gets an empty key.
func RepoKey(remoteURL string) string {
	s := strings.TrimSpace(remoteURL)
	if s == "" {
		return "unknown"
	}

	// Strip scheme: https://, http://, ssh://, git://
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}

	// Strip user@ prefix (e.g. git@github.com:org/repo.git)
	if i := strings.Index(s, "@"); i >= 0 {
		s = s[i+1:]
	}

	// Strip host. The host is separated from the path by ':' (scp-like) or '/'.
	if i := strings.IndexAny(s, ":/"); i >= 0 {
		s = s[i+1:]
	}

	// Drop trailing .git and surrounding slashes.
	s = strings.TrimSuffix(s, ".git")
	s = strings.Trim(s, "/")

	if s == "" {
		s = remoteURL
	}

	// Sanitize anything unexpected; collapse the result.
	s = nonKeyChars.ReplaceAllString(s, "-")
	s = strings.Trim(s, "/-")
	if s == "" {
		return "unknown"
	}
	return s
}

// fileName converts a repo key into a flat, filesystem-safe filename.
func fileName(repoKey string) string {
	return strings.ReplaceAll(repoKey, "/", "__") + ".md"
}

// dir returns the directory holding context files.
func dir() string {
	return filepath.Join(config.Dir(), "context")
}

// Path returns the on-disk path of the context file for the given repo key.
func Path(repoKey string) string {
	return filepath.Join(dir(), fileName(repoKey))
}

// Load returns the stored context for the repo key, or "" if none exists.
// Lines beginning with "#" are treated as comments and stripped, so the starter
// template's instructions never reach the LLM.
func Load(repoKey string) (string, error) {
	data, err := os.ReadFile(Path(repoKey))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read context file: %w", err)
	}

	var kept []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n")), nil
}

// EnsurePath returns the context file path, creating the directory and an empty
// (templated) file if it does not yet exist, so an editor can open it.
func EnsurePath(repoKey string) (string, error) {
	if err := os.MkdirAll(dir(), 0755); err != nil {
		return "", fmt.Errorf("failed to create context dir: %w", err)
	}
	path := Path(repoKey)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		template := fmt.Sprintf(template, repoKey)
		if err := os.WriteFile(path, []byte(template), 0644); err != nil {
			return "", fmt.Errorf("failed to create context file: %w", err)
		}
	}
	return path, nil
}

// template is the starter content for a new context file.
const template = `# bcommit PR context for %s
#
# Anything below (lines starting with "#" are ignored) is fed to the LLM as
# high-priority guidance when generating pull request descriptions for this repo.
# Use it for: team conventions, PR template sections, reviewers to mention,
# domain terminology, links to follow, or tone.

`
