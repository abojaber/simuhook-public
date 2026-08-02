package dashboard

import (
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"simuhook/mock"
)

// StatsSource is the interface for reading dashboard statistics.
type StatsSource interface {
	Snapshot() mock.StatsSnapshot
}

// tickMsg is a periodic refresh trigger.
type tickMsg time.Time

// Model is the bubbletea model for the dashboard.
type Model struct {
	source   StatsSource
	last     mock.StatsSnapshot
	view     viewport.Model
	height   int
	width    int
	lastErr  error
	interval time.Duration
}

func newModel(s StatsSource, w, h int) Model {
	vp := viewport.New(w, h-3) // header (2) + footer (1)
	return Model{
		source:   s,
		view:     vp,
		width:    w,
		height:   h,
		interval: time.Second,
	}
}

// Init returns the initial commands for the dashboard.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		refreshCmd(m.interval),
		tea.ClearScreen,
	)
}

// refreshCmd returns a command that fires a tickMsg after the given duration.
func refreshCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update handles incoming messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		m.last = m.source.Snapshot()
		m.renderFeed()
		return m, refreshCmd(m.interval)

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			m.last = m.source.Snapshot()
			m.renderFeed()
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.view.Width = msg.Width
		m.view.Height = msg.Height - 3 // header (2) + footer (1)
		m.renderFeed()
		return m, nil
	}

	// Forward all other messages to the viewport (keyboard navigation, etc.)
	var cmd tea.Cmd
	m.view, cmd = m.view.Update(msg)
	return m, cmd
}

// renderFeed builds the feed content from recent logs and sets it on the viewport.
func (m *Model) renderFeed() {
	m.view.SetContent(m.buildFeed())
}