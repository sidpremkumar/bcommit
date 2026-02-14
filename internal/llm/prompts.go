package llm

import (
	"fmt"
	"strings"
)

const SystemPrompt = `You are a git commit message generator. You analyze git diffs and produce commit messages following the Conventional Commits specification.

FORMAT RULES:
- First line: <type>(<scope>): <short summary>
- Type must be one of: feat, fix, refactor, docs, style, test, chore, perf, ci, build
- Scope is optional: only include it when ALL changes clearly belong to one module/area (e.g. auth, api, db, ui). If changes span multiple areas, OMIT the scope entirely.
- The first line must be a high-level summary in imperative mood, lowercase, no period, max 72 chars
- For breaking changes, add "!" after the type: feat!: or fix!:

BODY RULES:
- For small, focused changes: the first line alone is sufficient. Do NOT add a body.
- For larger changes touching multiple files or concepts: add a blank line after the first line, then bullet points starting with "- " describing specific changes.
- Bullets should add detail beyond the first line, not repeat it.

GUIDELINES:
- Focus on WHAT changed and WHY, not HOW
- Be specific: "add user authentication" not "update code"
- If multiple types of changes are present, choose the most significant one
- The first line should make sense on its own as a standalone summary

OUTPUT ONLY THE COMMIT MESSAGE. No explanations, no markdown fencing, no quotes.`

const SummarizeSystemPrompt = `You summarize code changes from git diffs.
Output only the summary, nothing else. Be concise: 1-2 sentences max.
Focus on what was changed and the likely purpose.
Do not describe the diff format itself.`

// BuildUserPrompt constructs the user prompt for commit message generation.
func BuildUserPrompt(diffContent, diffStat, hint, forceType string) string {
	var b strings.Builder

	b.WriteString("Generate a conventional commit message for the following staged changes.\n")

	if hint != "" {
		fmt.Fprintf(&b, "\nAdditional context from the developer: %s\n", hint)
	}

	if forceType != "" {
		fmt.Fprintf(&b, "\nThe commit type MUST be: %s\n", forceType)
	}

	if diffStat != "" {
		fmt.Fprintf(&b, "\nDiff statistics:\n%s\n", diffStat)
	}

	fmt.Fprintf(&b, "\n%s", diffContent)

	return b.String()
}

// BuildSummarizePrompt constructs the prompt for per-file summarization (Tier 3).
func BuildSummarizePrompt(filename, fileDiff string) string {
	return fmt.Sprintf("Summarize the code changes in this file diff in 1-2 concise sentences.\n\nFile: %s\n%s", filename, fileDiff)
}

// BuildAggregatePrompt constructs the prompt for aggregating per-file summaries into a commit message.
func BuildAggregatePrompt(summaries []FileSummary, diffStat, hint, forceType string) string {
	var b strings.Builder

	b.WriteString("Generate a conventional commit message based on these file change summaries.\n")

	if hint != "" {
		fmt.Fprintf(&b, "\nAdditional context from the developer: %s\n", hint)
	}

	if forceType != "" {
		fmt.Fprintf(&b, "\nThe commit type MUST be: %s\n", forceType)
	}

	if diffStat != "" {
		fmt.Fprintf(&b, "\nDiff statistics:\n%s\n", diffStat)
	}

	b.WriteString("\nFile change summaries:\n")
	for _, s := range summaries {
		fmt.Fprintf(&b, "- %s: %s\n", s.Filename, s.Summary)
	}

	return b.String()
}

// FileSummary holds a per-file summary for Tier 3 aggregation.
type FileSummary struct {
	Filename string
	Summary  string
}
