package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/sidpremkumar/bcommit/internal/config"
	"github.com/sidpremkumar/bcommit/internal/contextstore"
	"github.com/sidpremkumar/bcommit/internal/diff"
	"github.com/sidpremkumar/bcommit/internal/gh"
	"github.com/sidpremkumar/bcommit/internal/git"
	"github.com/sidpremkumar/bcommit/internal/llm"
	"github.com/sidpremkumar/bcommit/internal/secrets"
	"github.com/sidpremkumar/bcommit/internal/ui"
)

var (
	flagPRBase    string
	flagPRPrint   bool
	flagPRDryRun  bool
	flagPRModel   string
	flagPRHint    string
	flagPRDraft   bool
	flagPRVerbose bool
)

var prCmd = &cobra.Command{
	Use:           "pr",
	Short:         "Generate a pull request title and body and open it with gh",
	Long:          "bcommit pr diffs the current branch against its base branch, gathers the commits, folds in any per-repo context, generates a PR title and description with a local LLM, and creates the PR via the GitHub CLI.",
	RunE:          runPR,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	prCmd.Flags().StringVar(&flagPRBase, "base", "", "Base branch to target (default: auto-detected)")
	prCmd.Flags().BoolVarP(&flagPRPrint, "print", "p", false, "Print the title and body only (no PR created)")
	prCmd.Flags().BoolVar(&flagPRDryRun, "dry-run", false, "Do everything except creating the PR")
	prCmd.Flags().StringVarP(&flagPRModel, "model", "m", "", "Override model (e.g., qwen2.5-coder:7b)")
	prCmd.Flags().StringVar(&flagPRHint, "hint", "", "Provide additional context for the PR description")
	prCmd.Flags().BoolVar(&flagPRDraft, "draft", false, "Create the PR as a draft")
	prCmd.Flags().BoolVarP(&flagPRVerbose, "verbose", "v", false, "Show tier selection, token counts")
	rootCmd.AddCommand(prCmd)
}

func runPR(cmd *cobra.Command, args []string) error {
	cfg := config.Load()

	model := cfg.Model
	if flagPRModel != "" {
		model = flagPRModel
	}

	creating := !flagPRPrint && !flagPRDryRun

	// Preflight gh before doing any LLM work, so we fail fast with clear guidance.
	if creating {
		if !gh.Available() {
			return fmt.Errorf("gh (GitHub CLI) is not installed — get it from https://cli.github.com")
		}
		if err := gh.Authenticated(); err != nil {
			return err
		}
	}

	// Resolve head and base branches.
	head, err := git.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("not a git repository or git is not installed")
	}

	base := flagPRBase
	if base == "" {
		base = cfg.DefaultBase
	}
	if base == "" {
		base, err = git.DetectBaseBranch()
		if err != nil {
			return err
		}
	}
	if base == head {
		return fmt.Errorf("current branch (%s) is the base branch — switch to a feature branch first", head)
	}

	// Gather commits and diff for the branch.
	commits, err := git.GetCommitsBetweenBranches(base, head)
	if err != nil {
		return err
	}
	if len(commits) == 0 {
		return fmt.Errorf("no commits between %s and %s — nothing to open a PR for", base, head)
	}

	rawDiff, err := git.GetDiffBetweenBranches(base, head, 3)
	if err != nil {
		return err
	}
	diffStat, err := git.GetDiffStatBetweenBranches(base, head)
	if err != nil {
		return err
	}

	// Scan for secrets in the branch diff.
	if findings := secrets.ScanDiff(rawDiff); len(findings) > 0 {
		ui.PrintError(secrets.FormatWarnings(findings))
		if creating {
			if !ui.PromptYesNo("Continue anyway?") {
				return fmt.Errorf("aborted: potential secrets detected in branch changes")
			}
		}
	}

	// Load per-repo custom context (tolerate a missing remote).
	var customContext string
	if remote, rErr := git.GetRemoteURL("origin"); rErr == nil {
		key := contextstore.RepoKey(remote)
		if ctxText, cErr := contextstore.Load(key); cErr == nil {
			customContext = ctxText
		}
	}

	ui.PrintStatus(fmt.Sprintf("Analyzing %s...%s (%d commit(s))\n%s", base, head, len(commits), diffStat))

	// Create LLM client.
	client, err := llm.NewClient(model, 8192, 0.3)
	if err != nil {
		return err
	}
	defer client.Shutdown()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		client.Shutdown()
		os.Exit(130)
	}()

	if err := client.EnsureReady(context.Background()); err != nil {
		return err
	}

	// Process the diff through the tier pipeline.
	procCfg := diff.ProcessorConfig{
		Tier1Max: diff.DefaultTier1Max,
		Tier2Max: diff.DefaultTier2Max,
		Tier3Max: diff.DefaultTier3Max,
		Verbose:  flagPRVerbose,
	}
	result, err := diff.Process(rawDiff, procCfg, client)
	if err != nil {
		return fmt.Errorf("diff processing failed: %w", err)
	}
	if flagPRVerbose {
		fmt.Printf("  Estimated tokens: %d, Tier: %d\n", result.Tokens, result.Tier)
	}
	diffLabel := diffLabelForTier(result.Tier)

	author := git.GetUserName()

	ui.PrintStatus("Generating PR description...")
	title, body, err := client.GeneratePRDescription(commits, result.Content, diffStat, customContext, flagPRHint, diffLabel, author)
	if err != nil {
		return err
	}

	// Print-only mode.
	if flagPRPrint {
		fmt.Printf("%s\n\n%s\n", title, body)
		return nil
	}

	return prInteractiveLoop(title, body, base, head, client, result.Content, diffStat, commits, customContext, diffLabel, author, cfg)
}

