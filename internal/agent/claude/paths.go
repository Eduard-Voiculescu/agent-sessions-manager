package claude

import (
	"os"
	"path/filepath"
	"strings"
)

// projectsRoot returns ~/.claude/projects.
func projectsRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "projects"), nil
}

// encodeWorktreePath mirrors Claude Code's encoding for the projects directory:
// the absolute worktree path with each '/' replaced by '-'.
// Example: /Users/foo/bar → -Users-foo-bar
func encodeWorktreePath(absPath string) string {
	return strings.ReplaceAll(absPath, "/", "-")
}
