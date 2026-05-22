# asm — Agent Sessions Manager

A k9s/lazygit-inspired TUI for managing [Claude Code](https://docs.claude.com/claude-code) sessions, each tied to its own git worktree. `asm` owns the lifecycle end-to-end: create a worktree, launch the agent in it, resume, view, delete.

The code is architected behind an `Agent` interface so that codex / grok / antigravity / other coding agents can be added later as new packages — without touching the TUI.

## Install

Requirements:
- Go 1.25+
- `git`
- `claude` on your `$PATH` (the Claude Code CLI)

```bash
git clone https://github.com/eduard-voiculescu/agent-sessions-manager.git
cd agent-sessions-manager
go install ./cmd/asm
```

`asm` lands in `$(go env GOBIN)` (defaults to `$HOME/go/bin`). Make sure that directory is on your `PATH`.

## Quick start

Run `asm` inside any git repository:

```bash
cd ~/code/my-project
asm
```

You'll see your existing Claude Code sessions for that repo, plus a welcome pane on the right. Press `n` to spin up a new one — `asm` will create a sibling worktree, branch off the current `HEAD`, and exec into `claude` inside it.

When you exit Claude, you're back at your shell prompt. Run `asm` again to come back to the manager.

## What asm does

For each git worktree of the current repo, `asm` joins:

- **Worktree state** from `git worktree list`
- **Transcript state** from `~/.claude/projects/<encoded-path>/*.jsonl`
- **Process state** by scanning for running `claude` processes whose `cwd` matches a worktree

…and surfaces each as a row with one of three states:

| State          | Meaning                                                           |
|----------------|-------------------------------------------------------------------|
| `active`       | A `claude` process is currently running in this worktree          |
| `idle`         | The worktree exists; no process is running                        |
| `disconnected` | A transcript exists but its worktree is gone (former session)     |

## Keybindings

```
↑/↓, j/k     navigate sessions
↵            resume the highlighted session (launches fresh if no transcript yet)
n            new session (prompts for a name)
d            delete a session (with confirm; second confirm if dirty)
v            open the full transcript viewer in the right pane
/            filter the session list
?            full help overlay
q, esc       quit (or dismiss the current modal)
```

Pressing `↵` on an **active** row opens a blocker showing the PID instead of resuming — Claude doesn't support two `--resume` of the same session.

## CLI commands

The TUI is the main way to use `asm`, but a few CLI shortcuts skip it:

```bash
asm                    # opens the TUI in the current repo
asm new <name>         # creates a worktree + branch, execs into claude
asm version            # prints the version
```

`asm new <name>` is the exact same code path as pressing `n` in the TUI.

## How sessions are created

`asm new my-feature` (or `n` → `my-feature` in the TUI) runs:

```
git worktree add -b asm/my-feature ../<repo>-my-feature HEAD
```

Then `exec`s `claude` in that worktree. The branch and the worktree directory are siblings of the main checkout, on a fresh branch off the current `HEAD`.

Name rules: `^[a-z0-9][a-z0-9-]*$`. Existing branches or directories are refused.

## Detail pane

When you highlight a session, the right pane shows live detail for it, refreshed every 2 seconds:

- State, branch, worktree path, transcript path + size + age
- Turn counts (user / assistant)
- The most recent assistant turn, wrapped to the pane
- A `● writing` indicator when the transcript was modified in the last 5 seconds

Press `v` to swap that pane for the full scrollable transcript viewer; `esc` returns to the detail pane.

## Limitations (v1)

- Single-repo scope. `asm` only manages worktrees of the repo you launched it from.
- Claude Code only. The `Agent` interface is the seam for future agents.
- Once you launch into `claude`, exiting drops you at your shell — `asm` does not re-enter automatically. Run `asm` again to return.
- A directory rename of the repo strands the transcript under the old encoded path. Move the `.jsonl` once if it happens; future renames are rare.

## License

Apache 2.0.
