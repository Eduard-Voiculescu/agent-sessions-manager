package app

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/eduard-voiculescu/agent-sessions-manager/internal/agent"
)

func (m Model) View() string {
	if m.width == 0 {
		return "loading…"
	}
	if m.err != nil {
		return fmt.Sprintf("error: %v\n\npress q to quit", m.err)
	}

	header := m.renderHeader()
	footer := m.renderFooter()
	bodyH := m.height - lipgloss.Height(header) - lipgloss.Height(footer)
	if bodyH < 5 {
		bodyH = 5
	}
	body := m.renderBody(bodyH)

	base := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)

	switch m.mode {
	case modeNewPrompt:
		return m.overlay(base, m.renderNewPrompt())
	case modeBlocker:
		return m.overlay(base, m.renderBlocker())
	case modeDeleteConfirm:
		return m.overlay(base, m.renderDeleteConfirm(false))
	case modeDeleteForceConfirm:
		return m.overlay(base, m.renderDeleteConfirm(true))
	case modeHelp:
		return m.overlay(base, m.renderHelp())
	}
	return base
}

func (m Model) overlay(base, modal string) string {
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		modal,
		lipgloss.WithWhitespaceChars(" "),
	)
}

// --- header ------------------------------------------------------------------

func (m Model) renderHeader() string {
	active, idle, disc := m.stats()
	total := len(m.sessions)
	sep := mutedStyle.Render("│")

	left := strings.Join([]string{
		titleStyle.Render("⎇ Agent Sessions Manager"),
		sep,
		accentStyle.Render(strconv.Itoa(total)) + " " + mutedStyle.Render("sessions"),
		sep,
		statActiveStyle.Render(strconv.Itoa(active)) + " " + mutedStyle.Render("active"),
		sep,
		statIdleStyle.Render(strconv.Itoa(idle)) + " " + mutedStyle.Render("idle"),
		sep,
		statDisconStyle.Render(strconv.Itoa(disc)) + " " + mutedStyle.Render("disconnected"),
	}, " ")
	right := mutedStyle.Render("repo: " + filepath.Base(m.repo.Root))

	width := m.width - 2
	pad := width - lipgloss.Width(left) - lipgloss.Width(right)
	if pad < 1 {
		pad = 1
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", pad), right)
	return headerStyle.Width(width).Render(row)
}

// --- body --------------------------------------------------------------------

func (m Model) renderBody(h int) string {
	sidebarW := 36
	if m.width < 100 {
		sidebarW = 30
	}
	mainW := m.width - sidebarW - 2
	if mainW < 20 {
		mainW = 20
	}

	sidebar := m.renderSidebar(sidebarW, h)
	main := m.renderMain(mainW, h)
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, main)
}

// --- sidebar -----------------------------------------------------------------

func (m Model) renderSidebar(w, h int) string {
	var parts []string
	parts = append(parts, sidebarTitleStyle.Render("Sessions"))
	parts = append(parts, "")

	if m.mode == modeFilter || m.filter != "" {
		parts = append(parts, m.renderFilterBar(), "")
	}

	if len(m.visible) == 0 {
		var msg string
		if m.filter != "" {
			msg = fmt.Sprintf("no match: %q", m.filter)
		} else {
			msg = "no sessions yet"
		}
		parts = append(parts, mutedStyle.Render(msg), "", mutedStyle.Render("press n to create one"))
	} else {
		for i, idx := range m.visible {
			parts = append(parts, m.renderSidebarRow(i, m.sessions[idx], w-4))
		}
	}

	body := strings.Join(parts, "\n")
	return sidebarStyle.Width(w).Height(h).Render(body)
}

