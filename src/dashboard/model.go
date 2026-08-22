package dashboard

import (
	"sort"
	"strings"
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

const (
	wideMinWidth    = 100
	wideMinHeight   = 14
	panelHeaderRows = 3 // title | column header | separator
	leftFixed       = 29
	streamFixed     = 36
	minFlexCol      = 8
)

// geometry holds all computed layout dimensions.
type geom struct {
	wide, showLeft                                   bool
	leftTotal, leftContent, rightTotal, rightContent int
	panelTotal, panelContent                         int
	viewportH, feedH, epsH                           int
	streamW, epW, pathW                              int
}

// wide returns true when the terminal is wide enough for the two-column layout.
func (m Model) wide() bool {
	return m.width >= wideMinWidth && m.height >= wideMinHeight
}

// chromeHeight returns the number of top rows in the wide layout (4 single-line rows).
func (m Model) chromeHeight() int {
	return 4
}

// feedHeight computes the viewport height for the stream (right) panel. In the
// narrow layout this returns the same value as the old viewport-height math
// (byte-identical to today's headerHeight()-based path).
func (m Model) feedHeight() int {
	if !m.wide() {
		h := m.height - m.headerHeight() - 1
		if h < 1 {
			h = 1
		}
		return h
	}
	g := m.geometry()
	if g.viewportH < 1 {
		return 1
	}
	return g.viewportH
}

// geometry computes all layout dimensions. Pure function, single source of truth.
func (m Model) geometry() geom {
	w := m.width
	h := m.height

	lt := w * 2 / 5 // left panel total width
	rt := w - lt    // right panel total width
	lc := lt - 2    // left content width (inside border)
	rc := rt - 2    // right content width (inside border)

	panelTotal := h - 4 - 1 // chrome (4) + footer (1)
	if panelTotal < 9 {
		panelTotal = 9
	}
	panelContent := panelTotal - 2 // inside border

	vpH := panelContent - panelHeaderRows
	if vpH < 1 {
		vpH = 1
	}

	epW := lc - leftFixed
	if epW < minFlexCol {
		epW = minFlexCol
	}
	pathW := rc - streamFixed
	if pathW < minFlexCol {
		pathW = minFlexCol
	}
	streamW := rc
	if !m.showEndpoints {
		streamW = w - 2
	}
	_ = lc // epsW = leftContent — used by endpointsView.Width via applyGeometry

	return geom{
		wide:         m.wide(),
		showLeft:     m.showEndpoints,
		leftTotal:    lt,
		leftContent:  lc,
		rightTotal:   rt,
		rightContent: rc,
		panelTotal:   panelTotal,
		panelContent: panelContent,
		viewportH:    vpH,
		feedH:        vpH,
		epsH:         vpH,
		streamW:      streamW,
		epW:          epW,
		pathW:        pathW,
	}
}

// applyGeometry pushes the computed sizes onto BOTH viewports.
func (m *Model) applyGeometry() {
	if !m.wide() {
		vpH := m.feedHeight()
		m.view.Width = m.width
		m.view.Height = vpH
		m.endpointsView.Width = 1
		m.endpointsView.Height = 1
		return
	}

	g := m.geometry()
	m.view.Width = g.streamW
	m.view.Height = g.feedH
	if m.showEndpoints {
		m.endpointsView.Width = g.leftContent
		m.endpointsView.Height = g.epsH
	} else {
		m.endpointsView.Width = 1
		m.endpointsView.Height = 1
	}
}

// syncEndpoints rebuilds the endpoints panel content and pushes it to the
// endpoints viewport. No-op in narrow layout.
func (m *Model) syncEndpoints() {
	if !m.wide() {
		return
	}
	g := m.geometry()
	m.endpointsView.SetContent(strings.Join(m.endpointsRows(g), "\n"))
}

// Model is the bubbletea model for the dashboard.
type Model struct {
	source        StatsSource
	eps           EndpointSource
	rawSrc        RawLogSource // nil when no proxy rules configured
	cfg           Config
	last          mock.StatsSnapshot
	endpoints     []*mock.Endpoint
	view          viewport.Model
	endpointsView viewport.Model
	rawLogView    viewport.Model // raw log viewport (viewMode == 1)
	height        int
	width         int
	lastErr       error
	interval      time.Duration

	// View mode: 0 = main dashboard, 1 = raw log view.
	viewMode      int
	liveFollowRaw bool // auto-scroll for raw log (default true)

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
	// UXR-014 / UXR-018 — feed selection cursor (unified, both modes).
	sel        int  // selected entry index into feedEntries
	selTouched bool // true once the user moved the cursor (j/k/↑/↓/page/g); ticks then stop yanking it

	// WS-018 / UXR-013 — request/response inspector.
	inspect       bool // inspect mode active (only when expandedID != "")
	inspectFollow bool // preserved live-follow state for dismiss
}

func newModel(s StatsSource, eps EndpointSource, rawSrc RawLogSource, cfg Config, w, h int) Model {
	m := Model{
		source:        s,
		eps:           eps,
		rawSrc:        rawSrc,
		cfg:           cfg,
		width:         w,
		height:        h,
		interval:      time.Second,
		liveFollow:    true,
		liveFollowRaw: true,
		showEndpoints: true,
	}
	m.refreshEndpoints()

	// UXR-005 — auto-collapse endpoints panel on short terminals.
	if m.height < 20 && len(m.endpoints) > 0 {
		m.showEndpoints = false
		m.autoCollapsed = true
	}

	m.view = viewport.New(1, 1)
	m.endpointsView = viewport.New(1, 1)
	m.rawLogView = viewport.New(w, h-3)
	m.applyGeometry()
	m.syncFeed()
	m.syncEndpoints()
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
		if m.viewMode == 1 {
			// Proxy view: rebuild structured entries + raw bytes.
			m.buildProxyContent()
		} else {
			m.syncFeed()
			m.syncEndpoints()
		}
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

		m.applyGeometry()
		m.syncFeed()
		m.syncEndpoints()
		// Resize raw log viewport.
		m.rawLogView.Width = m.width
		m.rawLogView.Height = m.height - 3 // header + footer
		// UXR-008 — re-pin expanded summary after resize.
		if line := m.expandedSummaryLine(); line >= 0 {
			m.view.SetYOffset(line)
		}
		// UXR-018 polish — keep the cursor visible after a resize. Skipped
		// while a row is expanded: the re-pin owns the offset.
		if m.expandedID == "" {
			m.ensureSelectedVisible()
		}
		return m, nil
	}

	// Forward non-key messages to the appropriate viewport when not prompting.
	if !m.prompting {
		if m.viewMode == 1 {
			var cmd tea.Cmd
			m.rawLogView, cmd = m.rawLogView.Update(msg)
			return m, cmd
		}
		var cmd tea.Cmd
		m.view, cmd = m.view.Update(msg)
		return m, cmd
	}
	return m, nil
}

