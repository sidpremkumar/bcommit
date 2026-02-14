package diff

import (
	"fmt"
	"path/filepath"
	"strings"
)

// lockFiles is the set of known lock/dependency files to filter out.
var lockFiles = map[string]bool{
	"package-lock.json": true,
	"yarn.lock":         true,
	"pnpm-lock.yaml":    true,
	"Pipfile.lock":      true,
	"poetry.lock":       true,
	"uv.lock":           true,
	"Gemfile.lock":      true,
	"composer.lock":     true,
	"Cargo.lock":        true,
	"go.sum":            true,
}

// generatedPatterns are glob patterns for auto-generated files.
var generatedPatterns = []string{
	"*.generated.*",
	"*.pb.go",
	"*_pb2.py",
	"*.min.js",
	"*.min.css",
	"*.bundle.js",
	"*.snap",
}

// generatedMarkers are strings found in generated file headers.
var generatedMarkers = []string{
	"DO NOT EDIT",
	"auto-generated",
	"Auto-generated",
	"AUTO-GENERATED",
	"Code generated",
	"This file is generated",
}

// FilterResult contains the filtered files and notes about what was removed.
type FilterResult struct {
	Files []FileDiff
	Notes []string
}

// Filter applies all noise-reduction filters to the parsed diff.
func Filter(files []FileDiff, extraExcludes []string) FilterResult {
	var result FilterResult

	for _, f := range files {
		base := filepath.Base(f.Filename)

		// 1. Lock files
		if lockFiles[base] {
			result.Notes = append(result.Notes, fmt.Sprintf("[lock file updated: %s]", f.Filename))
			continue
		}

		// 2. Generated code
		if isGenerated(f) {
			result.Notes = append(result.Notes, fmt.Sprintf("[generated file modified: %s]", f.Filename))
			continue
		}

		// 3. Binary files
		if f.IsBinary {
			result.Notes = append(result.Notes, fmt.Sprintf("[binary file changed: %s]", f.Filename))
			continue
		}

		// 4. Extra exclude patterns from config
		if matchesAny(f.Filename, extraExcludes) {
			result.Notes = append(result.Notes, fmt.Sprintf("[excluded: %s]", f.Filename))
			continue
		}

		// 5. Whitespace-only changes
		if isWhitespaceOnly(f) {
			result.Notes = append(result.Notes, fmt.Sprintf("[whitespace changes: %s]", f.Filename))
			continue
		}

		// 6. Collapse large hunks
		if f.Additions+f.Deletions > 200 {
			result.Notes = append(result.Notes, fmt.Sprintf("[large change: +%d -%d lines in %s]", f.Additions, f.Deletions, f.Filename))
			continue
		}

		result.Files = append(result.Files, f)
	}

	return result
}

func isGenerated(f FileDiff) bool {
	// Check filename patterns
	base := filepath.Base(f.Filename)
	for _, pattern := range generatedPatterns {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}

	// Check file content for generation markers (first 5 lines of diff)
	lines := strings.Split(f.Raw, "\n")
	limit := 10
	if len(lines) < limit {
		limit = len(lines)
	}
	header := strings.Join(lines[:limit], "\n")
	for _, marker := range generatedMarkers {
		if strings.Contains(header, marker) {
			return true
		}
	}

	return false
}

func isWhitespaceOnly(f FileDiff) bool {
	if f.Additions == 0 && f.Deletions == 0 {
		return true
	}

	for _, line := range strings.Split(f.Raw, "\n") {
		if len(line) == 0 {
			continue
		}
		if line[0] == '+' && !strings.HasPrefix(line, "+++") {
			content := strings.TrimSpace(line[1:])
			if content != "" {
				// Has non-whitespace addition
				return false
			}
		}
		if line[0] == '-' && !strings.HasPrefix(line, "---") {
			content := strings.TrimSpace(line[1:])
			if content != "" {
				// Has non-whitespace deletion
				return false
			}
		}
	}

	return true
}

func matchesAny(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
		// Also try matching full path for patterns like "dist/*"
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	}
	return false
}
