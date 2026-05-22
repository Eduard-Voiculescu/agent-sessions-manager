package claude

import (
	"context"
	"strings"

	"github.com/shirou/gopsutil/v4/process"
)

// activeByCwd returns a map from cwd → pid for every running `claude`
// process. Errors on individual processes are ignored — perms can prevent
// reading some procs' metadata.
func activeByCwd(ctx context.Context) map[string]int {
	out := make(map[string]int)
	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return out
	}
	for _, p := range procs {
		name, err := p.NameWithContext(ctx)
		if err != nil || !isClaudeProc(name) {
			continue
		}
		cwd, err := p.CwdWithContext(ctx)
		if err != nil || cwd == "" {
			continue
		}
		out[cwd] = int(p.Pid)
	}
	return out
}

// isClaudeProc matches "claude" exactly and a few common shell-wrapped forms.
// The CLI is shipped as a Node binary, so on some systems the process name
// shows as "node" — we cannot rely on that. Conservative match for now.
func isClaudeProc(name string) bool {
	n := strings.ToLower(name)
	return n == "claude" || strings.HasPrefix(n, "claude-")
}