// handleKeyMsg processes keyboard input, accounting for prompt mode.
func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()

	// Raw log view: handle only view-switching, quit, help, and L keys;
	// forward everything else to the raw log viewport.
	if m.viewMode == 1 && !m.prompting {
		switch {
		case s == "q" || s == "ctrl+c":
			return m, tea.Quit
		case s == "1":
			m.viewMode = 0
			return m, nil
		case s == "esc":
			m.viewMode = 0
			return m, nil
		case s == "L":
			m.liveFollowRaw = !m.liveFollowRaw
			if m.liveFollowRaw {
				m.rawLogView.GotoBottom()
			}
			return m, nil
		case s == "?":
			m.showHelp = !m.showHelp
			return m, nil
		default:
			var cmd tea.Cmd
			m.rawLogView, cmd = m.rawLogView.Update(msg)
			return m, cmd
		}
	}

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

	// View switching: 2 = raw log view (from main view).
	case s == "2" && !m.prompting:
		m.viewMode = 1
		return m, nil

	// Enter prompt mode (main view only).
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
			m.syncEndpoints()
			// UXR-018 polish — explicit user action: keep the cursor visible.
			m.ensureSelectedVisible()
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
			// Collapse: resume live-follow, re-attach the cursor to the
			// newest entry and re-anchor to bottom (UXR-018).
			m.expandedID = ""
			m.inspect = false
			m.liveFollow = true
			m.renderFeed()
			m.attachCursorToNewest()
			m.view.GotoBottom()
			return m, nil
		}
		// Expand (single expanded row at a time). inspect resets to false
		// when switching to a different row.
		m.expandedID = id
		m.inspect = false
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
			m.syncEndpoints()
			if collapsed {
				m.attachCursorToNewest()
				m.view.GotoBottom()
			}
			// UXR-018 polish — explicit user action: keep the cursor visible.
			m.ensureSelectedVisible()
		}
		return m, nil

	// 'c' clears the filter when not prompting.
	case s == "c" && !m.prompting:
		collapsed := m.collapseExpansion()
		m.filter = Filter{}
		m.renderFeed()
		m.syncFeed()
		m.syncEndpoints()
		if collapsed {
			m.attachCursorToNewest()
			m.view.GotoBottom()
		}
		// UXR-018 polish — explicit user action: keep the cursor visible.
		m.ensureSelectedVisible()
		return m, nil

	// Toggle help overlay.
	case s == "?" && !m.prompting:
		m.showHelp = !m.showHelp
		return m, nil

	// Manual refresh.
	case s == "r" && !m.prompting:
		m.last = m.source.Snapshot()
		m.syncFeed()
		m.syncEndpoints()
		return m, nil

	// UXR-003 — toggle live-follow pause/resume.
	case s == "L" && !m.prompting:
		m.liveFollow = !m.liveFollow
		if m.liveFollow {
			// UXR-018 — resume re-attaches the cursor to the newest entry so
			// passive live-follow rides it again.
			m.attachCursorToNewest()
			m.view.GotoBottom()
		}
		m.syncFeed()
		return m, nil

	// UXR-004 — toggle endpoints panel collapse/expand.
	case s == "e" && !m.prompting:
		m.showEndpoints = !m.showEndpoints
		m.autoCollapsed = false // user explicitly toggled — override auto-collapse
		m.applyGeometry()
		m.syncFeed()
		m.syncEndpoints()
		return m, nil

	// WS-018 / UXR-013 — toggle request/response inspector. DASH-004: the
	// inspector opens standalone on the selected row when nothing is expanded
	// (no prior enter press needed); when a row IS expanded the behavior is
	// unchanged (inspect toggles on the expanded row).
	case s == "i" && !m.prompting:
		if m.inspect {
			// Dismiss inspect: preserve position before clearing.
			selBefore := m.selectedIdx()
			m.inspect = false
			m.renderFeed()
			if !m.inspectFollow && selBefore >= 0 {
				if line := m.feedEntryToLine[selBefore]; line >= 0 {
					m.view.SetYOffset(line)
				}
			}
			if m.inspectFollow {
				m.view.GotoBottom()
			}
			return m, nil
		}
		// Open inspect. If nothing is expanded, open standalone on the selected
		// row by adopting its RequestID as expandedID — this reuses the whole
		// expanded-inspect machinery (inline render, summary-line pinning,
		// dismiss/live-follow semantics), so DASH-004 AC1/AC4/AC5 hold by
		// construction. Empty feed or no selected row: safe no-op (AC2).
		if m.expandedID == "" {
			sel := m.selectedIdx()
			if sel < 0 {
				return m, nil
			}
			id := m.feedEntries[sel].RequestID
			if id == "" {
				return m, nil
			}
			m.expandedID = id
		}
		m.inspectFollow = m.liveFollow
		m.inspect = true
		m.liveFollow = false
		m.renderFeed()
		if line := m.expandedSummaryLine(); line >= 0 {
			m.view.SetYOffset(line)
		}
		return m, nil

	// UXR-009 — left-panel independent scroll (wide layout only).
	case (s == "shift+up" || s == "K") && !m.prompting:
		if g := m.geometry(); g.showLeft {
			m.endpointsView.ScrollUp(1)
		}
		return m, nil
	case (s == "shift+down" || s == "J") && !m.prompting:
		if g := m.geometry(); g.showLeft {
			m.endpointsView.ScrollDown(1)
		}
		return m, nil

	// backlog: viewport-g-g-bindings — jump to top/bottom of the stream
	// feed. bubbles v1.0.0's viewport.DefaultKeyMap does not bind g/G (only
	// PageDown/PageUp/HalfPageUp/HalfPageDown/Up/Down/Left/Right), so these
	// are handled explicitly here rather than falling through to
	// m.view.Update. UXR-018 — unconditional cursor jumps (AC9): the cursor
	// is the single source of truth, so GotoTop/GotoBottom set it directly
	// (no offset derivation). Pure navigation — does NOT collapse an active
	// row expansion, consistent with pgup/pgdn/arrow-key navigation.
	case s == "g" && !m.prompting:
		m.sel = clampSel(0, len(m.feedEntries))
		m.selTouched = true
		m.view.GotoTop()
		m.liveFollow = false
		return m, nil
	case s == "G" && !m.prompting:
		m.sel = clampSel(len(m.feedEntries)-1, len(m.feedEntries))
		m.selTouched = false
		m.view.GotoBottom()
		m.liveFollow = true
		return m, nil

	// UXR-018 — page keys: converted to cursor jumps of a viewport page
	// (half that for the half-page bindings), clamped + auto-scroll. The
	// deleted fallthrough re-sync would otherwise let these scroll the view
	// while the cursor drifts off-screen. `f` stays the filter-prompt key;
	// `h`/`l`/`left`/`right` fall through (horizontal scroll is a no-op on
	// vertical content).
	case (s == "pgdown" || s == "pgup" || s == "b" || s == " " ||
		s == "u" || s == "ctrl+u" || s == "d" || s == "ctrl+d") && !m.prompting:
		step := m.view.Height - 1
		if step < 1 {
			step = 1
		}
		switch s {
		case "pgup", "b":
			m.sel = clampSel(m.sel-step, len(m.feedEntries))
		case "u", "ctrl+u":
			m.sel = clampSel(m.sel-step/2, len(m.feedEntries))
		case "pgdown", " ":
			m.sel = clampSel(m.sel+step, len(m.feedEntries))
		default: // "d", "ctrl+d"
			m.sel = clampSel(m.sel+step/2, len(m.feedEntries))
		}
		m.selTouched = true
		m.ensureSelectedVisible()
		return m, nil

	// UXR-018 — nav keys: j/k/↑/↓ move the explicit cursor entry-by-entry
	// in BOTH modes (AC1-AC3, AC7) with auto-scroll (AC5). They never touch
	// liveFollow (AC8) and never reach the viewport fallthrough.
	case (s == "j" || s == "down") && !m.prompting:
		m.sel = clampSel(m.sel+1, len(m.feedEntries))
		m.selTouched = true
		m.ensureSelectedVisible()
		return m, nil
	case (s == "k" || s == "up") && !m.prompting:
		m.sel = clampSel(m.sel-1, len(m.feedEntries))
		m.selTouched = true
		m.ensureSelectedVisible()
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

	// Non-prompt mode: forward unhandled keys to the viewport (horizontal
	// scrolling, unbound keys). UXR-018: no sel re-sync and no live-follow
	// resume — the cursor is the single source of truth (AC7) and liveFollow
	// changes only via L/G/collapse (AC8, decision 2).
	var cmd tea.Cmd
	m.view, cmd = m.view.Update(msg)
	return m, cmd
}

