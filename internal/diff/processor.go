package diff

import (
	"fmt"
	"strings"
)

// Tier represents the processing tier for a diff.
type Tier int

const (
	Tier1Small  Tier = 1
	Tier2Medium Tier = 2
	Tier3Large  Tier = 3
	Tier4XLarge Tier = 4
)

// Default thresholds (in estimated tokens).
const (
	DefaultTier1Max = 4000
	DefaultTier2Max = 12000
	// DefaultTier3Max is the upper bound for per-file summarization.
	// Beyond this we switch to Tier 4 (stat + file list only).
	DefaultTier3Max = 40000

	// MaxFileTokensForSummarization caps the size of a single file's diff that
	// will be sent to the LLM for summarization. Huge single-file diffs (e.g.,
	// regenerated artifacts that slipped past filters) tend to make small local
	// models stall, so we skip the LLM call and fall back to a stat summary.
	MaxFileTokensForSummarization = 3000

	// MaxFilesForTier3 caps how many per-file LLM calls Tier 3 will make.
	// Beyond this we fall through to Tier 4 to avoid taking minutes per commit.
	MaxFilesForTier3 = 25
)

// Summarizer is the interface for LLM-based file summarization (Tier 3).
type Summarizer interface {
	SummarizeFileDiff(filename, fileDiff string) (string, error)
}

// ProcessorConfig holds configuration for the diff processor.
type ProcessorConfig struct {
	Tier1Max        int
	Tier2Max        int
	Tier3Max        int
	ExcludePatterns []string
	Verbose         bool
}

// DefaultProcessorConfig returns the default processor configuration.
func DefaultProcessorConfig() ProcessorConfig {
	return ProcessorConfig{
		Tier1Max: DefaultTier1Max,
		Tier2Max: DefaultTier2Max,
		Tier3Max: DefaultTier3Max,
	}
}

// ProcessResult contains the processed diff content and metadata.
type ProcessResult struct {
	Content string // The diff content or summaries to send to the LLM
	Tier    Tier
	Notes   []string // Filter notes (e.g., "[lock file updated: ...]")
	Tokens  int      // Estimated token count of the raw diff
}

