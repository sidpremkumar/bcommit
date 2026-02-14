package diff

import (
	"regexp"
	"strings"
)

// FileDiff represents the parsed diff for a single file.
type FileDiff struct {
	Filename    string
	OldFilename string // non-empty for renames
	IsNew       bool
	IsDeleted   bool
	IsBinary    bool
	Hunks       []string
	Raw         string // original diff text for this file
	Additions   int
	Deletions   int
}

var diffHeaderRe = regexp.MustCompile(`^diff --git a/(.+) b/(.+)$`)

// Parse splits a raw unified diff into per-file FileDiff structs.
func Parse(rawDiff string) []FileDiff {
	if strings.TrimSpace(rawDiff) == "" {
		return nil
	}

	lines := strings.Split(rawDiff, "\n")
	var files []FileDiff
	var current *FileDiff
	var currentLines []string

	flush := func() {
		if current != nil {
			current.Raw = strings.Join(currentLines, "\n")
			countChanges(current)
			files = append(files, *current)
		}
	}

	for _, line := range lines {
		if m := diffHeaderRe.FindStringSubmatch(line); m != nil {
			flush()
			current = &FileDiff{
				Filename: m[2],
			}
			if m[1] != m[2] {
				current.OldFilename = m[1]
			}
			currentLines = []string{line}
			continue
		}

		if current == nil {
			continue
		}

		currentLines = append(currentLines, line)

		// Detect new/deleted/binary from metadata lines
		switch {
		case strings.HasPrefix(line, "new file mode"):
			current.IsNew = true
		case strings.HasPrefix(line, "deleted file mode"):
			current.IsDeleted = true
		case strings.HasPrefix(line, "Binary files"):
			current.IsBinary = true
		case strings.HasPrefix(line, "@@"):
			current.Hunks = append(current.Hunks, line)
		}
	}

	flush()
	return files
}

// countChanges counts additions and deletions in a file diff.
func countChanges(f *FileDiff) {
	for _, line := range strings.Split(f.Raw, "\n") {
		if len(line) == 0 {
			continue
		}
		switch line[0] {
		case '+':
			if !strings.HasPrefix(line, "+++") {
				f.Additions++
			}
		case '-':
			if !strings.HasPrefix(line, "---") {
				f.Deletions++
			}
		}
	}
}

// Reassemble converts a slice of FileDiff back into a unified diff string.
func Reassemble(files []FileDiff) string {
	var parts []string
	for _, f := range files {
		parts = append(parts, f.Raw)
	}
	return strings.Join(parts, "\n")
}
