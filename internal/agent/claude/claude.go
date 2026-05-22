package claude

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/eduard-voiculescu/agent-sessions-manager/internal/agent"
	"github.com/eduard-voiculescu/agent-sessions-manager/internal/worktree"
)

type Claude struct{}

func New() *Claude { return &Claude{} }

func (c *Claude) Name() string { return "claude" }

// Discover joins worktrees with their most-recent transcripts under
// ~/.claude/projects/<encoded-path>/. Orphan transcript dirs that belong to
// this repo (former worktrees) come back as disconnected sessions.
// Transcripts for other repos are ignored.
func (c *Claude) Discover(repo worktree.Repo, wts []worktree.Worktree) ([]agent.Session, error) {
	root, err := projectsRoot()
	if err != nil {
		return nil, err
	}

	wtByEncoded := make(map[string]worktree.Worktree, len(wts))
	for _, w := range wts {
		wtByEncoded[encodeWorktreePath(w.Path)] = w
	}

	repoPrefix := encodeWorktreePath(repo.Root)

	// One process scan per Discover so each row can be tagged active/idle
	// without re-walking /proc per session.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	procs := activeByCwd(ctx)

	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var sessions []agent.Session
	matched := make(map[string]bool)

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name != repoPrefix && !strings.HasPrefix(name, repoPrefix+"-") {
			continue
		}
		dir := filepath.Join(root, name)
		latest, err := latestTranscript(dir)
		if err != nil || latest == "" {
			continue
		}
		s := agent.Session{
			ID:             strings.TrimSuffix(filepath.Base(latest), ".jsonl"),
			Agent:          "claude",
			TranscriptPath: latest,
		}
		fi, err := os.Stat(latest)
		if err == nil {
			s.LastActive = fi.ModTime()
		}
		s.MsgCount = countLines(latest)

		if w, ok := wtByEncoded[name]; ok {
			s.WorktreePath = w.Path
			s.Branch = w.Branch
			if pid, running := procs[w.Path]; running {
				s.State = agent.StateActive
				s.PID = pid
			} else {
				s.State = agent.StateIdle
			}
			matched[name] = true
		} else {
			s.State = agent.StateDisconnected
		}
		sessions = append(sessions, s)
	}

	// Worktrees with no transcript at all — still surface them as rows.
	for enc, w := range wtByEncoded {
		if matched[enc] {
			continue
		}
		s := agent.Session{
			Agent:        "claude",
			WorktreePath: w.Path,
			Branch:       w.Branch,
			State:        agent.StateIdle,
		}
		if pid, running := procs[w.Path]; running {
			s.State = agent.StateActive
			s.PID = pid
		}
		sessions = append(sessions, s)
	}

	sort.SliceStable(sessions, func(i, j int) bool {
		// Connected (worktree-backed) rows first, then disconnected.
		// Within each group, sort alphabetically by sort key so row order
		// stays stable across refreshes regardless of state transitions.
		if (sessions[i].WorktreePath == "") != (sessions[j].WorktreePath == "") {
			return sessions[i].WorktreePath != ""
		}
		return sortKey(sessions[i]) < sortKey(sessions[j])
	})
	return sessions, nil
}

// sortKey returns the stable, lowercase alphabetical sort key for a session.
// Connected rows sort by their worktree basename; orphans by transcript ID.
func sortKey(s agent.Session) string {
	if s.WorktreePath != "" {
		return strings.ToLower(filepath.Base(s.WorktreePath))
	}
	return strings.ToLower(s.ID)
}

func (c *Claude) IsActiveIn(_ string) (bool, int, error) {
	// Wired up in M3 via gopsutil.
	return false, 0, nil
}

func (c *Claude) NewCommand(worktreePath string) (string, []string) {
	return "claude", []string{"claude"}
}

func (c *Claude) ResumeCommand(s agent.Session) (string, []string) {
	return "claude", []string{"claude", "--resume", s.ID}
}

// latestTranscript returns the most recently modified top-level .jsonl
// inside dir (subdirectories like subagents/ are ignored).
func latestTranscript(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var newest string
	var newestMod int64
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		full := filepath.Join(dir, e.Name())
		fi, err := os.Stat(full)
		if err != nil {
			continue
		}
		if t := fi.ModTime().UnixNano(); t > newestMod {
			newestMod = t
			newest = full
		}
	}
	return newest, nil
}

func countLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	n := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		n++
	}
	return n
}
