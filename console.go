package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type Console struct {
	*strings.Builder
	Name string
}

func NewConsole(name string) *Console {
	return &Console{
		Builder: new(strings.Builder),
		Name:    name,
	}
}

func (*Console) Init() tea.Cmd                         { return nil }
func (c *Console) Update(tea.Msg) (tea.Model, tea.Cmd) { return c, nil }
func (c *Console) View() string                        { return c.Name + "\n\n" + c.String() }
