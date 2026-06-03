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

const BranchNameSystemPrompt = `You generate short, descriptive git branch names from code diffs.

RULES:
- Output ONLY the branch name, nothing else
- Use kebab-case: lowercase letters, numbers, and hyphens only
- Keep it short: 2-5 words, max 50 characters
- Be descriptive of the change: e.g. add-user-auth, fix-memory-leak, refactor-db-queries
- No special characters beyond hyphens
- No leading or trailing hyphens
- Do NOT include a type prefix like feat/ or fix/ — just the descriptive name

OUTPUT ONLY THE BRANCH NAME. No explanations, no quotes, no markdown.`

// BuildBranchNamePrompt constructs the user prompt for branch name generation.
func BuildBranchNamePrompt(diffContent, diffStat, hint string) string {
	var b strings.Builder

	b.WriteString("Generate a short, descriptive branch name for the following staged changes.\n")

	if hint != "" {
		fmt.Fprintf(&b, "\nAdditional context from the developer: %s\n", hint)
	}

	if diffStat != "" {
		fmt.Fprintf(&b, "\nDiff statistics:\n%s\n", diffStat)
	}

	fmt.Fprintf(&b, "\n%s", diffContent)

	return b.String()
}

// BuildPRBodySystemPrompt builds the system prompt for the PR body. When author
// is non-empty the model is framed as that person writing up their own work, so
// the description reads in a natural first-person voice instead of generic
// AI-template prose.
func BuildPRBodySystemPrompt(author string) string {
	who := "a developer"
	if author != "" {
		who = author
	}

	return fmt.Sprintf(`You are %s, writing the description for your own pull request. Write the way a real developer types a PR description to teammates — plain, direct, first person.

HOW TO WRITE IT:
- Open with 1-3 sentences saying what you changed and why. NO "Summary" or "Summary of Changes" heading. NO "This PR...". Just say it.
- Add a short "## Changes" bullet list ONLY when there are several distinct changes worth separating out. For a small, focused change, a sentence or two is the whole thing — don't pad it.
- Do NOT use numbered lists, bold sub-headers, or nested structure. Plain prose, plus at most one flat bullet list.
- Keep it SHORT — a few sentences, or at most ~5 bullets. A reviewer should read it in 15 seconds.
- Describe changes at the behavior level, not the code level. Do NOT inventory the diff: no listing every function, constant, variable, or file you added/touched. Say what the change does, not which symbols it introduces.
- Never write empty or filler sections ("Notes: None", headings with nothing under them).
- Do NOT end with a wrap-up/marketing sentence like "These changes make X more flexible and user-friendly." Stop when you've said what changed.
- Mention testing or caveats only when they genuinely matter.

Base everything strictly on the commits and diff provided — never invent changes.
If repository conventions are provided, follow them; they win over these defaults.
Output only the description in markdown. No title line, no code fences, no preamble.

Here is the voice and length to match.

EXAMPLE 1 — small change:
Adds me to the PR review rotation so I get auto-assigned on new PRs. Just appends my name to PR_REVIEW_AUTHORS in the workflow file.

EXAMPLE 2 — larger change:
Reworks PR generation so the title and body come from two separate model calls instead of one. The old single-call approach kept leaking the body's first heading into the title, so this splits them: generate the body first, then derive the title from it.

## Changes
- Split GeneratePRDescription into generatePRBody + generatePRTitle.
- Pull the git user.name so the body reads in the author's voice.
- Drop the rigid Summary/Changes/Notes scaffold from the prompt.

Now write the description for the actual change below in that same voice.`, who)
}

const PRTitleSystemPrompt = `You write a single pull request title from a PR description.

RULES:
- Output ONLY the title: one line, nothing else.
- Imperative mood, under 70 characters, no trailing period.
- No "PR:" prefix, no conventional-commit type prefix (no "feat:"), no quotes, no markdown.
- Capture the main change plainly and specifically, the way a person would title it.`

// BuildPRBodyPrompt constructs the user prompt for PR body generation.
// diffLabel describes how to interpret the processed diff (e.g. "Full diff",
// "Per-file change summaries", "File-level change list"), keeping this package
// decoupled from the diff tier types.
func BuildPRBodyPrompt(commits []string, processedDiff, diffStat, customContext, hint, diffLabel string) string {
	var b strings.Builder

	b.WriteString("Write the description for a pull request covering the following changes.\n")

	if customContext != "" {
		fmt.Fprintf(&b, "\nRepository conventions and context (follow these closely):\n%s\n", customContext)
	}

	if hint != "" {
		fmt.Fprintf(&b, "\nAdditional context from the developer: %s\n", hint)
	}

	if len(commits) > 0 {
		b.WriteString("\nCommits in this branch (newest first):\n")
		for _, c := range commits {
			fmt.Fprintf(&b, "- %s\n", c)
		}
	}

	if diffStat != "" {
		fmt.Fprintf(&b, "\nDiff statistics:\n%s\n", diffStat)
	}

	if processedDiff != "" {
		label := diffLabel
		if label == "" {
			label = "Diff"
		}
		fmt.Fprintf(&b, "\n%s:\n%s", label, processedDiff)
	}

	return b.String()
}

// BuildPRTitlePrompt constructs the user prompt for deriving a title from the
// already-generated PR body (with commits as supporting context).
func BuildPRTitlePrompt(body string, commits []string) string {
	var b strings.Builder

	b.WriteString("Write a pull request title for the following change.\n")

	if len(commits) > 0 {
		b.WriteString("\nCommits in this branch (newest first):\n")
		for _, c := range commits {
			fmt.Fprintf(&b, "- %s\n", c)
		}
	}

	fmt.Fprintf(&b, "\nPR description:\n%s\n", body)

	return b.String()
}
