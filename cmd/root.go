package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/eduard-voiculescu/agent-sessions-manager/internal/agent/claude"
	"github.com/eduard-voiculescu/agent-sessions-manager/internal/app"
	"github.com/eduard-voiculescu/agent-sessions-manager/internal/launcher"
	"github.com/eduard-voiculescu/agent-sessions-manager/internal/worktree"
)

var rootCmd = &cobra.Command{
	Use:           "asm",
	Short:         "Agent Sessions Manager — TUI for managing coding-agent sessions tied to git worktrees",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := worktree.FindRepo(".")
		if err != nil {
			return fmt.Errorf("asm must be run inside a git repository: %w", err)
		}

		agent := claude.New()
		model := app.New(repo, agent)

		p := tea.NewProgram(model, tea.WithAltScreen())
		final, err := p.Run()
		if err != nil {
			return err
		}

		if fm, ok := final.(app.Model); ok && fm.PendingExec != nil {
			req := fm.PendingExec
			return launcher.Exec(req.Cwd, req.Bin, req.Args)
		}
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
