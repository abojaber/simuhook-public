package dashboard

import (
	"sort"
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
	source    StatsSource
	eps       EndpointSource
	cfg       Config
	last      mock.StatsSnapshot
	endpoints []*mock.Endpoint
	view      viewport.Model
	height    int
	width     int
	lastErr   error
	interval  time.Duration

	// Filtering and prompt state.
	filter    Filter
	prompt    string
	promptErr string
	prompting bool
	showHelp  bool

	// UXR-003 / UXR-004 — live-follow and collapsible endpoints panel.
	liveFollow    bool
	showEndpoints bool

	// UXR-005 — tracks whether endpoints were auto-collapsed (short terminal).
	autoCollapsed bool

	// UXR-008 — feed row expansion.
	expandedID      string          // RequestID of expanded row; "" = none. One at a time.
	feedEntries     []mock.LogEntry // displayed (filtered) entries backing current content
	feedLineToEntry []int           // content line -> index into feedEntries, or -1
	feedEntryToLine []int           // feedEntries index -> content line of its summary line
}

func newModel(s StatsSource, eps EndpointSource, cfg Config, w, h int) Model {
	m := Model{
		source:        s,
		eps:           eps,
		cfg:           cfg,
		width:         w,
		height:        h,
		interval:      time.Second,
		liveFollow:    true,
		showEndpoints: true,
	}
	m.refreshEndpoints()

	// UXR-005 — auto-collapse endpoints panel on short terminals.
	if m.height < 20 && len(m.endpoints) > 0 {
		m.showEndpoints = false
		m.autoCollapsed = true
	}

	vpHeight := h - m.headerHeight() - 1
	if vpHeight < 1 {
		vpHeight = 1
	}
	m.view = viewport.New(w, vpHeight)
	return m
}

// refreshEndpoints re-reads the endpoint list from the EndpointSource and sorts
// it by ID so the list stays stable across tick refreshes (the registry returns
// map iteration order, which is non-deterministic). Safe with a nil source.
func (m *Model) refreshEndpoints() {
	if m.eps != nil {
		eps := m.eps.List()
		sort.Slice(eps, func(i, j int) bool {
			return eps[i].ID < eps[j].ID
		})
		m.endpoints = eps
	} else {
		m.endpoints = nil
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
		m.refreshEndpoints()
		m.syncFeed()
		return m, refreshCmd(m.interval)

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// UXR-005 — auto-collapse/re-expand on terminal resize.
		if m.height < 20 && len(m.endpoints) > 0 {
			m.showEndpoints = false
			m.autoCollapsed = true
		} else if m.autoCollapsed {
			m.showEndpoints = true
			m.autoCollapsed = false
		}
		// else: user explicitly toggled — leave their choice alone.

		m.view.Width = msg.Width
		vpHeight := m.height - m.headerHeight() - 1 // header + footer
		if vpHeight < 1 {
			vpHeight = 1
		}
		m.view.Height = vpHeight
		m.syncFeed()
		// UXR-008 — re-pin expanded summary after resize.
		if line := m.expandedSummaryLine(); line >= 0 {
			m.view.SetYOffset(line)
		}
		return m, nil
	}

	// Forward non-key messages to the viewport only when not prompting.
	if !m.prompting {
		var cmd tea.Cmd
		m.view, cmd = m.view.Update(msg)
		return m, cmd
	}
	return m, nil
}

// handleKeyMsg processes keyboard input, accounting for prompt mode.
func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()

	switch {
	// Quit: cancels prompt when active, otherwise exits the program.
	case s == "q" || s == "ctrl+c":
		if m.prompting {
			m.prompting = false
			m.prompt = ""
			m.promptErr = ""
			return m, nil
		}
		return m, tea.Quit

	// Enter prompt mode.
	case (s == "/" || s == "f") && !m.prompting:
		m.prompting = true
		m.prompt = ""
		m.promptErr = ""
		return m, nil

	// Apply filter from prompt.
	case s == "enter" && m.prompting:
		f, ok := ParseFilter(m.prompt)
		if ok {
			m.filter = f
			m.prompting = false
			m.prompt = ""
			m.promptErr = ""
			m.renderFeed()
			m.syncFeed()
			return m, nil
		}
		// UXR-006 — invalid filter: stay in prompt, show inline error.
		m.promptErr = "invalid status: " + BadStatusToken(m.prompt)
		return m, nil

	// UXR-008 — toggle expansion of the row at the top visible line.
	case s == "enter" && !m.prompting:
		sel := m.selectedIdx()
		if sel < 0 {
			return m, nil
		}
		id := m.feedEntries[sel].RequestID
		if id == "" {
			return m, nil
		}
		if id == m.expandedID {
			// Collapse: resume live-follow and re-anchor to bottom.
			m.expandedID = ""
			m.liveFollow = true
			m.renderFeed()
			m.view.GotoBottom()
			return m, nil
		}
		// Expand (single expanded row at a time).
		m.expandedID = id
		m.liveFollow = false
		m.renderFeed()
		if line := m.expandedSummaryLine(); line >= 0 {
			m.view.SetYOffset(line)
		}
		return m, nil

	// Escape: cancel prompt or clear filter.
	case s == "esc":
		if m.prompting {
			m.prompting = false
			m.prompt = ""
			m.promptErr = ""
		} else {
			collapsed := m.collapseExpansion()
			m.filter = Filter{}
			m.renderFeed()
			m.syncFeed()
			if collapsed {
				m.view.GotoBottom()
			}
		}
		return m, nil

	// 'c' clears the filter when not prompting.
	case s == "c" && !m.prompting:
		collapsed := m.collapseExpansion()
		m.filter = Filter{}
		m.renderFeed()
		m.syncFeed()
		if collapsed {
			m.view.GotoBottom()
		}
		return m, nil

	// Toggle help overlay.
	case s == "?" && !m.prompting:
		m.showHelp = !m.showHelp
		return m, nil

	// Manual refresh.
	case s == "r" && !m.prompting:
		m.last = m.source.Snapshot()
		m.syncFeed()
		return m, nil

	// UXR-003 — toggle live-follow pause/resume.
	case s == "L" && !m.prompting:
		m.liveFollow = !m.liveFollow
		if m.liveFollow {
			m.view.GotoBottom()
		}
		m.syncFeed()
		return m, nil

	// UXR-004 — toggle endpoints panel collapse/expand.
	case s == "e" && !m.prompting:
		m.showEndpoints = !m.showEndpoints
		m.autoCollapsed = false // user explicitly toggled — override auto-collapse
		// Re-size the viewport to account for the new header height.
		m.view.Height = m.height - m.headerHeight() - 1
		if m.view.Height < 1 {
			m.view.Height = 1
		}
		m.syncFeed()
		return m, nil
	}

	// Prompt input mode: swallow all keys.
	if m.prompting {
		switch {
		case msg.Type == tea.KeyBackspace || s == "backspace" || s == "ctrl+h":
			if len(m.prompt) > 0 {
				m.prompt = m.prompt[:len(m.prompt)-1]
			}
			m.promptErr = ""
		case msg.Type == tea.KeyRunes:
			m.prompt += string(msg.Runes)
			m.promptErr = ""
		case msg.Type == tea.KeySpace || s == " ":
			// bubbletea delivers a space keypress as KeySpace (not KeyRunes).
			m.prompt += " "
			m.promptErr = ""
		}
		return m, nil
	}

	// Non-prompt mode: forward unhandled keys to the viewport for scrolling.
	var cmd tea.Cmd
	m.view, cmd = m.view.Update(msg)
	// UXR-003 — resume live-follow when the user scrolls to the bottom.
	if m.view.AtBottom() {
		m.liveFollow = true
	}
	return m, cmd
}