// feedFits reports whether the rendered feed content fits entirely within the
// stream viewport (no scrolling possible). UXR-018 — this is NO LONGER a
// selection-path concept: the cursor (sel) is explicit in both modes. The one
// remaining use is the syncFeed live-anchor guard, which keeps fits-mode
// tick-survival byte-identical.
func (m Model) feedFits() bool {
	return len(m.feedLineToEntry) <= m.view.Height
}

// clampSel clamps sel into [0, n-1]; returns -1 when n == 0.
func clampSel(sel, n int) int {
	if n <= 0 {
		return -1
	}
	if sel < 0 {
		return 0
	}
	if sel >= n {
		return n - 1
	}
	return sel
}

// selectedIdx returns the feedEntries index of the currently selected row.
// UXR-018 — one unified selection model (AC7): the explicit sel cursor in
// BOTH fits and overflow modes. Returns -1 when there is no valid selection
// (empty feed or empty filter result).
func (m Model) selectedIdx() int {
	return clampSel(m.sel, len(m.feedEntries))
}

// ensureSelectedVisible scrolls the stream viewport so the selected entry's
// summary line stays within [YOffset+margin, YOffset+Height-1-margin] with
// margin = 1 (0 when Height < 3). SetYOffset clamps internally (bubbles
// v1.0.0), so in fits mode this is a no-op (maxYOffset 0) — one code path
// for both modes (AC5/AC7). The trailing -1 in feedLineToEntry is never
// targeted (feedEntryToLine holds only real entries); bottom-margin math
// lands exactly on maxYOffset for the last entry.
func (m *Model) ensureSelectedVisible() {
	n := len(m.feedEntries)
	if n == 0 {
		return
	}
	idx := clampSel(m.sel, n)
	if m.feedEntryToLine == nil || idx >= len(m.feedEntryToLine) {
		return
	}
	line := m.feedEntryToLine[idx]
	h := m.view.Height
	margin := 1
	if h < 3 {
		margin = 0
	}
	y := m.view.YOffset
	if line < y+margin {
		m.view.SetYOffset(line - margin)
		return
	}
	if line > y+h-1-margin {
		m.view.SetYOffset(line - h + 1 + margin)
	}
}

