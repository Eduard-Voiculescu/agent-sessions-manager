package app

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/eduard-voiculescu/agent-sessions-manager/internal/agent"
	"github.com/eduard-voiculescu/agent-sessions-manager/internal/worktree"
)

const refreshInterval = 2 * time.Second

type mode int

const (
	modeNormal mode = iota
	modeNewPrompt
	modeBlocker
	modeDeleteConfirm
	modeDeleteForceConfirm
	modeFilter
	modeHelp
	modeViewer
)

// ExecRequest is set by the model when the TUI wants to hand the terminal
// to the agent. The caller (cmd/root) checks for it after the program exits
// and runs launcher.Exec.
type ExecRequest struct {
	Cwd  string
	Bin  string
	Args []string
}

type Model struct {
	repo     worktree.Repo
	agent    agent.Agent
	sessions []agent.Session
	cursor   int
	width    int
	height   int
	err      error

	mode        mode
	nameInput   textinput.Model
	promptErr   string
	blockerText string

	// deleteTarget is the session (by cursor index) that the active delete
	// confirm modal is acting on. -1 when none.
	deleteTarget int

	filter       string
	filterInput  textinput.Model
	visible      []int // indices into sessions, post-filter

	viewer       viewport.Model
	viewerTitle  string
	viewerPath   string // transcript path the viewer is open on; re-rendered on toggle
	viewerRaw    string // unwrapped transcript body; re-wrapped on resize
	viewerFull   bool   // true = no trimming of thinking/tool blocks

	// detailCache maps a transcript path → its lightweight summary, used to
	// render the right-pane session detail without re-parsing on every keystroke.
	detailCache map[string]transcriptSummary

	PendingExec *ExecRequest
}

func New(repo worktree.Repo, a agent.Agent) Model {
	ti := textinput.New()
	ti.Placeholder = "session-name"
	ti.CharLimit = 64
	ti.Width = 40
	ti.Prompt = "› "

	fi := textinput.New()
	fi.Placeholder = "filter"
	fi.CharLimit = 64
	fi.Width = 40
	fi.Prompt = "/ "

	return Model{
		repo:         repo,
		agent:        a,
		nameInput:    ti,
		filterInput:  fi,
		deleteTarget: -1,
		detailCache:  make(map[string]transcriptSummary),
	}
}

// selected returns the session under the cursor (post-filter) or false if
// there's no valid selection.
func (m Model) selected() (agent.Session, int, bool) {
	if m.cursor < 0 || m.cursor >= len(m.visible) {
		return agent.Session{}, -1, false
	}
	idx := m.visible[m.cursor]
	if idx < 0 || idx >= len(m.sessions) {
		return agent.Session{}, -1, false
	}
	return m.sessions[idx], idx, true
}

// recomputeVisible filters m.sessions by m.filter and clamps the cursor into
// the resulting m.visible.
func (m *Model) recomputeVisible() {
	m.visible = m.visible[:0]
	needle := strings.ToLower(strings.TrimSpace(m.filter))
	for i, s := range m.sessions {
		if needle == "" || matchesFilter(s, needle) {
			m.visible = append(m.visible, i)
		}
	}
	if m.cursor >= len(m.visible) {
		m.cursor = len(m.visible) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func matchesFilter(s agent.Session, needle string) bool {
	hay := strings.ToLower(s.WorktreePath + " " + s.Branch + " " + s.ID)
	return strings.Contains(hay, needle)
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(refreshCmd(m.repo, m.agent), tickCmd())
}

// selectedTranscriptPath returns the transcript path of the currently
// selected session, or "" if no session is selected or it has no transcript.
func (m Model) selectedTranscriptPath() string {
	s, _, ok := m.selected()
	if !ok {
		return ""
	}
	return s.TranscriptPath
}

type refreshMsg struct {
	sessions []agent.Session
	err      error
}

type tickMsg struct{}

type detailMsg struct {
	path    string
	summary transcriptSummary
	err     error
}

func tickCmd() tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

func summarizeCmd(path string) tea.Cmd {
	if path == "" {
		return nil
	}
	return func() tea.Msg {
		sum, err := summarize(path)
		return detailMsg{path: path, summary: sum, err: err}
	}
}

func refreshCmd(repo worktree.Repo, a agent.Agent) tea.Cmd {
	return func() tea.Msg {
		wts, err := worktree.List(repo.Root)
		if err != nil {
			return refreshMsg{err: err}
		}
		sessions, err := a.Discover(repo, wts)
		return refreshMsg{sessions: sessions, err: err}
	}
}

func (m Model) stats() (active, idle, disconnected int) {
	for _, s := range m.sessions {
		switch s.State {
		case agent.StateActive:
			active++
		case agent.StateDisconnected:
			disconnected++
		default:
			idle++
		}
	}
	return
}