func (m Model) renderSidebarRow(i int, s agent.Session, w int) string {
	selected := i == m.cursor
	marker := stateMarker(s.State)
	if selected {
		marker = accentStyle.Render("●")
	}

	name := sessionName(s)
	if selected {
		name = accentStyle.Bold(true).Render(name)
	}

	age := humanAge(s.LastActive)
	branch := s.Branch
	if branch == "" {
		branch = "—"
	}
	stateLabel := mutedStyle.Render(s.State.String())

	if w < 12 {
		w = 12
	}
	nameRoom := w - lipgloss.Width(marker) - 1 - lipgloss.Width(age) - 1
	if nameRoom < 4 {
		nameRoom = 4
	}
	if lipgloss.Width(name) > nameRoom {
		name = truncate(sessionName(s), nameRoom)
		if selected {
			name = accentStyle.Bold(true).Render(name)
		}
	}

	gap := w - lipgloss.Width(marker) - 1 - lipgloss.Width(name) - lipgloss.Width(age)
	if gap < 1 {
		gap = 1
	}
	titleLine := marker + " " + name + strings.Repeat(" ", gap) + mutedStyle.Render(age)
	subLine := "  " + truncate(branch, w-4) + "  " + mutedStyle.Render("·") + " " + stateLabel

	return titleLine + "\n" + subLine
}

func (m Model) renderFilterBar() string {
	if m.mode == modeFilter {
		return m.filterInput.View()
	}
	return mutedStyle.Render(fmt.Sprintf("/ %s", m.filter))
}

// --- main panel --------------------------------------------------------------

func (m Model) renderMain(w, h int) string {
	if m.mode == modeViewer {
		return m.renderViewerPane(w, h)
	}
	if w < 30 {
		// Too narrow for the welcome layout — render compact session detail
		// of the current selection instead.
		return mainBoxStyle.Width(w).Height(h).Render(m.renderSessionDetail(w - 4))
	}

	title := titleStyle.Render("Agent Sessions Manager")
	tagline := mutedStyle.Render("k9s for your coding-agent sessions")

	rows := []string{
		"",
		title,
		tagline,
		"",
		"",
		accentStyle.Render("n") + mutedStyle.Render("  create a new session"),
		accentStyle.Render("↵") + mutedStyle.Render("  resume the highlighted session"),
		accentStyle.Render("d") + mutedStyle.Render("  delete a session"),
		accentStyle.Render("v") + mutedStyle.Render("  view its transcript"),
		accentStyle.Render("/") + mutedStyle.Render("  filter the list"),
		accentStyle.Render("?") + mutedStyle.Render("  full keybindings"),
		"",
		"",
		mutedStyle.Render("Each session is a git worktree on its own branch."),
		mutedStyle.Render("Sessions are derived from "+filepath.Join("~", ".claude", "projects")+"."),
	}

	content := lipgloss.JoinVertical(lipgloss.Center, rows...)
	centered := lipgloss.Place(w-2, h-2, lipgloss.Center, lipgloss.Center, content)
	return mainBoxStyle.Width(w).Height(h).Render(centered)
}

