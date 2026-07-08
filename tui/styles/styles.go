package styles

import "github.com/charmbracelet/lipgloss"

var (
	colorBorder = lipgloss.Color("63")
	colorAccent = lipgloss.Color("212")
	colorMuted  = lipgloss.Color("241")
	colorOn     = lipgloss.Color("42")
	colorOff    = lipgloss.Color("203")
	colorWarn   = lipgloss.Color("214")

	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("57")).
			Padding(0, 1)

	TabActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("230")).
			Background(colorAccent).
			Padding(0, 1)

	TabInactiveStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				Padding(0, 1)

	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	StatusOnStyle  = lipgloss.NewStyle().Foreground(colorOn).Bold(true)
	StatusOffStyle = lipgloss.NewStyle().Foreground(colorOff).Bold(true)

	SearchBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorMuted).
			Padding(0, 1)

	HelpStyle = lipgloss.NewStyle().Foreground(colorMuted)

	DeniedStyle = lipgloss.NewStyle().Foreground(colorWarn).Bold(true)
	OkStyle     = lipgloss.NewStyle().Foreground(colorOn).Bold(true)
)
