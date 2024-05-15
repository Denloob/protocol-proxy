package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type Console struct {
	buf        string
	windowSize tea.WindowSizeMsg
	Name       string
}

func NewConsole(name string) *Console {
	return &Console{
		Name: name,
	}
}
func (c *Console) trimFromRightToSize() {
	lines := strings.Split(c.buf, "\n")
	begin := len(lines) - c.windowSize.Height + 1
	begin = Clamp(begin, 0, len(lines)-1)
	lines = lines[begin:]

	c.buf = strings.Join(lines, "\n")
}

func (c *Console) Write(b []byte) (int, error) {
	c.buf += string(b)

	c.trimFromRightToSize()

	return len(b), nil
}

func (*Console) Init() tea.Cmd { return nil }
func (c *Console) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.windowSize = msg
		c.trimFromRightToSize()
	}
	return c, nil
}
func (c *Console) View() string { return c.Name + "\n\n" + c.buf }
