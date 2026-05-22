# Agent Sessions Manager (`asm`) — v1 Plan

A k9s/lazygit-inspired TUI for managing Claude Code sessions, each tied to a git worktree. `asm` owns the end-to-end lifecycle: create, resume, view, delete.

## Goals

- Single binary `asm`. Run inside a git repo. No config files in v1.
- Each "session" = one git worktree + one (or more) agent transcripts.
- States: `active` (agent process running in the worktree), `idle` (no process), `disconnected` (transcript with no matching worktree).
- Two entry points to creating a session — both share one code path:
  - `asm new <name>` (CLI shortcut)
  - `n` in the TUI (modal prompt)
- Both end by `exec`ing into the agent in the new worktree (lazygit-style: TUI vanishes, agent takes over the terminal).

## Non-goals (v1)

- Multi-repo / global view across `~/.claude/projects/`.
- Multi-agent delivery (designed for, not shipped — see below).
- Re-entering the TUI after the agent exits.
- Attaching to an already-running session (we block double-resume instead).

## Future-friendly: agent abstraction

v1 only supports Claude Code, but the code is structured so adding codex / grok / antigravity later is a new package, not a TUI rewrite. The TUI talks to an `Agent` interface; `agent/claude` is the only implementation in v1.

```go
// internal/agent/agent.go
type State int
const ( StateIdle State = iota; StateActive; StateDisconnected )

type Session struct {
    ID          string    // agent-defined session id
    WorktreePath string   // empty for disconnected
    Branch      string
    State       State
    LastActive  time.Time
    MsgCount    int
    Agent       string    // "claude", future: "codex", "grok", ...
}

type Agent interface {
    Name() string

    // Discover transcripts/sessions visible on disk for the given worktrees.
    // Orphan transcripts (no matching worktree) come back as disconnected.
    Discover(worktrees []worktree.Worktree) ([]Session, error)

    // Is the agent currently running in this worktree?
    IsActiveIn(worktreePath string) (bool, int, error)  // (active, pid, err)

    // Build the exec-out command. Launcher does syscall.Exec.
    NewCommand(worktreePath string) (bin string, args []string)
    ResumeCommand(s Session)          (bin string, args []string)
}
```

The TUI never references "claude" directly. `cmd/root.go` picks the active agent (hardcoded to claude in v1; future: flag or auto-detect).

## Stack

- `github.com/spf13/cobra` — CLI surface
- `github.com/charmbracelet/bubbletea` — TUI event loop
- `github.com/charmbracelet/bubbles` — list, textinput, help, key
- `github.com/charmbracelet/lipgloss` — styling
- `github.com/shirou/gopsutil/v4/process` — process cwd lookup (added in M3, not M1)
- Stdlib `os/exec` for git, `syscall.Exec` for the agent launcher

## Project layout

```
agent-sessions/
├── go.mod
├── main.go                    # cmd.Execute()
├── cmd/
│   ├── root.go                # `asm` (no args) → opens TUI
│   ├── new.go                 # `asm new <name>`
│   └── version.go             # `asm --version`
├── internal/
│   ├── agent/                 # Agent interface + registry
│   │   ├── agent.go
│   │   └── claude/            # Claude Code implementation
│   │       ├── claude.go
│   │       └── paths.go       # ~/.claude/projects path encoding
│   ├── worktree/              # `git worktree` operations
│   │   ├── worktree.go
│   │   └── repo.go            # find repo root, current branch
│   ├── session/               # create-session orchestration (shared by CLI + TUI)
│   │   └── create.go
│   ├── launcher/              # syscall.Exec into the agent
│   │   └── launcher.go
│   └── app/                   # Bubble Tea TUI
│       ├── app.go             # Model
│       ├── update.go
│       ├── view.go
│       ├── keys.go
│       └── styles.go
└── .plans/
    └── v1-plan.md             # this file
```

## State derivation (no state file)

```
worktrees      :=  git worktree list --porcelain
transcripts    :=  agent.Discover(worktrees)            # per-agent
running        :=  agent.IsActiveIn(worktree.path)      # per-agent

for each worktree:
    state = active if running else idle
    pick most-recent transcript as the row's "current" session

orphan transcripts (no matching worktree) → state = disconnected, shown at bottom
```

One row per worktree (with its most recent session). Orphans listed below.

## Key bindings

```
↑/↓  nav     ↵  resume     n  new     d  delete     v  view
/    filter  ?  help       q  quit
```

Footer always visible (lazygit). Modal overlays for: name prompt (`n`), delete confirm (`d`), "already running" blocker (↵ on active).

## Launcher contract

```go
// internal/launcher/launcher.go
func Exec(bin string, args []string, cwd string) error {
    // Caller has already torn down bubbletea.
    if err := syscall.Chdir(cwd); err != nil { return err }
    return syscall.Exec(bin, args, os.Environ())
}
```

The process *becomes* the agent. When the agent exits, the shell prompt is back. No return-to-TUI in v1.

## Worktree creation

- Path: sibling — `<repo-parent>/<repo>-<name>`
- Branch: new branch `asm/<name>` off current `HEAD`
- Command: `git worktree add -b asm/<name> ../<repo>-<name> HEAD`
- Name validation: `^[a-z0-9][a-z0-9-]*$`, reject if branch or path already exists

## Milestones

### M1 — Cobra skeleton + read-only TUI list

- `go.mod` initialized; deps: cobra, bubbletea, bubbles, lipgloss
- `asm` / `asm --version` work
- `asm new <name>` exists as a stub (errors: "not yet implemented")
- TUI opens: header (stats placeholders), list (worktrees joined with transcripts), footer
- Nav (↑/↓), quit (q/esc)
- Claude agent: implements `Discover` (filesystem only — no proc detection yet); all rows show as `idle` for now
- Errors clearly when run outside a git repo

### M2 — `new` works end-to-end (both entry points)

- `internal/session.Create(name)` → validate + git worktree add
- `internal/launcher.Exec(...)` wired
- `cmd/new.go`: validates, creates, exec'd into `claude`
- TUI: `n` opens a textinput modal, on submit same path runs
- Bubbletea is properly released before exec

### M3 — Active detection + stats + resume

- Add `gopsutil/v4/process`
- `claude.IsActiveIn(path)`: any process named `claude` with that cwd
- Header stats become live: `Active: N  Idle: N  Disconnected: N`
- 2s refresh tick
- ↵ on idle → exec `claude --resume <session-id>` in worktree
- ↵ on active → blocking modal: "session already running (PID X)"
- ↵ on disconnected → no-op (defer to M5 detail view)

### M4 — Delete

- `d` opens confirm modal
- Default: `git worktree remove <path>`; if dirty, second confirm step + `--force`
- Optionally move transcript dir to `~/.claude/projects/.archived/<timestamp>-<name>/`
- Refresh list

### M5 — Polish

- `/` filter
- `?` help overlay
- `v` transcript viewer (read-only pager, scrollable) — also the action for disconnected rows
- Styling pass
- Small-terminal fallback

## Open questions / deferred

- **Worktree location**: sibling vs `<repo>/.worktrees/<name>`. Sibling for now.
- **`--no-launch` flag** on `asm new`: deferred; default is always launch.
- **Multi-agent discovery**: v1 hardcodes claude; future flag like `--agent` or auto-detect.
- **Coming back to the TUI** after the agent exits: deferred. Would require child-process model + bubbletea re-entry.
- **Module path**: `github.com/eduardvoiculescu/agent-sessions` as a placeholder — adjust if the canonical path differs.
