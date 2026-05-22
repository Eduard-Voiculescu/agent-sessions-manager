// Package session orchestrates session lifecycle operations shared by the
// CLI and the TUI.
package session

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/eduard-voiculescu/agent-sessions-manager/internal/worktree"
)

var (
	ErrInvalidName  = errors.New("name must match [a-z0-9][a-z0-9-]*")
	ErrPathExists   = errors.New("worktree path already exists")
	ErrBranchExists = errors.New("branch already exists")
	ErrNoMain       = errors.New("could not locate the main worktree")
)

var nameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Created describes the result of Create so callers can launch into it.
type Created struct {
	WorktreePath string
	Branch       string
}

// Create makes a new sibling worktree on a fresh branch.
//   - path:   <main-parent>/<main-basename>-<name>
//   - branch: asm/<name>, branched from current HEAD
//
// Pre-checks: name regex, path collision, branch collision.
func Create(repo worktree.Repo, name string) (Created, error) {
	if !nameRE.MatchString(name) {
		return Created{}, ErrInvalidName
	}

	wts, err := worktree.List(repo.Root)
	if err != nil {
		return Created{}, fmt.Errorf("list worktrees: %w", err)
	}
	if len(wts) == 0 {
		return Created{}, ErrNoMain
	}
	// `git worktree list` always lists the main worktree first.
	main := wts[0].Path

	target := filepath.Join(filepath.Dir(main), filepath.Base(main)+"-"+name)
	branch := "asm/" + name

	if _, err := os.Stat(target); err == nil {
		return Created{}, fmt.Errorf("%w: %s", ErrPathExists, target)
	}
	if branchExists(repo.Root, branch) {
		return Created{}, fmt.Errorf("%w: %s", ErrBranchExists, branch)
	}

	cmd := exec.Command("git", "worktree", "add", "-b", branch, target, "HEAD")
	cmd.Dir = repo.Root
	if out, err := cmd.CombinedOutput(); err != nil {
		return Created{}, fmt.Errorf("git worktree add: %w: %s", err, string(out))
	}

	return Created{WorktreePath: target, Branch: branch}, nil
}

func branchExists(repoRoot, branch string) bool {
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = repoRoot
	return cmd.Run() == nil
}
