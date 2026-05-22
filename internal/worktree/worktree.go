package worktree

import (
	"bufio"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type Repo struct {
	Root   string // primary worktree root (the main checkout)
	Branch string // current branch in the primary worktree
}

type Worktree struct {
	Path   string
	Branch string
	Head   string
	IsMain bool // true for the primary worktree
}

// FindRepo locates the primary worktree root starting from dir.
func FindRepo(dir string) (Repo, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Repo{}, err
	}
	out, err := runGit(abs, "rev-parse", "--show-toplevel")
	if err != nil {
		return Repo{}, fmt.Errorf("not a git repository (or any parent): %s", abs)
	}
	root := strings.TrimSpace(out)

	branch, _ := runGit(root, "rev-parse", "--abbrev-ref", "HEAD")
	return Repo{Root: root, Branch: strings.TrimSpace(branch)}, nil
}

// List returns all worktrees for the repo.
func List(repoRoot string) ([]Worktree, error) {
	out, err := runGit(repoRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return parsePorcelain(out, repoRoot), nil
}

func parsePorcelain(out, repoRoot string) []Worktree {
	var wts []Worktree
	var cur Worktree
	flush := func() {
		if cur.Path != "" {
			cur.IsMain = (cur.Path == repoRoot)
			wts = append(wts, cur)
		}
		cur = Worktree{}
	}
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			flush()
			continue
		}
		key, val, _ := strings.Cut(line, " ")
		switch key {
		case "worktree":
			cur.Path = val
		case "HEAD":
			cur.Head = val
		case "branch":
			cur.Branch = strings.TrimPrefix(val, "refs/heads/")
		}
	}
	flush()
	return wts
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// Remove deletes the worktree at path. If force is true, dirty worktrees are
// removed too. The asm/<...> branch, if it was created by asm and is no
// longer checked out anywhere, is also deleted.
//
// Returns ErrDirty if the worktree has local changes and force is false —
// callers can prompt the user and retry with force=true.
func Remove(repoRoot, path, branch string, force bool) error {
	args := []string{"worktree", "remove", path}
	if force {
		args = append(args, "--force")
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		s := string(out)
		if !force && (strings.Contains(s, "is dirty") || strings.Contains(s, "contains modified")) {
			return ErrDirty
		}
		return fmt.Errorf("git worktree remove: %w: %s", err, s)
	}

	// Best-effort branch cleanup for asm-created branches.
	if strings.HasPrefix(branch, "asm/") {
		bcmd := exec.Command("git", "branch", "-D", branch)
		bcmd.Dir = repoRoot
		_ = bcmd.Run()
	}
	return nil
}

// ErrDirty signals that `git worktree remove` refused because the worktree
// has uncommitted changes. Re-run Remove with force=true to override.
var ErrDirty = errDirty{}

type errDirty struct{}

func (errDirty) Error() string { return "worktree has uncommitted changes" }
