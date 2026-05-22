package claude

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/eduard-voiculescu/agent-sessions-manager/internal/agent"
)

// Archive moves the Claude Code project directory tied to s into
// ~/.claude/projects/.archived/<timestamp>-<encoded>/. After this call,
// Discover no longer surfaces the session.
//
// Best-effort semantics:
//   - If the project dir doesn't exist, returns nil.
//   - Resolves the project dir from WorktreePath (preferred) or from the
//     parent of TranscriptPath (for disconnected rows where the worktree is gone).
func (c *Claude) Archive(s agent.Session) error {
	srcDir, err := projectDirForSession(s)
	if err != nil {
		return err
	}
	if srcDir == "" {
		return nil
	}
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("stat %s: %w", srcDir, err)
	}

	root, err := projectsRoot()
	if err != nil {
		return err
	}
	archiveRoot := filepath.Join(root, ".archived")
	if err := os.MkdirAll(archiveRoot, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", archiveRoot, err)
	}

	stamp := time.Now().Format("20060102-150405")
	dst := filepath.Join(archiveRoot, stamp+"-"+filepath.Base(srcDir))
	if err := os.Rename(srcDir, dst); err != nil {
		return fmt.Errorf("rename %s → %s: %w", srcDir, dst, err)
	}
	return nil
}

// projectDirForSession returns the absolute path of the Claude project dir
// (~/.claude/projects/<encoded>/) that holds the session's transcripts, or
// "" if it can't be determined.
func projectDirForSession(s agent.Session) (string, error) {
	root, err := projectsRoot()
	if err != nil {
		return "", err
	}
	if s.WorktreePath != "" {
		return filepath.Join(root, encodeWorktreePath(s.WorktreePath)), nil
	}
	if s.TranscriptPath != "" {
		// Orphan / disconnected — derive from the transcript's parent dir.
		return filepath.Dir(s.TranscriptPath), nil
	}
	return "", nil
}