// attachCursorToNewest moves the cursor to the newest (last) entry and clears
// selTouched so passive live-follow rides it again. Does NOT touch liveFollow
// or the viewport — callers re-anchor (L-resume, G, collapse paths).
func (m *Model) attachCursorToNewest() {
	m.sel = clampSel(len(m.feedEntries)-1, len(m.feedEntries))
	m.selTouched = false
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
	m.inspect = false
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
		// UXR-018 — live-attach guard (AC8): while passively live-following,
		// the cursor rides the newest entry; once the user moves the cursor
		// (selTouched=true), ticks do not yank it. The !feedFits() guard
		// keeps fits-mode tick-survival byte-identical
		// (TestModelFitsSelection_SurvivesTickAndFilter).
		if !m.selTouched && !m.feedFits() {
			m.sel = clampSel(len(m.feedEntries)-1, len(m.feedEntries))
		}
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
		m.inspect = false
	}
	// UXR-018 — clamp-only: the cursor is the single source of truth, so
	// there is no offset derivation. Clamping keeps the selection in range
	// across ticks and filters (no snap-to-0, no panic). No scroll at tick
	// time — cursor-follow runs only on navigation keys (AC5/AC8).
	m.sel = clampSel(m.sel, len(m.feedEntries))
}

// headerHeight returns the number of rows the header occupies: the title,
// the config lines (address/stubs/log-dir), the stats line, and the
// endpoints panel. When showEndpoints is false, the panel is collapsed to a
// single heading line; when true and endpoints exist, the full list is shown.
// UNCHANGED — narrow layout depends on this byte-identically.
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
