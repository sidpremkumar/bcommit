package ui

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
)

var (
	red    = color.New(color.FgRed, color.Bold)
	green  = color.New(color.FgGreen, color.Bold)
	yellow = color.New(color.FgYellow, color.Bold)
	cyan   = color.New(color.FgCyan)
	dim    = color.New(color.Faint)
)

// PrintMessage displays a commit message with formatting.
func PrintMessage(msg string) {
	fmt.Println()
	lines := strings.Split(msg, "\n")
	for i, line := range lines {
		if i == 0 {
			green.Printf("  %s\n", line)
		} else {
			fmt.Printf("  %s\n", line)
		}
	}
	fmt.Println()
}

// PrintBranchName displays a generated branch name with formatting.
func PrintBranchName(name string) {
	fmt.Println()
	cyan.Printf("  Branch: %s\n", name)
	fmt.Println()
}

// PrintStatus prints a status line with a bullet.
func PrintStatus(msg string) {
	cyan.Printf("● %s\n", msg)
}

// PrintSuccess prints a success line with a checkmark.
func PrintSuccess(msg string) {
	green.Printf("✓ %s\n", msg)
}

// PrintWarning prints a warning message.
func PrintWarning(msg string) {
	yellow.Printf("⚠ %s\n", msg)
}

// PrintError prints an error message with a cross mark.
func PrintError(msg string) {
	red.Printf("✗ %s\n", msg)
}

// PromptYesNo asks the user a yes/no question. Default is yes (empty input = yes).
func PromptYesNo(question string) bool {
	cyan.Printf("● %s [Y/n]: ", question)

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	switch input {
	case "", "y", "yes":
		return true
	default:
		return false
	}
}

// PrintProgress prints a progress update on the same line (carriage return).
func PrintProgress(msg string) {
	fmt.Printf("\r● %s", msg)
}

// Action represents the user's choice in interactive mode.
type Action int

const (
	ActionAccept Action = iota
	ActionEdit
	ActionRegenerate
	ActionQuit
)

// PromptAction asks the user to choose an action in interactive mode.
func PromptAction() Action {
	dim.Print("[a]ccept  [e]dit  [r]egenerate  [q]uit: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	switch input {
	case "a", "accept":
		return ActionAccept
	case "e", "edit":
		return ActionEdit
	case "r", "regenerate":
		return ActionRegenerate
	case "q", "quit", "":
		return ActionQuit
	default:
		fmt.Println("Invalid choice. Use a/e/r/q.")
		return PromptAction()
	}
}

// resolveEditor returns the user's preferred editor ($EDITOR → $VISUAL → vi).
func resolveEditor() string {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}
	return editor
}

// LaunchEditor opens the user's editor on an existing file path, wiring stdio to
// the terminal, and waits for it to exit. Edits persist to the given path.
func LaunchEditor(path string) error {
	cmd := exec.Command(resolveEditor(), path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}
	return nil
}

// EditBuffer opens the user's editor on a temp file pre-filled with content and
// returns the edited result. filenameHint names the temp file (its suffix lets
// editors choose syntax highlighting). If allowEmpty is false, an empty result
// is an error.
func EditBuffer(content, filenameHint string, allowEmpty bool) (string, error) {
	tmpFile := filepath.Join(os.TempDir(), filenameHint)
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile)

	if err := LaunchEditor(tmpFile); err != nil {
		return "", err
	}

	edited, err := os.ReadFile(tmpFile)
	if err != nil {
		return "", fmt.Errorf("failed to read edited file: %w", err)
	}

	result := strings.TrimSpace(string(edited))
	if result == "" && !allowEmpty {
		return "", fmt.Errorf("content is empty after editing")
	}
	return result, nil
}

// EditMessage opens the user's editor with the message pre-filled and returns the edited result.
func EditMessage(msg string) (string, error) {
	return EditBuffer(msg, "bcommit-msg.txt", false)
}

// EditPR opens a single editor buffer containing the title on the first line, a
// blank line, then the body. It returns the parsed (title, body).
func EditPR(title, body string) (newTitle, newBody string, err error) {
	buf := title + "\n\n" + body + "\n"
	edited, err := EditBuffer(buf, "bcommit-pr.md", false)
	if err != nil {
		return "", "", err
	}

	lines := strings.Split(edited, "\n")
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) {
		return "", "", fmt.Errorf("PR title is empty after editing")
	}
	newTitle = strings.TrimSpace(lines[i])
	newBody = strings.TrimSpace(strings.Join(lines[i+1:], "\n"))
	return newTitle, newBody, nil
}

// PrintPR displays a PR title and body with formatting.
func PrintPR(title, body string) {
	fmt.Println()
	green.Printf("  %s\n", title)
	fmt.Println()
	for _, line := range strings.Split(body, "\n") {
		dim.Printf("  %s\n", line)
	}
	fmt.Println()
}
