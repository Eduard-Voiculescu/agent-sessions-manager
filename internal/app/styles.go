package app

import "github.com/charmbracelet/lipgloss"

var (
	colorBorder   = lipgloss.Color("240")
	colorMuted    = lipgloss.Color("244")
	colorAccent   = lipgloss.Color("39")
	colorActive   = lipgloss.Color("42")
	colorIdle     = lipgloss.Color("244")
	colorDiscon   = lipgloss.Color("167")
	colorSelected = lipgloss.Color("236")
	colorTitle    = lipgloss.Color("213")
)

var (
	titleStyle = lipgloss.NewStyle().
			Foreground(colorTitle).
			Bold(true).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	statActiveStyle = lipgloss.NewStyle().Foreground(colorActive).Bold(true)
	statIdleStyle   = lipgloss.NewStyle().Foreground(colorIdle)
	statDisconStyle = lipgloss.NewStyle().Foreground(colorDiscon)
	mutedStyle      = lipgloss.NewStyle().Foreground(colorMuted)
	accentStyle     = lipgloss.NewStyle().Foreground(colorAccent)

	// labelStyle is for "branch:", "worktree:" labels in the detail pane —
	// fixed width so values line up.
	labelStyle = lipgloss.NewStyle().Foreground(colorMuted).Width(11)

	rowStyle = lipgloss.NewStyle().Padding(0, 1)
	rowSelectedStyle = lipgloss.NewStyle().
				Background(colorSelected).
				Foreground(lipgloss.Color("231")).
				Padding(0, 1)

	footerStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorBorder).
			Foreground(colorMuted).
			Padding(0, 1)

	listBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorBorder)

	modalStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(1, 2)

	sidebarStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(0, 1)

	mainBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	sidebarTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("231")).
				Bold(true).
				Padding(0, 1).
				Background(lipgloss.Color("237"))
)

func badgeFor(state string) string {
	switch state {
	case "active":
		return statActiveStyle.Render("● active")
	case "disconnected":
		return statDisconStyle.Render("● disconnected")
	default:
		return statIdleStyle.Render("○ idle")
	}
}
