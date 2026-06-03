package cli

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/sidpremkumar/bcommit/internal/config"
)

var configCmd = &cobra.Command{
	Use:           "config",
	Short:         "View or update bcommit settings",
	Long:          "View current configuration or use 'bcommit config set KEY VALUE' to update a setting.",
	RunE:          runConfig,
	SilenceUsage:  true,
	SilenceErrors: true,
}

var configSetCmd = &cobra.Command{
	Use:           "set KEY VALUE",
	Short:         "Set a config value",
	Example:       "  bcommit config set auto_commit true\n  bcommit config set model qwen2.5-coder:7b",
	Args:          cobra.ExactArgs(2),
	RunE:          runConfigSet,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	configCmd.AddCommand(configSetCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfig(cmd *cobra.Command, args []string) error {
	cfg := config.Load()

	dim := color.New(color.Faint)
	bold := color.New(color.Bold)

	fmt.Println()
	fmt.Printf("  %-16s %s\n", bold.Sprint("auto_commit:"), formatBool(cfg.AutoCommit))
	fmt.Printf("  %-16s %s\n", bold.Sprint("model:"), cfg.Model)
	if cfg.BranchPrefix != "" {
		fmt.Printf("  %-16s %s\n", bold.Sprint("branch_prefix:"), cfg.BranchPrefix)
	}
	if cfg.DefaultBase != "" {
		fmt.Printf("  %-16s %s\n", bold.Sprint("default_base:"), cfg.DefaultBase)
	}
	if cfg.PRReviewers != "" {
		fmt.Printf("  %-16s %s\n", bold.Sprint("pr_reviewers:"), cfg.PRReviewers)
	}
	fmt.Println()
	dim.Printf("  Config file: %s\n", config.ConfigPath())
	fmt.Println()

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key, value := args[0], args[1]

	if err := config.Set(key, value); err != nil {
		return err
	}

	green := color.New(color.FgGreen, color.Bold)
	green.Printf("✓ %s = %s\n", key, value)
	return nil
}

func formatBool(b bool) string {
	if b {
		return color.GreenString("true")
	}
	return color.RedString("false")
}
