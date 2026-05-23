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
	ActionAccept     Action = iota
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

// EditMessage opens the user's editor with the message pre-filled and returns the edited result.
func EditMessage(msg string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}

	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, "bcommit-msg.txt")
	if err := os.WriteFile(tmpFile, []byte(msg), 0644); err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile)

	cmd := exec.Command(editor, tmpFile)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor failed: %w", err)
	}

	edited, err := os.ReadFile(tmpFile)
	if err != nil {
		return "", fmt.Errorf("failed to read edited file: %w", err)
	}

	result := strings.TrimSpace(string(edited))
	if result == "" {
		return "", fmt.Errorf("commit message is empty after editing")
	}

	return result, nil
}
