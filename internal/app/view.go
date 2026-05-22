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
	sidebarW := m.sidebarWidth()
	mainW := m.width - sidebarW - 2
	if mainW < 20 {
		mainW = 20
	}

	sidebar := m.renderSidebar(sidebarW, h)
	main := m.renderMain(mainW, h)
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, main)
}

// sidebarWidth returns the sidebar column width. Used in both renderBody
// and viewerDims so the right pane matches what's actually rendered.
func (m Model) sidebarWidth() int {
	if m.width < 110 {
		return 40
	}
	return 46
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
	if s, _, ok := m.selected(); ok {
		return m.renderDetailPane(w, h, s)
	}
	return m.renderWelcomePane(w, h)
}

func (m Model) renderWelcomePane(w, h int) string {
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
		mutedStyle.Render("Sessions are derived from " + filepath.Join("~", ".claude", "projects") + "."),
	}
	content := lipgloss.JoinVertical(lipgloss.Center, rows...)
	centered := lipgloss.Place(w-2, h-2, lipgloss.Center, lipgloss.Center, content)
	return mainBoxStyle.Width(w).Height(h).Render(centered)
}

func (m Model) renderDetailPane(w, h int, s agent.Session) string {
	innerW := w - 4
	if innerW < 20 {
		innerW = 20
	}

	// Header line: name + state badge (+ PID when active).
	header := titleStyle.Render(sessionName(s))
	stateBit := badgeFor(s.State.String())
	if s.State == agent.StateActive && s.PID != 0 {
		stateBit += " " + mutedStyle.Render(fmt.Sprintf("PID %d", s.PID))
	}

	branch := s.Branch
	if branch == "" {
		branch = "—"
	}

	// Metadata rows.
	var metaRows []string
	if s.ID != "" {
		metaRows = append(metaRows, fmt.Sprintf("%s %s",
			labelStyle.Render("id"),
			mutedStyle.Render(s.ID),
		))
	}
	metaRows = append(metaRows,
		fmt.Sprintf("%s %s", labelStyle.Render("state"), stateBit),
		fmt.Sprintf("%s %s", labelStyle.Render("branch"), branch),
	)
	if s.WorktreePath != "" {
		metaRows = append(metaRows, fmt.Sprintf("%s %s",
			labelStyle.Render("worktree"),
			truncate(s.WorktreePath, innerW-10)))
	}

	// Transcript summary if we have one cached.
	sum, haveSummary := m.detailCache[s.TranscriptPath]
	if s.TranscriptPath != "" {
		tInfo := filepath.Base(s.TranscriptPath)
		if haveSummary {
			tInfo += "   " + mutedStyle.Render(fmt.Sprintf(
				"%s · %s",
				humanBytes(sum.bytes),
				humanAge(sum.mtime),
			))
			if isWritingNow(sum.mtime) {
				tInfo += "  " + statActiveStyle.Render("● writing")
			}
		}
		metaRows = append(metaRows, fmt.Sprintf("%s %s",
			labelStyle.Render("transcript"),
			truncate(tInfo, innerW-12)))
	}

	if haveSummary {
		metaRows = append(metaRows, fmt.Sprintf("%s %d user · %d assistant",
			labelStyle.Render("turns"),
			sum.userTurns,
			sum.assistantTurns,
		))
	}

	// Recent assistant turns section.
	var lastBlock string
	switch {
	case haveSummary && len(sum.recentAssistants) > 0:
		count := len(sum.recentAssistants)
		label := fmt.Sprintf("last %d assistant turn(s)", count)
		divider := mutedStyle.Render(strings.Repeat("─", innerW-2) + "  " + label)
		linesPerTurn := perTurnLines(h, count)
		var rendered []string
		for i, t := range sum.recentAssistants {
			marker := mutedStyle.Render(fmt.Sprintf("[%d/%d]", i+1, count))
			body := wrapAndClamp(t, innerW, linesPerTurn)
			rendered = append(rendered, marker+"\n"+body)
		}
		lastBlock = "\n" + divider + "\n\n" + strings.Join(rendered, "\n\n")
	case s.TranscriptPath == "":
		lastBlock = "\n" + mutedStyle.Render("No transcript yet. Press ↵ to launch this session.")
	case !haveSummary:
		lastBlock = "\n" + mutedStyle.Render("loading transcript…")
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		strings.Join(metaRows, "\n"),
		lastBlock,
	)
	return mainBoxStyle.Width(w).Height(h).Render(content)
}

// perTurnLines returns how many wrapped lines we allow per recent assistant
// turn so that all `count` turns fit inside the pane.
func perTurnLines(paneH, count int) int {
	if count <= 0 {
		return 4
	}
	// pane height − borders/padding (2) − header (1) − blank (1) − meta (≈6)
	// − divider+blank (2) − marker+blank between turns (2 per turn except last)
	usable := paneH - 12 - (count-1)*2
	if usable < count*2 {
		return 2
	}
	n := usable / count
	if n < 2 {
		n = 2
	}
	if n > 8 {
		n = 8
	}
	return n
}

// lastAssistantLines returns roughly how many lines we'll dedicate to the
// last-assistant block given the available pane height.
func lastAssistantLines(h int) int {
	n := h - 11
	if n < 4 {
		n = 4
	}
	if n > 30 {
		n = 30
	}
	return n
}

// wrapAndClamp soft-wraps s to width w and returns at most max lines, with an
// ellipsis row if truncated.
func wrapAndClamp(s string, w, max int) string {
	wrapped := lipgloss.NewStyle().Width(w).Render(s)
	lines := strings.Split(wrapped, "\n")
	if len(lines) <= max {
		return wrapped
	}
	out := append([]string{}, lines[:max]...)
	out = append(out, mutedStyle.Render("…"))
	return strings.Join(out, "\n")
}

func isWritingNow(mtime time.Time) bool {
	return !mtime.IsZero() && time.Since(mtime) < 5*time.Second
}

func humanBytes(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/1024/1024)
	}
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

	var body string
	if s.WorktreePath != "" {
		body = lipgloss.JoinVertical(lipgloss.Left,
			title, "",
			fmt.Sprintf("worktree: %s", s.WorktreePath),
			fmt.Sprintf("branch:   %s", s.Branch),
		)
	} else {
		// Disconnected — no worktree to remove; this is a pure transcript archive.
		body = lipgloss.JoinVertical(lipgloss.Left,
			title, "",
			fmt.Sprintf("transcript: %s", filepath.Base(s.TranscriptPath)),
			mutedStyle.Render("(orphan — no worktree to remove)"),
		)
	}

	body = lipgloss.JoinVertical(lipgloss.Left, body, "",
		mutedStyle.Render("Transcripts will be moved to ~/.claude/projects/.archived/"))

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
	titleText := "transcript · " + m.viewerTitle
	if m.viewerFull {
		titleText += "  " + accentStyle.Render("[full]")
	}
	title := titleStyle.Render(titleText)
	toggleHint := "e expand"
	if m.viewerFull {
		toggleHint = "e collapse"
	}
	hint := mutedStyle.Render("↑/↓ scroll   " + toggleHint + "   esc close")
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
