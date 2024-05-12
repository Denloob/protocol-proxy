package styles

import (
	"github.com/charmbracelet/lipgloss"
)

var Selected = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#FAFAFA"))
