package styles

import (
	"github.com/charmbracelet/lipgloss"
)

var Selected = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#FAFAFA"))

var Unstyled = lipgloss.NewStyle()

var Unfocused = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#AAAAAA"))

var UnfocusedSelected = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#AEAEAE"))
