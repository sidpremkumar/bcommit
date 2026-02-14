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
)

// Default thresholds (in estimated tokens).
const (
	DefaultTier1Max = 4000
	DefaultTier2Max = 12000
)

// Summarizer is the interface for LLM-based file summarization (Tier 3).
type Summarizer interface {
	SummarizeFileDiff(filename, fileDiff string) (string, error)
}

// ProcessorConfig holds configuration for the diff processor.
type ProcessorConfig struct {
	Tier1Max       int
	Tier2Max       int
	ExcludePatterns []string
	Verbose        bool
}

// DefaultProcessorConfig returns the default processor configuration.
func DefaultProcessorConfig() ProcessorConfig {
	return ProcessorConfig{
		Tier1Max: DefaultTier1Max,
		Tier2Max: DefaultTier2Max,
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

	// Tier 3: Large diff — per-file summarization
	if summarizer == nil {
		// No summarizer available, truncate with a note
		content := reassembled
		if len(filtered.Notes) > 0 {
			content = strings.Join(filtered.Notes, "\n") + "\n\n" + content
		}
		// Rough truncation to fit
		maxChars := cfg.Tier2Max * 3 // convert back from tokens
		if len(content) > maxChars {
			content = content[:maxChars] + "\n\n[diff truncated]"
		}
		return &ProcessResult{
			Content: content,
			Tier:    Tier3Large,
			Notes:   filtered.Notes,
			Tokens:  tokens,
		}, nil
	}

	if cfg.Verbose {
		fmt.Printf("  Tier 3 (large): %d estimated tokens, summarizing %d files\n", tokens, len(filtered.Files))
	}

	// Summarize each file
	var summaries []string
	for _, f := range filtered.Files {
		summary, err := summarizer.SummarizeFileDiff(f.Filename, f.Raw)
		if err != nil {
			// On error, use a fallback summary
			summary = fmt.Sprintf("+%d -%d lines changed", f.Additions, f.Deletions)
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
