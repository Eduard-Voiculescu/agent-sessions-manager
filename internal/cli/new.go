package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/eduard-voiculescu/agent-sessions-manager/internal/agent/claude"
	"github.com/eduard-voiculescu/agent-sessions-manager/internal/launcher"
	"github.com/eduard-voiculescu/agent-sessions-manager/internal/session"
	"github.com/eduard-voiculescu/agent-sessions-manager/internal/worktree"
)

var newCmd = &cobra.Command{
	Use:           "new <name>",
	Short:         "Create a new agent session: worktree + branch, then exec into the agent",
	Args:          cobra.ExactArgs(1),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := worktree.FindRepo(".")
		if err != nil {
			return fmt.Errorf("asm must be run inside a git repository: %w", err)
		}

		created, err := session.Create(repo, args[0])
		if err != nil {
			return err
		}

		a := claude.New()
		bin, argv := a.NewCommand(created.WorktreePath)
		return launcher.Exec(created.WorktreePath, bin, argv)
	},
}

func init() {
	rootCmd.AddCommand(newCmd)
}