func (m Model) renderSessionDetail(w int) string {
	s, _, ok := m.selected()
	if !ok {
		return mutedStyle.Render("no session selected")
	}
	lines := []string{
		titleStyle.Render(sessionName(s)),
		"",
		fmt.Sprintf("state:   %s", badgeFor(s.State.String())),
		fmt.Sprintf("branch:  %s", s.Branch),
		fmt.Sprintf("path:    %s", truncate(s.WorktreePath, w-9)),
	}
	if !s.LastActive.IsZero() {
		lines = append(lines, fmt.Sprintf("active:  %s", humanAge(s.LastActive)))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// --- footer ------------------------------------------------------------------

func (m Model) renderFooter() string {
	help := strings.Join([]string{
		accentStyle.Render("↑/↓") + " nav",
		accentStyle.Render("↵") + " resume",
		accentStyle.Render("n") + " new",
		accentStyle.Render("d") + " delete",
		accentStyle.Render("v") + " view",
		accentStyle.Render("/") + " filter",
		accentStyle.Render("?") + " help",
		accentStyle.Render("q") + " quit",
	}, "   ")
	return footerStyle.Width(m.width - 2).Render(help)
}

// --- modals ------------------------------------------------------------------

func (m Model) renderNewPrompt() string {
	title := titleStyle.Render("New session")
	help := mutedStyle.Render("a worktree at  ../" + sessionParentBase(m.repo.Root) + "-<name>  on branch  asm/<name>")
	input := m.nameInput.View()
	hint := mutedStyle.Render("↵ create   esc cancel")

	body := lipgloss.JoinVertical(lipgloss.Left,
		title, "", help, "", input,
	)
	if m.promptErr != "" {
		body = lipgloss.JoinVertical(lipgloss.Left, body, "", statDisconStyle.Render(m.promptErr))
	}
	body = lipgloss.JoinVertical(lipgloss.Left, body, "", hint)
	return modalStyle.Width(60).Render(body)
}

func (m Model) renderBlocker() string {
	title := statDisconStyle.Render("Blocked")
	body := lipgloss.JoinVertical(lipgloss.Left,
		title, "", m.blockerText, "",
		mutedStyle.Render("press any key to dismiss"),
	)
	return modalStyle.Width(60).Render(body)
}

func (m Model) renderDeleteConfirm(force bool) string {
	if m.deleteTarget < 0 || m.deleteTarget >= len(m.sessions) {
		return ""
	}
	s := m.sessions[m.deleteTarget]
	title := statDisconStyle.Render("Delete session")

	body := lipgloss.JoinVertical(lipgloss.Left,
		title, "",
		fmt.Sprintf("worktree: %s", s.WorktreePath),
		fmt.Sprintf("branch:   %s", s.Branch),
	)
	if force {
		warn := statDisconStyle.Render("⚠ worktree has uncommitted changes — this will discard them.")
		body = lipgloss.JoinVertical(lipgloss.Left, body, "", warn)
	}
	hint := mutedStyle.Render("y confirm   n/esc cancel")
	body = lipgloss.JoinVertical(lipgloss.Left, body, "", hint)
	return modalStyle.Width(70).Render(body)
}

func (m Model) renderHelp() string {
	title := titleStyle.Render("Keybindings")
	rows := []string{
		"↑/k, ↓/j      navigate",
		"↵             resume selected (blocks if active)",
		"n             new session (worktree + branch + launch)",
		"d             delete selected (confirm)",
		"v             view transcript",
		"/             filter (esc clears)",
		"?             this help",
		"q, esc        quit / dismiss",
	}
	body := lipgloss.JoinVertical(lipgloss.Left, title, "")
	for _, r := range rows {
		body = lipgloss.JoinVertical(lipgloss.Left, body, r)
	}
	body = lipgloss.JoinVertical(lipgloss.Left, body, "", mutedStyle.Render("press any key to dismiss"))
	return modalStyle.Width(60).Render(body)
}

func (m Model) renderViewerPane(w, h int) string {
	title := titleStyle.Render("transcript · " + m.viewerTitle)
	hint := mutedStyle.Render("↑/↓ scroll   esc close")
	body := lipgloss.JoinVertical(lipgloss.Left, title, "", m.viewer.View(), "", hint)
	return mainBoxStyle.Width(w).Height(h).Render(body)
}

// --- helpers -----------------------------------------------------------------

func sessionName(s agent.Session) string {
	if s.WorktreePath != "" {
		return filepath.Base(s.WorktreePath)
	}
	if s.ID != "" {
		return s.ID[:min(8, len(s.ID))]
	}
	return "—"
}

func sessionParentBase(root string) string {
	return filepath.Base(root)
}

func stateMarker(state agent.State) string {
	switch state {
	case agent.StateActive:
		return statActiveStyle.Render("●")
	case agent.StateDisconnected:
		return statDisconStyle.Render("●")
	default:
		return statIdleStyle.Render("○")
	}
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

func humanAge(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
