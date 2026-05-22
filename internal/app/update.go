package app

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/eduard-voiculescu/agent-sessions-manager/internal/agent"
	"github.com/eduard-voiculescu/agent-sessions-manager/internal/session"
	"github.com/eduard-voiculescu/agent-sessions-manager/internal/worktree"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.mode == modeViewer {
			w, h := m.viewerDims()
			m.viewer.Width = w
			m.viewer.Height = h
			if m.viewerRaw != "" {
				m.viewer.SetContent(ansi.Wrap(m.viewerRaw, w, " -"))
			}
		}
		return m, nil

	case refreshMsg:
		m.sessions = msg.sessions
		m.err = msg.err
		m.recomputeVisible()
		return m, summarizeCmd(m.selectedTranscriptPath())

	case tickMsg:
		return m, tea.Batch(
			refreshCmd(m.repo, m.agent),
			tickCmd(),
			summarizeCmd(m.selectedTranscriptPath()),
		)

	case detailMsg:
		if msg.err == nil {
			m.detailCache[msg.path] = msg.summary
		}
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeNewPrompt:
			return m.updateNewPrompt(msg)
		case modeBlocker:
			m.mode = modeNormal
			m.blockerText = ""
			return m, nil
		case modeDeleteConfirm:
			return m.updateDeleteConfirm(msg, false)
		case modeDeleteForceConfirm:
			return m.updateDeleteConfirm(msg, true)
		case modeFilter:
			return m.updateFilter(msg)
		case modeHelp:
			m.mode = modeNormal
			return m, nil
		case modeViewer:
			return m.updateViewer(msg)
		default:
			return m.updateNormal(msg)
		}
	}
	return m, nil
}

func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, keys.Up):
		if m.cursor > 0 {
			m.cursor--
			return m, summarizeCmd(m.selectedTranscriptPath())
		}
	case key.Matches(msg, keys.Down):
		if m.cursor < len(m.visible)-1 {
			m.cursor++
			return m, summarizeCmd(m.selectedTranscriptPath())
		}
	case key.Matches(msg, keys.New):
		m.mode = modeNewPrompt
		m.nameInput.SetValue("")
		m.promptErr = ""
		return m, m.nameInput.Focus()
	case key.Matches(msg, keys.Enter):
		return m.handleEnter()
	case key.Matches(msg, keys.Delete):
		return m.handleDelete()
	case key.Matches(msg, keys.View):
		return m.handleView()
	case key.Matches(msg, keys.Filter):
		m.mode = modeFilter
		m.filterInput.SetValue(m.filter)
		return m, m.filterInput.Focus()
	case key.Matches(msg, keys.Help):
		m.mode = modeHelp
		return m, nil
	}
	return m, nil
}

func (m Model) handleDelete() (tea.Model, tea.Cmd) {
	s, idx, ok := m.selected()
	if !ok {
		return m, nil
	}
	if s.State == agent.StateActive {
		m.mode = modeBlocker
		m.blockerText = fmt.Sprintf(
			"%s is running (PID %d). Close it before deleting.",
			sessionLabel(s), s.PID,
		)
		return m, nil
	}
	m.deleteTarget = idx
	m.mode = modeDeleteConfirm
	return m, nil
}

func (m Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filterInput.Blur()
		m.filterInput.SetValue("")
		m.filter = ""
		m.recomputeVisible()
		m.mode = modeNormal
		return m, nil
	case "enter":
		m.filterInput.Blur()
		m.mode = modeNormal
		return m, nil
	}
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	m.filter = m.filterInput.Value()
	m.recomputeVisible()
	return m, cmd
}

func (m Model) handleView() (tea.Model, tea.Cmd) {
	s, _, ok := m.selected()
	if !ok || s.TranscriptPath == "" {
		m.mode = modeBlocker
		m.blockerText = "No transcript on this row."
		return m, nil
	}
	m.viewerFull = false
	body, err := renderTranscript(s.TranscriptPath, m.viewerFull)
	if err != nil {
		m.mode = modeBlocker
		m.blockerText = "open transcript: " + err.Error()
		return m, nil
	}
	w, h := m.viewerDims()
	m.viewer = viewport.New(w, h)
	m.viewerPath = s.TranscriptPath
	m.viewerRaw = body
	m.viewer.SetContent(ansi.Wrap(body, w, " -"))
	m.viewerTitle = sessionLabel(s)
	m.mode = modeViewer
	return m, nil
}

