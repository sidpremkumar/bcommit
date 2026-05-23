package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/sidpremkumar/bcommit/internal/config"
	"github.com/sidpremkumar/bcommit/internal/diff"
	"github.com/sidpremkumar/bcommit/internal/git"
	"github.com/sidpremkumar/bcommit/internal/llm"
	"github.com/sidpremkumar/bcommit/internal/secrets"
	"github.com/sidpremkumar/bcommit/internal/ui"
)

var (
	flagCommit  bool
	flagPrint   bool
	flagModel   string
	flagType    string
	flagHint    string
	flagVerbose bool
	flagBranch  bool
)

// rootCmd is the main bcommit command.
var rootCmd = &cobra.Command{
	Use:           "bcommit",
	Short:         "Generate git commit messages with a local LLM",
	Long:          "bcommit analyzes your staged changes and generates conventional commit messages using a local LLM via Ollama.",
	RunE:          run,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.Flags().BoolVarP(&flagCommit, "commit", "c", false, "Auto-commit with the generated message")
	rootCmd.Flags().BoolVarP(&flagPrint, "print", "p", false, "Print the message only (no interactive prompt)")
	rootCmd.Flags().StringVarP(&flagModel, "model", "m", "", "Override model (e.g., qwen2.5-coder:7b)")
	rootCmd.Flags().StringVarP(&flagType, "type", "t", "", "Force commit type (feat, fix, refactor, etc.)")
	rootCmd.Flags().StringVar(&flagHint, "hint", "", "Provide additional context for the commit message")
	rootCmd.Flags().BoolVarP(&flagVerbose, "verbose", "v", false, "Show tier selection, token counts, timing")
	rootCmd.Flags().BoolVarP(&flagBranch, "branch", "b", false, "Generate a branch name, create it, and commit")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func run(cmd *cobra.Command, args []string) error {
	// Load config
	cfg := config.Load()

	// Apply CLI flag overrides
	model := cfg.Model
	if flagModel != "" {
		model = flagModel
	}

	// Check for staged changes
	hasStagedChanges, err := git.HasStagedChanges()
	if err != nil {
		return fmt.Errorf("not a git repository or git is not installed")
	}
	if !hasStagedChanges {
		return fmt.Errorf("no staged changes. Use 'git add' to stage files first")
	}

	// Get diff and stat
	rawDiff, err := git.GetStagedDiff(3)
	if err != nil {
		return err
	}
	diffStat, err := git.GetDiffStat()
	if err != nil {
		return err
	}

	// Scan for secrets in staged changes
	if findings := secrets.ScanDiff(rawDiff); len(findings) > 0 {
		ui.PrintError(secrets.FormatWarnings(findings))
		if !flagCommit && !flagPrint {
			if !ui.PromptYesNo("Continue anyway?") {
				return fmt.Errorf("aborted: potential secrets detected in staged changes")
			}
		}
	}

	ui.PrintStatus(fmt.Sprintf("Analyzing staged changes...\n%s", diffStat))

	// Create LLM client (hardcoded defaults for advanced settings)
	client, err := llm.NewClient(model, 8192, 0.3)
	if err != nil {
		return err
	}
	defer client.Shutdown()

	// Handle signals so Shutdown runs even on Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		client.Shutdown()
		os.Exit(130)
	}()

	// Ensure Ollama is installed, server is running, and model is available
	if err := client.EnsureReady(context.Background()); err != nil {
		return err
	}

	// Process the diff through the tier pipeline
	procCfg := diff.ProcessorConfig{
		Tier1Max: diff.DefaultTier1Max,
		Tier2Max: diff.DefaultTier2Max,
		Tier3Max: diff.DefaultTier3Max,
		Verbose:  flagVerbose,
	}

	result, err := diff.Process(rawDiff, procCfg, client)
	if err != nil {
		return fmt.Errorf("diff processing failed: %w", err)
	}

	if flagVerbose {
		fmt.Printf("  Estimated tokens: %d, Tier: %d\n", result.Tokens, result.Tier)
	}

	// Branch generation mode
	if flagBranch {
		ui.PrintStatus("Generating branch name...")
		branchName, err := client.GenerateBranchName(result.Content, diffStat, flagHint)
		if err != nil {
			return err
		}

		// Apply prefix if configured
		if cfg.BranchPrefix != "" {
			branchName = cfg.BranchPrefix + "/" + branchName
		}

		// Print-only mode: just show the branch name
		if flagPrint {
			fmt.Println(branchName)
			return nil
		}

		// Auto-commit mode: skip interactive prompt for branch name
		if flagCommit || cfg.AutoCommit {
			branchName, err = ensureUniqueBranch(branchName)
			if err != nil {
				return err
			}
			if err := git.CreateAndCheckoutBranch(branchName); err != nil {
				return err
			}
			ui.PrintSuccess(fmt.Sprintf("Switched to new branch: %s", branchName))
		} else {
			// Interactive mode for branch name
			branchName, err = branchInteractiveLoop(branchName, client, result.Content, diffStat, cfg)
			if err != nil {
				return err
			}
		}
	}

	// Generate commit message
	ui.PrintStatus("Generating commit message...")
	message, err := client.GenerateCommitMessage(result.Content, diffStat, flagHint, flagType)
	if err != nil {
		return err
	}

	// Auto-commit mode
	if flagCommit || cfg.AutoCommit {
		ui.PrintMessage(message)
		if err := git.Commit(message); err != nil {
			return err
		}
		ui.PrintSuccess(fmt.Sprintf("Committed: %s", firstLine(message)))
		return nil
	}

	// Print-only mode
	if flagPrint {
		fmt.Println(message)
		return nil
	}

	// Interactive mode (default)
	return interactiveLoop(message, client, result.Content, diffStat, cfg)
}

