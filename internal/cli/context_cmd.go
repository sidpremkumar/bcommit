package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sidpremkumar/bcommit/internal/contextstore"
	"github.com/sidpremkumar/bcommit/internal/git"
	"github.com/sidpremkumar/bcommit/internal/ui"
)

var flagContextPath bool

var contextCmd = &cobra.Command{
	Use:           "context",
	Short:         "Edit per-repo context used for PR generation",
	Long:          "Opens this repository's custom context file in your editor. The context is fed to the LLM as high-priority guidance when generating pull request descriptions. Stored centrally (keyed by remote URL), not committed to the repo.",
	RunE:          runContext,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	contextCmd.Flags().BoolVar(&flagContextPath, "path", false, "Print the context file path instead of opening it")
	rootCmd.AddCommand(contextCmd)
}

func runContext(cmd *cobra.Command, args []string) error {
	remote, err := git.GetRemoteURL("origin")
	if err != nil {
		return fmt.Errorf("not in a git repository with an 'origin' remote")
	}
	key := contextstore.RepoKey(remote)

	path, err := contextstore.EnsurePath(key)
	if err != nil {
		return err
	}

	if flagContextPath {
		fmt.Println(path)
		return nil
	}

	if err := ui.LaunchEditor(path); err != nil {
		return err
	}

	ui.PrintSuccess(fmt.Sprintf("Saved context for %s", key))
	return nil
}
