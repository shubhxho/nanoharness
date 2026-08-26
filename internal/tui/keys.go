package tui

import (
	"github.com/charmbracelet/bubbles/key"
)

type keyMap struct {
	Send, Newline, Provider, Model, Attach, Super, Write, Help, Quit key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Send, k.Newline, k.Super, k.Help, k.Quit}
}
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Send, k.Newline, k.Provider, k.Model},
		{k.Attach, k.Super, k.Write, k.Help, k.Quit},
	}
}

var keys = keyMap{
	Send:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "send via harness")),
	Newline:  key.NewBinding(key.WithKeys("ctrl+j"), key.WithHelp("ctrl+j", "newline")),
	Provider: key.NewBinding(key.WithKeys("f2", "ctrl+p"), key.WithHelp("f2", "provider")),
	Model:    key.NewBinding(key.WithKeys("f3"), key.WithHelp("f3", "model")),
	Attach:   key.NewBinding(key.WithKeys("f4"), key.WithHelp("f4", "attach")),
	Super:    key.NewBinding(key.WithKeys("f5"), key.WithHelp("f5", "superpower")),
	Write:    key.NewBinding(key.WithKeys("ctrl+w"), key.WithHelp("ctrl+w", "arm write")),
	Help:     key.NewBinding(key.WithKeys("f1", "?"), key.WithHelp("f1", "help")),
	Quit:     key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
}