// Process takes a raw diff and returns processed content suitable for the LLM.
// For Tier 3, it requires a Summarizer to do per-file summarization.
func Process(rawDiff string, cfg ProcessorConfig, summarizer Summarizer) (*ProcessResult, error) {
	tokens := EstimateTokens(rawDiff)

	// Tier 1: Small diff — pass through
	if tokens <= cfg.Tier1Max {
		if cfg.Verbose {
			fmt.Printf("  Tier 1 (small): %d estimated tokens, passing verbatim\n", tokens)
		}
		return &ProcessResult{
			Content: rawDiff,
			Tier:    Tier1Small,
			Tokens:  tokens,
		}, nil
	}

	// Parse the diff for filtering
	files := Parse(rawDiff)
	if len(files) == 0 {
		return &ProcessResult{
			Content: rawDiff,
			Tier:    Tier1Small,
			Tokens:  tokens,
		}, nil
	}

	// Tier 2: Medium diff — filter noise
	filtered := Filter(files, cfg.ExcludePatterns)

	if len(filtered.Files) == 0 {
		// Everything was filtered — just send notes
		content := strings.Join(filtered.Notes, "\n")
		if cfg.Verbose {
			fmt.Printf("  Tier 2 (filtered): all files filtered, %d notes\n", len(filtered.Notes))
		}
		return &ProcessResult{
			Content: content,
			Tier:    Tier2Medium,
			Notes:   filtered.Notes,
			Tokens:  tokens,
		}, nil
	}

	reassembled := Reassemble(filtered.Files)
	filteredTokens := EstimateTokens(reassembled)

	// Check if filtering brought us down to Tier 1
	if filteredTokens <= cfg.Tier1Max {
		content := reassembled
		if len(filtered.Notes) > 0 {
			content = strings.Join(filtered.Notes, "\n") + "\n\n" + content
		}
		if cfg.Verbose {
			fmt.Printf("  Tier 2 (filtered→small): %d→%d estimated tokens\n", tokens, filteredTokens)
		}
		return &ProcessResult{
			Content: content,
			Tier:    Tier2Medium,
			Notes:   filtered.Notes,
			Tokens:  tokens,
		}, nil
	}

	// Check if filtered diff fits in Tier 2
	if filteredTokens <= cfg.Tier2Max {
		content := reassembled
		if len(filtered.Notes) > 0 {
			content = strings.Join(filtered.Notes, "\n") + "\n\n" + content
		}
		if cfg.Verbose {
			fmt.Printf("  Tier 2 (filtered): %d→%d estimated tokens\n", tokens, filteredTokens)
		}
		return &ProcessResult{
			Content: content,
			Tier:    Tier2Medium,
			Notes:   filtered.Notes,
			Tokens:  tokens,
		}, nil
	}

	// Tier 4: Extreme diff — too big or too many files for per-file summarization.
	// Skip the LLM-per-file fan-out and send a structured file list + stat instead.
	tier3Max := cfg.Tier3Max
	if tier3Max == 0 {
		tier3Max = DefaultTier3Max
	}
	if summarizer == nil || filteredTokens > tier3Max || len(filtered.Files) > MaxFilesForTier3 {
		if cfg.Verbose {
			fmt.Printf("  Tier 4 (xlarge): %d estimated tokens, %d files — skipping per-file summarization\n", tokens, len(filtered.Files))
		}
		content := buildFileListSummary(filtered.Files, filtered.Notes)
		return &ProcessResult{
			Content: content,
			Tier:    Tier4XLarge,
			Notes:   filtered.Notes,
			Tokens:  tokens,
		}, nil
	}

	// Tier 3: Large diff — per-file summarization
	if cfg.Verbose {
		fmt.Printf("  Tier 3 (large): %d estimated tokens, summarizing %d files\n", tokens, len(filtered.Files))
	}

	// Summarize each file. Skip the LLM call for individual files that are too
	// large — those hang small local models — and use a stat fallback instead.
	var summaries []string
	for _, f := range filtered.Files {
		fileTokens := EstimateTokens(f.Raw)
		var summary string
		if fileTokens > MaxFileTokensForSummarization {
			if cfg.Verbose {
				fmt.Printf("    skipping summarization for %s (%d tokens > %d cap)\n", f.Filename, fileTokens, MaxFileTokensForSummarization)
			}
			summary = fmt.Sprintf("+%d -%d lines changed (file too large to summarize)", f.Additions, f.Deletions)
		} else {
			s, err := summarizer.SummarizeFileDiff(f.Filename, f.Raw)
			if err != nil {
				summary = fmt.Sprintf("+%d -%d lines changed", f.Additions, f.Deletions)
			} else {
				summary = s
			}
		}
		summaries = append(summaries, fmt.Sprintf("- %s: %s", f.Filename, summary))
	}

	// Combine notes and summaries
	var parts []string
	if len(filtered.Notes) > 0 {
		parts = append(parts, filtered.Notes...)
	}
	parts = append(parts, summaries...)
	content := strings.Join(parts, "\n")

	return &ProcessResult{
		Content: content,
		Tier:    Tier3Large,
		Notes:   filtered.Notes,
		Tokens:  tokens,
	}, nil
}

// buildFileListSummary produces a compact, LLM-friendly summary for Tier 4:
// notes + files grouped by change kind + a top-N list by line count.
func buildFileListSummary(files []FileDiff, notes []string) string {
	var added, deleted, renamed, modified []FileDiff
	for _, f := range files {
		switch {
		case f.IsNew:
			added = append(added, f)
		case f.IsDeleted:
			deleted = append(deleted, f)
		case f.OldFilename != "":
			renamed = append(renamed, f)
		default:
			modified = append(modified, f)
		}
	}

	var b strings.Builder
	b.WriteString("[Large diff — sending file list only, no per-file content]\n\n")

	if len(notes) > 0 {
		for _, n := range notes {
			b.WriteString(n)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	writeGroup := func(label string, group []FileDiff) {
		if len(group) == 0 {
			return
		}
		fmt.Fprintf(&b, "%s (%d):\n", label, len(group))
		limit := 30
		if len(group) < limit {
			limit = len(group)
		}
		for _, f := range group[:limit] {
			if f.OldFilename != "" {
				fmt.Fprintf(&b, "- %s → %s (+%d -%d)\n", f.OldFilename, f.Filename, f.Additions, f.Deletions)
			} else {
				fmt.Fprintf(&b, "- %s (+%d -%d)\n", f.Filename, f.Additions, f.Deletions)
			}
		}
		if len(group) > limit {
			fmt.Fprintf(&b, "- ...and %d more\n", len(group)-limit)
		}
		b.WriteString("\n")
	}

	writeGroup("Added", added)
	writeGroup("Deleted", deleted)
	writeGroup("Renamed", renamed)
	writeGroup("Modified", modified)

	return strings.TrimSpace(b.String())
}