// viewerDims returns the viewport dimensions sized to fit inside the right
// pane (mainBoxStyle border + padding + title + hint lines).
func (m Model) viewerDims() (int, int) {
	sidebarW := m.sidebarWidth()
	mainW := m.width - sidebarW - 2
	// mainBoxStyle: 1-char border each side + 1-char padding each side = -4
	w := mainW - 4
	if w < 20 {
		w = 20
	}

	headerH := 3 // header + its border
	footerH := 3
	bodyH := m.height - headerH - footerH
	// title + blank + hint + blank + box borders/padding = ~6
	h := bodyH - 6
	if h < 5 {
		h = 5
	}
	return w, h
}

func (m Model) updateViewer(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.mode = modeNormal
		return m, nil
	case "e":
		m.viewerFull = !m.viewerFull
		body, err := renderTranscript(m.viewerPath, m.viewerFull)
		if err == nil {
			m.viewerRaw = body
			m.viewer.SetContent(ansi.Wrap(body, m.viewer.Width, " -"))
			m.viewer.GotoTop()
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.viewer, cmd = m.viewer.Update(msg)
	return m, cmd
}

func (m Model) updateDeleteConfirm(msg tea.KeyMsg, force bool) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "n", "N":
		m.mode = modeNormal
		m.deleteTarget = -1
		return m, nil
	case "y", "Y", "enter":
		if m.deleteTarget < 0 || m.deleteTarget >= len(m.sessions) {
			m.mode = modeNormal
			m.deleteTarget = -1
			return m, nil
		}
		s := m.sessions[m.deleteTarget]

		// Step 1: remove the git worktree (skip when there's nothing to remove).
		if s.WorktreePath != "" {
			err := worktree.Remove(m.repo.Root, s.WorktreePath, s.Branch, force)
			if errors.Is(err, worktree.ErrDirty) {
				m.mode = modeDeleteForceConfirm
				return m, nil
			}
			if err != nil {
				m.mode = modeBlocker
				m.blockerText = "delete failed: " + err.Error()
				m.deleteTarget = -1
				return m, nil
			}
		}

		// Step 2: archive the transcript dir. Best-effort but surface failures.
		if err := m.agent.Archive(s); err != nil {
			m.mode = modeBlocker
			m.blockerText = "worktree removed; archive failed: " + err.Error()
			m.deleteTarget = -1
			return m, refreshCmd(m.repo, m.agent)
		}

		m.mode = modeNormal
		m.deleteTarget = -1
		return m, refreshCmd(m.repo, m.agent)
	}
	return m, nil
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	s, _, ok := m.selected()
	if !ok {
		return m, nil
	}
	switch s.State {
	case agent.StateActive:
		m.mode = modeBlocker
		m.blockerText = fmt.Sprintf(
			"%s is already running (PID %d).\nClose that session before resuming.",
			sessionLabel(s), s.PID,
		)
		return m, nil
	case agent.StateDisconnected:
		m.mode = modeBlocker
		m.blockerText = "This session's worktree is gone. Press v to view the transcript (M5)."
		return m, nil
	default: // idle
		if s.WorktreePath == "" {
			m.mode = modeBlocker
			m.blockerText = "Nothing to launch on this row (no worktree)."
			return m, nil
		}
		var bin string
		var args []string
		if s.ID == "" {
			// Worktree exists but no transcript yet — launch a fresh claude.
			bin, args = m.agent.NewCommand(s.WorktreePath)
		} else {
			bin, args = m.agent.ResumeCommand(s)
		}
		m.PendingExec = &ExecRequest{Cwd: s.WorktreePath, Bin: bin, Args: args}
		return m, tea.Quit
	}
}

func sessionLabel(s agent.Session) string {
	if s.WorktreePath != "" {
		return "session " + s.Branch
	}
	if s.ID != "" {
		return "session " + s.ID[:min(8, len(s.ID))]
	}
	return "session"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m Model) updateNewPrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.promptErr = ""
		m.nameInput.Blur()
		return m, nil
	case "enter":
		name := m.nameInput.Value()
		created, err := session.Create(m.repo, name)
		if err != nil {
			m.promptErr = err.Error()
			if errors.Is(err, session.ErrInvalidName) {
				m.promptErr = "name must be lowercase letters, digits, dashes (e.g. feat-auth)"
			}
			return m, nil
		}
		bin, args := m.agent.NewCommand(created.WorktreePath)
		m.PendingExec = &ExecRequest{Cwd: created.WorktreePath, Bin: bin, Args: args}
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.nameInput, cmd = m.nameInput.Update(msg)
	return m, cmd
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