func interactiveLoop(message string, client *llm.Client, diffContent, diffStat string, cfg config.Config) error {
	for {
		ui.PrintMessage(message)

		action := ui.PromptAction()
		switch action {
		case ui.ActionAccept:
			if err := git.Commit(message); err != nil {
				return err
			}
			ui.PrintSuccess(fmt.Sprintf("Committed: %s", firstLine(message)))
			return nil

		case ui.ActionEdit:
			edited, err := ui.EditMessage(message)
			if err != nil {
				ui.PrintWarning(fmt.Sprintf("Edit failed: %v", err))
				continue
			}
			message = edited
			// Loop back to show the edited message and prompt again

		case ui.ActionRegenerate:
			ui.PrintStatus("Regenerating...")
			newMsg, err := client.GenerateCommitMessage(diffContent, diffStat, flagHint, flagType)
			if err != nil {
				ui.PrintWarning(fmt.Sprintf("Regeneration failed: %v", err))
				continue
			}
			message = newMsg

		case ui.ActionQuit:
			fmt.Println("Aborted.")
			return nil
		}
	}
}

func branchInteractiveLoop(branchName string, client *llm.Client, diffContent, diffStat string, cfg config.Config) (string, error) {
	for {
		ui.PrintBranchName(branchName)

		action := ui.PromptAction()
		switch action {
		case ui.ActionAccept:
			branchName, err := ensureUniqueBranch(branchName)
			if err != nil {
				return "", err
			}
			if err := git.CreateAndCheckoutBranch(branchName); err != nil {
				return "", err
			}
			ui.PrintSuccess(fmt.Sprintf("Switched to new branch: %s", branchName))
			return branchName, nil

		case ui.ActionEdit:
			edited, err := ui.EditMessage(branchName)
			if err != nil {
				ui.PrintWarning(fmt.Sprintf("Edit failed: %v", err))
				continue
			}
			branchName = edited

		case ui.ActionRegenerate:
			ui.PrintStatus("Regenerating branch name...")
			newName, err := client.GenerateBranchName(diffContent, diffStat, flagHint)
			if err != nil {
				ui.PrintWarning(fmt.Sprintf("Regeneration failed: %v", err))
				continue
			}
			if cfg.BranchPrefix != "" {
				newName = cfg.BranchPrefix + "/" + newName
			}
			branchName = newName

		case ui.ActionQuit:
			fmt.Println("Aborted.")
			return "", fmt.Errorf("aborted")
		}
	}
}

func ensureUniqueBranch(name string) (string, error) {
	exists, err := git.BranchExists(name)
	if err != nil {
		return "", err
	}
	if !exists {
		return name, nil
	}

	for i := 2; i <= 99; i++ {
		candidate := fmt.Sprintf("%s-%d", name, i)
		exists, err := git.BranchExists(candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not find a unique branch name for %q", name)
}

func firstLine(s string) string {
	for i, c := range s {
		if c == '\n' {
			return s[:i]
		}
	}
	return s
}
