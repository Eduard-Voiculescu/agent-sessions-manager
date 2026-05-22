// Package agent defines the contract that any coding agent (claude, codex,
// grok, antigravity, …) implements so asm can manage its sessions.
package agent

import (
	"time"

	"github.com/eduard-voiculescu/agent-sessions-manager/internal/worktree"
)

type State int

const (
	StateIdle State = iota
	StateActive
	StateDisconnected
)

func (s State) String() string {
	switch s {
	case StateActive:
		return "active"
	case StateDisconnected:
		return "disconnected"
	default:
		return "idle"
	}
}

type Session struct {
	ID             string    // agent-defined session id
	Agent          string    // "claude", "codex", ...
	WorktreePath   string    // empty for disconnected
	Branch         string    // empty for disconnected
	State          State
	LastActive     time.Time
	MsgCount       int
	TranscriptPath string    // absolute path to the jsonl
	PID            int       // populated when State == StateActive
}

// Agent is the integration point for a coding-agent CLI.
type Agent interface {
	Name() string

	// Discover returns one Session per (worktree, most-recent transcript) pair,
	// plus orphan transcripts (former worktrees of repo) as disconnected sessions.
	Discover(repo worktree.Repo, worktrees []worktree.Worktree) ([]Session, error)

	// IsActiveIn reports whether the agent is currently running in worktreePath.
	// Returns (active, pid, err). M1 may stub this to (false, 0, nil).
	IsActiveIn(worktreePath string) (bool, int, error)

	// NewCommand builds the argv for starting a fresh session in worktreePath.
	NewCommand(worktreePath string) (bin string, args []string)

	// ResumeCommand builds the argv for resuming an existing session.
	ResumeCommand(s Session) (bin string, args []string)
}