// selectedIdx returns the feedEntries index of the row at the top visible line.
// Returns -1 when there is no valid entry at YOffset (blank line, placeholder,
// bounds exceeded, or nil maps before first render).
func (m Model) selectedIdx() int {
	if m.feedLineToEntry == nil {
		return -1
	}
	y := m.view.YOffset
	if y < 0 || y >= len(m.feedLineToEntry) {
		return -1
	}
	idx := m.feedLineToEntry[y]
	if idx < 0 || idx >= len(m.feedEntries) {
		return -1
	}
	return idx
}

// expandedIdx returns the index of m.expandedID in feedEntries, or -1 if not
// found or not expanded.
func (m Model) expandedIdx() int {
	if m.expandedID == "" {
		return -1
	}
	for i, e := range m.feedEntries {
		if e.RequestID == m.expandedID {
			return i
		}
	}
	return -1
}

// expandedSummaryLine returns the content line number of the expanded entry's
// summary line, or -1 if nothing is expanded or the entry is not in the current
// displayed set.
func (m Model) expandedSummaryLine() int {
	idx := m.expandedIdx()
	if idx < 0 {
		return -1
	}
	if m.feedEntryToLine == nil || idx >= len(m.feedEntryToLine) {
		return -1
	}
	return m.feedEntryToLine[idx]
}

// collapseExpansion clears the expanded row and resumes live-follow. Returns
// true if there was an expansion to clear.
func (m *Model) collapseExpansion() bool {
	if m.expandedID == "" {
		return false
	}
	m.expandedID = ""
	m.liveFollow = true
	return true
}

// syncFeed re-anchors the viewport to the bottom if live-follow is enabled and
// the user was already at the bottom. When an entry is expanded, uses relative
// delta correction so the expanded row's summary stays at its offset.
func (m *Model) syncFeed() {
	atBottom := m.view.AtBottom()
	prevLine := m.expandedSummaryLine()
	prevOffset := m.view.YOffset
	m.renderFeed()
	switch {
	case atBottom && m.liveFollow:
		m.view.GotoBottom()
	case prevLine >= 0:
		if newLine := m.expandedSummaryLine(); newLine >= 0 {
			m.view.SetYOffset(prevOffset + (newLine - prevLine))
		}
	}
}

// renderFeed builds the feed content from recent logs and sets it on the
// viewport. If the expanded entry is no longer in the ring or no longer
// matches the active filter, the expansion is auto-cleared.
func (m *Model) renderFeed() {
	fr := m.buildFeed(-1)
	m.feedEntries = fr.entries
	m.feedLineToEntry = fr.lineToEntry
	m.feedEntryToLine = fr.entryToLine
	m.view.SetContent(fr.content)
	if m.expandedID != "" && !fr.expandedHit {
		m.expandedID = ""
	}
}

// headerHeight returns the number of rows the header occupies: the title,
// the config lines (address/stubs/log-dir/admin), the stats line, and the
// endpoints panel. When showEndpoints is false, the panel is collapsed to a
// single heading line; when true and endpoints exist, the full list is shown.
func (m Model) headerHeight() int {
	h := 3 // title + config + stats
	if len(m.endpoints) > 0 {
		if m.showEndpoints {
			h += 1 + len(m.endpoints) // "Endpoints:" heading + one row each
		} else {
			h += 1 // collapsed: just "Endpoints: (N) ▾"
		}
	}
	return h
}