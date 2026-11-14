// Package dashboard renders a live bubbletea dashboard of mock server stats.
package dashboard

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/abojaber/simuhook-public/mock"
)

// Config carries static server settings shown in the dashboard header.
type Config struct {
	Addr     string // listen address, e.g. ":8080"
	StubsDir string // stubs directory
	LogDir   string // log directory
}

// EndpointSource provides the list of currently-registered endpoints. The mock
// registry satisfies this via its List method.
type EndpointSource interface {
	List() []*mock.Endpoint
}

// Run starts the dashboard program on the alternate screen and returns a
// non-nil error if the program fails to start or exits abnormally.
func Run(s StatsSource, eps EndpointSource, rawSrc RawLogSource, cfg Config, w, h int) error {
	m := newModel(s, eps, rawSrc, cfg, w, h)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
