package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up         key.Binding
	Down       key.Binding
	Enter      key.Binding
	TogglePass key.Binding
	CopyPass   key.Binding
	Quit       key.Binding
	Escape     key.Binding
}

var keys = keyMap{
	Up:         key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("k/up", "move up")),
	Down:       key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("j/down", "move down")),
	Enter:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "view details")),
	TogglePass: key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "toggle password")),
	CopyPass:   key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy password")),
	Quit:       key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Escape:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear detail")),
}
