// Package dashboard renders a live bubbletea dashboard of mock server stats.
package dashboard

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Run starts the dashboard program on the alternate screen and returns a
// non-nil error if the program fails to start or exits abnormally.
func Run(s StatsSource, w, h int) error {
	m := newModel(s, w, h)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}