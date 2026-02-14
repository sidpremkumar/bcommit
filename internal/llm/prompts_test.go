package llm

import (
	"strings"
	"testing"
)

func TestBuildUserPrompt(t *testing.T) {
	prompt := BuildUserPrompt("diff content", "3 files changed", "", "")

	if !strings.Contains(prompt, "diff content") {
		t.Error("prompt should contain diff content")
	}
	if !strings.Contains(prompt, "3 files changed") {
		t.Error("prompt should contain diff stat")
	}
	if strings.Contains(prompt, "Additional context") {
		t.Error("prompt should not contain hint section when hint is empty")
	}
}

func TestBuildUserPromptWithHint(t *testing.T) {
	prompt := BuildUserPrompt("diff", "stat", "fixing race condition", "")

	if !strings.Contains(prompt, "fixing race condition") {
		t.Error("prompt should contain the hint")
	}
	if !strings.Contains(prompt, "Additional context") {
		t.Error("prompt should contain hint label")
	}
}

func TestBuildUserPromptWithForceType(t *testing.T) {
	prompt := BuildUserPrompt("diff", "stat", "", "fix")

	if !strings.Contains(prompt, "MUST be: fix") {
		t.Error("prompt should contain forced type instruction")
	}
}

func TestBuildAggregatePrompt(t *testing.T) {
	summaries := []FileSummary{
		{Filename: "main.go", Summary: "added entry point"},
		{Filename: "lib.go", Summary: "added helper functions"},
	}

	prompt := BuildAggregatePrompt(summaries, "2 files changed", "", "")

	if !strings.Contains(prompt, "main.go: added entry point") {
		t.Error("prompt should contain file summaries")
	}
	if !strings.Contains(prompt, "2 files changed") {
		t.Error("prompt should contain diff stat")
	}
}