func prInteractiveLoop(title, body, base, head string, client *llm.Client, diffContent, diffStat string, commits []string, customContext, diffLabel, author string, cfg config.Config) error {
	for {
		ui.PrintPR(title, body)

		switch ui.PromptAction() {
		case ui.ActionAccept:
			return createPR(title, body, base, head, cfg)

		case ui.ActionEdit:
			newTitle, newBody, err := ui.EditPR(title, body)
			if err != nil {
				ui.PrintWarning(fmt.Sprintf("Edit failed: %v", err))
				continue
			}
			title, body = newTitle, newBody

		case ui.ActionRegenerate:
			ui.PrintStatus("Regenerating PR description...")
			newTitle, newBody, err := client.GeneratePRDescription(commits, diffContent, diffStat, customContext, flagPRHint, diffLabel, author)
			if err != nil {
				ui.PrintWarning(fmt.Sprintf("Regeneration failed: %v", err))
				continue
			}
			title, body = newTitle, newBody

		case ui.ActionQuit:
			fmt.Println("Aborted.")
			return nil
		}
	}
}

// createPR pushes the branch if needed, then opens the PR (or, in dry-run mode,
// prints what it would do).
func createPR(title, body, base, head string, cfg config.Config) error {
	// Ensure the branch is on the remote so gh can open a PR.
	hasUpstream, err := git.HasUpstream()
	if err != nil {
		return err
	}

	if flagPRDryRun {
		if !hasUpstream {
			fmt.Printf("[dry-run] would push: git push -u origin HEAD\n")
		}
		fmt.Printf("[dry-run] would run: gh pr create --base %s --title %q --body <body>%s\n", base, title, draftSuffix())
		return nil
	}

	if !hasUpstream {
		ui.PrintStatus("Pushing branch to origin...")
		if err := git.PushCurrentBranch(); err != nil {
			return err
		}
	}

	url, err := gh.CreatePR(gh.CreatePROptions{
		Base:      base,
		Title:     title,
		Body:      body,
		Draft:     flagPRDraft,
		Reviewers: parseReviewers(cfg.PRReviewers),
	})
	if err != nil {
		var exists *gh.ErrPRExists
		if errors.As(err, &exists) {
			ui.PrintWarning("A pull request already exists for this branch.")
			if existingURL, vErr := gh.ViewPRURL(); vErr == nil && existingURL != "" {
				ui.PrintStatus(fmt.Sprintf("Existing PR: %s", existingURL))
			}
			return nil
		}
		return err
	}

	ui.PrintSuccess(fmt.Sprintf("Created PR: %s", url))
	return nil
}

func draftSuffix() string {
	if flagPRDraft {
		return " --draft"
	}
	return ""
}

func parseReviewers(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for _, r := range strings.Split(s, ",") {
		if r = strings.TrimSpace(r); r != "" {
			out = append(out, r)
		}
	}
	return out
}

// diffLabelForTier maps a processing tier to a human label describing how the
// processed diff content should be read by the LLM.
func diffLabelForTier(tier diff.Tier) string {
	switch tier {
	case diff.Tier3Large:
		return "Per-file change summaries"
	case diff.Tier4XLarge:
		return "File-level change list"
	default:
		return "Full diff"
	}
}
