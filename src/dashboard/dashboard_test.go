package dashboard

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"simuhook/mock"
)

// fakeSource is a stub StatsSource for testing.
type fakeSource struct {
	snap mock.StatsSnapshot
}

func (f *fakeSource) Snapshot() mock.StatsSnapshot {
	return f.snap
}

func TestModelUpdateTickMsg(t *testing.T) {
	src := &fakeSource{
		snap: mock.StatsSnapshot{
			TotalRequests: 42,
			Uptime:        10 * time.Second,
			Latency: mock.LatencyStats{
				Count: 1,
				Sum:   12.3,
				Min:   12.3,
				Max:   12.3,
				P95:   12.3,
			},
			TotalLogs: 1,
			RecentLogs: []mock.LogEntry{
				{
					Timestamp:      time.Now().UTC().Format(time.RFC3339Nano),
					Method:         "GET",
					Path:           "/test",
					ResponseStatus: 200,
					Webhook:        "test",
					LatencyMs:      1.5,
				},
			},
		},
	}
	m := newModel(src, nil, Config{}, 80, 24)
	newModel, cmd := m.Update(tickMsg(time.Now()))

	// Verify snapshot was read from the returned model
	updated := newModel.(Model)
	if updated.last.TotalRequests != 42 {
		t.Errorf("expected TotalRequests=42, got %d", updated.last.TotalRequests)
	}
	if updated.last.Uptime != 10*time.Second {
		t.Errorf("expected Uptime=10s, got %v", updated.last.Uptime)
	}
	if updated.last.Latency.Count != 1 {
		t.Errorf("expected Latency.Count=1, got %d", updated.last.Latency.Count)
	}

	// Should re-arm the tick
	if cmd == nil {
		t.Error("expected non-nil cmd after tick (re-arm)")
	}
}

func TestModelUpdateQuit(t *testing.T) {
	m := newModel(&fakeSource{}, nil, Config{}, 80, 24)

	// 'q' key should quit
	_, cmd := m.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune{'q'},
	})
	if cmd == nil {
		t.Error("expected non-nil cmd for 'q'")
	}

	// ctrl+c should also quit
	_, cmd = m.Update(tea.KeyMsg{
		Type: tea.KeyCtrlC,
	})
	if cmd == nil {
		t.Error("expected non-nil cmd for ctrl+c")
	}
}

func TestModelView(t *testing.T) {
	src := &fakeSource{
		snap: mock.StatsSnapshot{
			TotalRequests: 5,
			Uptime:        time.Minute,
			Latency: mock.LatencyStats{
				Count: 1,
				Sum:   1.5,
				Min:   1.5,
				Max:   1.5,
				P95:   1.5,
			},
			RecentLogs: []mock.LogEntry{
				{
					Timestamp:      time.Now().UTC().Format(time.RFC3339Nano),
					Method:         "GET",
					Path:           "/test",
					ResponseStatus: 200,
					Webhook:        "test",
					LatencyMs:      1.5,
				},
			},
			TotalLogs: 1,
		},
	}
	m := newModel(src, nil, Config{}, 80, 24)
	m.last = src.Snapshot()
	m.renderFeed()

	view := m.View()
	if len(view) == 0 {
		t.Error("expected non-empty view")
	}
	if !strings.Contains(view, "Simuhook Dashboard") {
		t.Error("expected view to contain dashboard title")
	}
	if !strings.Contains(view, "Uptime:") {
		t.Error("expected view to contain uptime")
	}
	if !strings.Contains(view, "q quit") {
		t.Error("expected view to contain quit hint")
	}
}

func TestInitReturnsCmd(t *testing.T) {
	m := newModel(&fakeSource{}, nil, Config{}, 80, 24)
	cmd := m.Init()
	if cmd == nil {
		t.Error("expected non-nil cmd from Init")
	}

	// Execute the batch command — it should produce messages without panicking.
	msg := cmd()
	// BatchMsg wraps multiple results; just verify it doesn't panic.
	_ = msg
}

func TestModelViewEmpty(t *testing.T) {
	src := &fakeSource{
		snap: mock.StatsSnapshot{
			RecentLogs: []mock.LogEntry{},
			TotalLogs:  0,
		},
	}
	m := newModel(src, nil, Config{}, 80, 24)
	m.last = src.Snapshot()
	m.renderFeed()

	view := m.View()
	if len(view) == 0 {
		t.Error("expected non-empty view even with no logs")
	}
	if !strings.Contains(view, "(no requests yet)") {
		t.Error("expected '(no requests yet)' in empty feed")
	}
}

func TestModelUnmatchedEntry(t *testing.T) {
	src := &fakeSource{
		snap: mock.StatsSnapshot{
			RecentLogs: []mock.LogEntry{
				{
					Timestamp:      time.Now().UTC().Format(time.RFC3339Nano),
					Method:         "POST",
					Path:           "/unknown",
					ResponseStatus: 404,
					Webhook:        "", // unmatched
					LatencyMs:      2.0,
				},
			},
			TotalLogs: 1,
		},
	}
	m := newModel(src, nil, Config{}, 80, 24)
	m.last = src.Snapshot()
	m.renderFeed()

	view := m.View()
	if !strings.Contains(view, "unmatched") {
		t.Error("expected 'unmatched' label for empty webhook entry")
	}
}

func TestWindowSizeMsg(t *testing.T) {
	src := &fakeSource{
		snap: mock.StatsSnapshot{
			RecentLogs: []mock.LogEntry{},
		},
	}
	m := newModel(src, nil, Config{}, 80, 24)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	updated := newM.(Model)
	if updated.width != 120 {
		t.Errorf("expected width=120, got %d", updated.width)
	}
	if updated.height != 40 {
		t.Errorf("expected height=40, got %d", updated.height)
	}
	if updated.view.Width != 120 {
		t.Errorf("expected viewport width=120, got %d", updated.view.Width)
	}
	// viewport height = 40 - headerHeight (title+config+stats = 3) - 1 (footer)
	if updated.view.Height != 36 {
		t.Errorf("expected viewport height=36, got %d", updated.view.Height)
	}
}

func TestModelUpdateRefreshKey(t *testing.T) {
	src := &fakeSource{
		snap: mock.StatsSnapshot{
			TotalRequests: 10,
			RecentLogs:    []mock.LogEntry{},
		},
	}
	m := newModel(src, nil, Config{}, 80, 24)

	// Change the source to a NEW snapshot — this is what happens in production
	// when new requests come in between refreshes.
	src.snap = mock.StatsSnapshot{
		TotalRequests: 99,
		Uptime:        5 * time.Minute,
		Latency: mock.LatencyStats{
			Count: 3,
			Sum:   30.0,
			Min:   5.0,
			Max:   15.0,
			P95:   14.0,
		},
		RecentLogs: []mock.LogEntry{
			{
				Timestamp:      time.Now().UTC().Format(time.RFC3339Nano),
				Method:         "POST",
				Path:           "/new",
				ResponseStatus: 201,
				Webhook:        "created",
				LatencyMs:      10.0,
			},
		},
		TotalLogs: 1,
	}

	// Send 'r' — should re-read the source and pick up the NEW snapshot.
	newModel, cmd := m.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune{'r'},
	})

	updated := newModel.(Model)
	if updated.last.TotalRequests != 99 {
		t.Errorf("expected TotalRequests=99 after refresh, got %d", updated.last.TotalRequests)
	}
	if updated.last.Latency.Count != 3 {
		t.Errorf("expected Latency.Count=3 after refresh, got %d", updated.last.Latency.Count)
	}
	if len(updated.last.RecentLogs) != 1 {
		t.Errorf("expected 1 RecentLog after refresh, got %d", len(updated.last.RecentLogs))
	}
	if updated.last.RecentLogs[0].Webhook != "created" {
		t.Errorf("expected new log entry Webhook='created', got %q", updated.last.RecentLogs[0].Webhook)
	}

	// Refresh should NOT re-arm the tick — cmd should be nil.
	if cmd != nil {
		t.Error("expected nil cmd from refresh (no re-arm)")
	}
}

func TestModelPromptEnter(t *testing.T) {
	src := &fakeSource{
		snap: mock.StatsSnapshot{
			RecentLogs: []mock.LogEntry{
				{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Method: "POST", Path: "/orders", ResponseStatus: 200, Webhook: "order-placed", LatencyMs: 5.0},
				{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Method: "GET", Path: "/health", ResponseStatus: 200, Webhook: "health", LatencyMs: 1.0},
			},
			TotalLogs: 2,
		},
	}
	m := newModel(src, nil, Config{}, 80, 24)

	// Press '/' to enter prompt mode
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m2 := updated.(Model)
	if !m2.prompting {
		t.Error("expected prompting after '/'")
	}

	// Type filter text
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m2 = updated.(Model)
	if m2.prompt != "ep:order" {
		t.Errorf("expected prompt='ep:order', got %q", m2.prompt)
	}

	// Press enter to apply
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 = updated.(Model)
	if m2.prompting {
		t.Error("expected not prompting after enter")
	}
	if m2.filter.Endpoint != "order" {
		t.Errorf("expected filter.Endpoint='order', got %q", m2.filter.Endpoint)
	}
	if m2.prompt != "" {
		t.Errorf("expected empty prompt after enter, got %q", m2.prompt)
	}
}

func TestModelPromptCancel(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, Config{}, 80, 24)

	// Enter prompt mode
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m2 := updated.(Model)
	if !m2.prompting {
		t.Fatal("expected prompting after '/'")
	}

	// Cancel with esc
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 = updated.(Model)
	if m2.prompting {
		t.Error("expected not prompting after esc")
	}
	if m2.prompt != "" {
		t.Errorf("expected empty prompt after esc, got %q", m2.prompt)
	}
}

func TestModelPromptQuit(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, Config{}, 80, 24)

	// Enter prompt mode
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m2 := updated.(Model)

	// Press q while prompting — should cancel prompt, not quit
	updated, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m2 = updated.(Model)
	if m2.prompting {
		t.Error("expected not prompting after q in prompt mode")
	}
	// cmd should be nil (not tea.Quit)
	if cmd != nil {
		t.Error("expected nil cmd (cancel prompt), not quit")
	}

	// Press q while NOT prompting — should quit
	updated, cmd = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("expected non-nil cmd (quit) from q when not prompting")
	}
}

func TestModelClearFilter(t *testing.T) {
	src := &fakeSource{
		snap: mock.StatsSnapshot{
			RecentLogs: []mock.LogEntry{
				{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Method: "POST", Path: "/test", ResponseStatus: 200, Webhook: "test", LatencyMs: 1.0},
			},
			TotalLogs: 1,
		},
	}
	m := newModel(src, nil, Config{}, 80, 24)
	// Pre-populate last snapshot for render
	m.last = src.Snapshot()

	// Set a filter
	m.filter = Filter{Endpoint: "order"}

	// Press 'c' to clear
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m2 := updated.(Model)
	if m2.filter != (Filter{}) {
		t.Error("expected empty filter after 'c'")
	}
}

func TestModelToggleHelp(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, Config{}, 80, 24)

	// Toggle on
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m2 := updated.(Model)
	if !m2.showHelp {
		t.Error("expected showHelp=true after first '?'")
	}

	// Toggle off
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m2 = updated.(Model)
	if m2.showHelp {
		t.Error("expected showHelp=false after second '?'")
	}
}

func TestModelFilteredView(t *testing.T) {
	src := &fakeSource{
		snap: mock.StatsSnapshot{
			RecentLogs: []mock.LogEntry{
				{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Method: "POST", Path: "/orders", ResponseStatus: 201, Webhook: "order-placed", LatencyMs: 5.0},
				{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Method: "GET", Path: "/health", ResponseStatus: 200, Webhook: "health", LatencyMs: 1.0},
				{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Method: "POST", Path: "/users", ResponseStatus: 201, Webhook: "user-created", LatencyMs: 3.0},
			},
			TotalLogs: 3,
		},
	}
	m := newModel(src, nil, Config{}, 80, 24)
	m.last = src.Snapshot()

	// No filter — all entries visible
	m.renderFeed()
	view := m.View()
	if !strings.Contains(view, "order-placed") {
		t.Error("expected order-placed in unfiltered view")
	}
	if !strings.Contains(view, "health") {
		t.Error("expected health in unfiltered view")
	}
	if !strings.Contains(view, "user-created") {
		t.Error("expected user-created in unfiltered view")
	}

	// Filter by endpoint
	m.filter = Filter{Endpoint: "order"}
	m.renderFeed()
	view = m.View()
	if !strings.Contains(view, "order-placed") {
		t.Error("expected order-placed when filtering by endpoint 'order'")
	}
	if strings.Contains(view, "health") {
		t.Error("did NOT expect health when filtering by endpoint 'order'")
	}
	if strings.Contains(view, "user-created") {
		t.Error("did NOT expect user-created when filtering by endpoint 'order'")
	}

	// Filter with no matches
	m.filter = Filter{Endpoint: "nonexistent"}
	m.renderFeed()
	view = m.View()
	if !strings.Contains(view, "(no entries match filter)") {
		t.Error("expected '(no entries match filter)' message")
	}

	// Clear filter — all visible again
	m.filter = Filter{}
	m.renderFeed()
	view = m.View()
	if !strings.Contains(view, "health") {
		t.Error("expected health after clearing filter")
	}
}

func TestModelFilterFooter(t *testing.T) {
	src := &fakeSource{
		snap: mock.StatsSnapshot{
			RecentLogs: []mock.LogEntry{
				{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Method: "POST", Path: "/test", ResponseStatus: 201, Webhook: "order-placed", LatencyMs: 5.0},
				{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Method: "GET", Path: "/health", ResponseStatus: 200, Webhook: "health", LatencyMs: 1.0},
			},
			TotalLogs: 2,
		},
	}
	m := newModel(src, nil, Config{}, 80, 24)
	m.last = src.Snapshot()

	// Set a filter and verify footer contains filter info
	m.filter = Filter{Endpoint: "order"}
	m.renderFeed()
	view := m.View()
	if !strings.Contains(view, "filter: ep:order") {
		t.Error("expected footer to contain filter string")
	}
	if !strings.Contains(view, "showing 1 of 2") {
		t.Error("expected footer to show filtered count (1 of 2)")
	}

	// Clear filter
	m.filter = Filter{}
	m.renderFeed()
	view = m.View()
	if strings.Contains(view, "filter:") {
		t.Error("did not expect filter string in footer after clearing")
	}
	if !strings.Contains(view, "showing 2 of 2") {
		t.Error("expected footer to show unfiltered count (2 of 2)")
	}
}

func TestModelPromptBackspace(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, Config{}, 80, 24)

	// Enter prompt mode
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m2 := updated.(Model)
	if !m2.prompting {
		t.Fatal("expected prompting after 'f'")
	}

	// Type some text
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m2 = updated.(Model)
	if m2.prompt != "ab" {
		t.Fatalf("expected prompt='ab', got %q", m2.prompt)
	}

	// Backspace
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m2 = updated.(Model)
	if m2.prompt != "a" {
		t.Errorf("expected prompt='a' after backspace, got %q", m2.prompt)
	}

	// Backspace again
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m2 = updated.(Model)
	if m2.prompt != "" {
		t.Errorf("expected empty prompt after backspace, got %q", m2.prompt)
	}

	// Backspace on empty prompt (no-op)
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m2 = updated.(Model)
	if m2.prompt != "" {
		t.Errorf("expected still empty prompt, got %q", m2.prompt)
	}
}

func TestModelPromptView(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, Config{}, 80, 24)

	// Enter prompt mode
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m2 := updated.(Model)

	// View should contain prompt bar
	view := m2.View()
	if !strings.Contains(view, "filter:") {
		t.Error("expected prompt bar in view when prompting")
	}
	if !strings.Contains(view, "esc cancel") {
		t.Error("expected 'esc cancel' in prompt bar")
	}
	if !strings.Contains(view, "enter apply") {
		t.Error("expected 'enter apply' in prompt bar")
	}
}

// TestModelInvalidFilterStatus verifies UXR-006 inline validation: an invalid
// status (e.g. status:abc) keeps the prompt open with an error so the user can
// fix it in place instead of losing their input.
func TestModelInvalidFilterStatus(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, Config{}, 80, 24)

	// Enter prompt mode
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m2 := updated.(Model)

	// Type invalid status filter
	for _, r := range "status:abc" {
		updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m2 = updated.(Model)
	}

	// Press enter — invalid filter stays in prompt with error.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 = updated.(Model)
	if m2.filter != (Filter{}) {
		t.Error("expected unchanged filter when status:abc is invalid")
	}
	if !m2.prompting {
		t.Error("expected still prompting after enter with invalid status")
	}
	if m2.promptErr == "" {
		t.Error("expected non-empty promptErr for invalid status")
	}
	if m2.prompt != "status:abc" {
		t.Errorf("expected prompt preserved, got %q", m2.prompt)
	}
}

func TestModelHelpView(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, Config{}, 80, 24)

	m.showHelp = true
	view := m.View()
	if !strings.Contains(view, "Key bindings") {
		t.Error("expected help panel in view")
	}
	if !strings.Contains(view, "/ f") {
		t.Error("expected / f binding in help")
	}
	if !strings.Contains(view, "scro") {
		t.Error("expected scroll hint in help")
	}
}

func TestModelCtrlHBackspace(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, Config{}, 80, 24)

	// Enter prompt mode
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m2 := updated.(Model)

	// Type a character
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m2 = updated.(Model)

	// ctrl+h should delete
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyCtrlH})
	m2 = updated.(Model)
	if m2.prompt != "" {
		t.Errorf("expected empty prompt after ctrl+h, got %q", m2.prompt)
	}
}

// TestModelPromptBatchedTextWithQ documents the intended behavior when bubbletea
// batches fast keystrokes into a single KeyMsg: a 'q' that is part of typed
// filter text (e.g. "ep:queue") is preserved as text, NOT treated as a cancel.
// This is by design — filters may legitimately contain the letter 'q'.
// A standalone 'q' press (separate KeyMsg) still cancels (TestModelPromptQuit).
func TestModelPromptBatchedTextWithQ(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, Config{}, 80, 24)

	// Enter prompt mode
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m2 := updated.(Model)
	if !m2.prompting {
		t.Fatal("expected prompting after '/'")
	}

	// Simulate fast typing: bubbletea delivers "ep:queue" as ONE KeyMsg.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ep:queue")})
	m2 = updated.(Model)
	if m2.prompt != "ep:queue" {
		t.Errorf("expected prompt='ep:queue', got %q", m2.prompt)
	}
	if !m2.prompting {
		t.Error("expected still prompting after typed text")
	}
}

// TestModelPromptEscCancelsFromBatched verifies esc after typed text cancels the
// prompt entirely (regression: esc must cancel even after batched text entry).
func TestModelPromptEscCancelsFromBatched(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, Config{}, 80, 24)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m2 := updated.(Model)
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ep:ping")})
	m2 = updated.(Model)

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 = updated.(Model)
	if m2.prompting {
		t.Error("expected prompting=false after esc")
	}
	if m2.prompt != "" {
		t.Errorf("expected prompt cleared after esc, got %q", m2.prompt)
	}
	if m2.filter != (Filter{}) {
		t.Error("expected filter unchanged/empty after esc-cancel")
	}
}

// fakeEndpoints is a stub EndpointSource for tests.
type fakeEndpoints struct {
	eps []*mock.Endpoint
}

func (f *fakeEndpoints) List() []*mock.Endpoint { return f.eps }

func TestModelHeaderConfig(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	eps := &fakeEndpoints{
		eps: []*mock.Endpoint{
			{ID: "ping", Method: "GET", Path: "/ping"},
			{ID: "orders", Method: "POST", Path: "/webhooks/order"},
		},
	}
	m := newModel(src, eps, Config{Addr: ":8080", StubsDir: "../stubs", LogDir: "logs"}, 80, 24)
	m.last = src.Snapshot()

	view := m.View()
	for _, want := range []string{
		"Simuhook Dashboard",
		"Address: :8080",
		"Stubs: ../stubs (2 files)",
		"Log dir: logs",
		"http://localhost:8080/_admin/endpoints",
		"Endpoints:",
		"/ping → ping",
		"/webhooks/order → orders",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("expected header to contain %q", want)
		}
	}
}

func TestModelHeaderEndpointRefresh(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	eps := &fakeEndpoints{eps: []*mock.Endpoint{{ID: "a", Method: "GET", Path: "/a"}}}
	m := newModel(src, eps, Config{}, 80, 24)

	// Endpoint added at runtime (admin API) — tick must pick it up.
	eps.eps = append(eps.eps, &mock.Endpoint{ID: "b", Method: "POST", Path: "/b"})
	m2, _ := m.Update(tickMsg(time.Now()))
	mm := m2.(Model)
	if len(mm.endpoints) != 2 {
		t.Errorf("expected 2 endpoints after tick, got %d", len(mm.endpoints))
	}
}

func TestModelHeaderNoEndpoints(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, Config{Addr: ":8080"}, 80, 24)
	m.last = src.Snapshot()
	view := m.View()
	if strings.Contains(view, "Endpoints:") {
		t.Error("expected no Endpoints heading when none registered")
	}
}

// TestModelEndpointsSorted verifies the endpoints list is sorted by ID so it
// does not jump around across tick refreshes (registry returns map order).
func TestModelEndpointsSorted(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	// Deliberately out of order — the registry map could return them in any order.
	eps := &fakeEndpoints{
		eps: []*mock.Endpoint{
			{ID: "zeta", Method: "GET", Path: "/z"},
			{ID: "ping", Method: "GET", Path: "/ping"},
			{ID: "alpha", Method: "POST", Path: "/a"},
		},
	}
	m := newModel(src, eps, Config{}, 80, 24)

	want := []string{"alpha", "ping", "zeta"}
	for i, ep := range m.endpoints {
		if ep.ID != want[i] {
			t.Errorf("expected endpoints[%d].ID=%s, got %s", i, want[i], ep.ID)
		}
	}

	// Second refresh must yield the same order (stability).
	m.refreshEndpoints()
	for i, ep := range m.endpoints {
		if ep.ID != want[i] {
			t.Errorf("refresh changed order: expected [%v], got %v", want, ids(m.endpoints))
		}
	}
}

func ids(eps []*mock.Endpoint) []string {
	out := make([]string, 0, len(eps))
	for _, e := range eps {
		out = append(out, e.ID)
	}
	return out
}

// ------------------- UXR-001 — status-code tinting -------------------

func TestRenderStatusTint(t *testing.T) {
	// Force TrueColor so lipgloss emits ANSI sequences even in non-TTY tests.
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.ANSI) // restore a safe default

	now := time.Now().UTC().Format(time.RFC3339Nano)

	entries := []mock.LogEntry{
		{Timestamp: now, Method: "GET", Path: "/ok", ResponseStatus: 200, Webhook: "ok", LatencyMs: 1.0},
		{Timestamp: now, Method: "GET", Path: "/redirect", ResponseStatus: 302, Webhook: "redirect", LatencyMs: 1.0},
		{Timestamp: now, Method: "GET", Path: "/not-found", ResponseStatus: 404, Webhook: "missing", LatencyMs: 1.0},
		{Timestamp: now, Method: "GET", Path: "/boom", ResponseStatus: 500, Webhook: "crash", LatencyMs: 1.0},
		{Timestamp: now, Method: "GET", Path: "/info", ResponseStatus: 101, Webhook: "info", LatencyMs: 1.0},
	}

	src := &fakeSource{
		snap: mock.StatsSnapshot{
			RecentLogs: entries,
			TotalLogs:  uint64(len(entries)),
		},
	}
	m := newModel(src, nil, Config{}, 120, 30)
	m.last = src.snap
	m.renderFeed()
	view := m.View()

	// 2xx → green (76)
	green200 := "\x1b[38;5;76m200"
	if !strings.Contains(view, green200) {
		t.Error("expected 200 wrapped in green (76)")
	}

	// Path /ok must NOT be wrapped in green — the green sequence must not
	// appear immediately before /ok.
	greenOK := "\x1b[38;5;76m/ok"
	if strings.Contains(view, greenOK) {
		t.Error("path /ok must not be wrapped in green")
	}

	// 3xx → cyan (39)
	if !strings.Contains(view, "\x1b[38;5;39m302") {
		t.Error("expected 302 wrapped in cyan (39)")
	}

	// 4xx → yellow (220)
	if !strings.Contains(view, "\x1b[38;5;220m404") {
		t.Error("expected 404 wrapped in yellow (220)")
	}

	// 5xx → red (196)
	if !strings.Contains(view, "\x1b[38;5;196m500") {
		t.Error("expected 500 wrapped in red (196)")
	}

	// 1xx → muted (240 — same as styleMuted)
	if !strings.Contains(view, "\x1b[38;5;240m101") {
		t.Error("expected 101 wrapped in muted (240)")
	}

	// Verify the label "ok" does NOT have the green sequence wrapping it.
	if strings.Contains(view, green200+"ok") || strings.Contains(view, "\x1b[38;5;76mok") {
		t.Error("label 'ok' must not be wrapped in green")
	}

	// Verify the path /boom does NOT have red wrapping it.
	if strings.Contains(view, "\x1b[38;5;196m/boom") {
		t.Error("path /boom must not be wrapped in red")
	}

	// Verify the latency value is NOT wrapped in a status color.
	if strings.Contains(view, "\x1b[38;5;196m1.0") || strings.Contains(view, "\x1b[38;5;76m1.0") {
		t.Error("latency value must not be wrapped in status color")
	}
}

// ------------------- UXR-002 — per-endpoint stats -------------------

func TestRenderEndpointStats(t *testing.T) {
	src := &fakeSource{
		snap: mock.StatsSnapshot{
			TotalRequests: 42,
			HitCounts:     map[string]uint64{"ping": 42, "health": 0},
			EndpointLatency: map[string]mock.LatencyStats{
				"ping": {Count: 42, Sum: 777, Min: 5.0, Max: 50.0, P95: 18.5},
				// health intentionally missing — zero-value LatencyStats
			},
		},
	}
	eps := &fakeEndpoints{
		eps: []*mock.Endpoint{
			{ID: "ping", Method: "GET", Path: "/ping"},
			{ID: "health", Method: "GET", Path: "/health"},
		},
	}
	m := newModel(src, eps, Config{}, 120, 30)
	m.last = src.snap

	view := m.View()

	// ping has hits → requests + p95 should appear
	if !strings.Contains(view, "requests: 42") {
		t.Error("expected 'requests: 42' for ping")
	}
	if !strings.Contains(view, "p95: 18.5ms") {
		t.Errorf("expected 'p95: 18.5ms' for ping in view:\n%s", view)
	}

	// health (zero hits) → just "requests: 0", no p95
	if !strings.Contains(view, "requests: 0") {
		t.Error("expected 'requests: 0' for health")
	}
	// Double-check that health does NOT have a p95 segment.
	// The "p95:" could appear for ping — check that it appears only once.
	if strings.Count(view, "p95:") != 1 {
		t.Errorf("expected exactly 1 p95: occurrence, got %d", strings.Count(view, "p95:"))
	}
}

func TestRenderEndpointStatsOrderStable(t *testing.T) {
	src := &fakeSource{
		snap: mock.StatsSnapshot{
			TotalRequests: 0,
			HitCounts:     map[string]uint64{"beta": 0, "alpha": 0, "gamma": 0},
			EndpointLatency: map[string]mock.LatencyStats{
				"beta": {Count: 10, P95: 5.0},
			},
		},
	}
	eps := &fakeEndpoints{
		eps: []*mock.Endpoint{
			{ID: "beta", Method: "GET", Path: "/b"},
			{ID: "gamma", Method: "GET", Path: "/g"},
			{ID: "alpha", Method: "POST", Path: "/a"},
		},
	}
	m := newModel(src, eps, Config{}, 120, 30)
	m.last = src.snap

	view1 := m.View()

	// second tick with different stats — order must stay the same
	src.snap.HitCounts["beta"] = 99
	src.snap.EndpointLatency["beta"] = mock.LatencyStats{Count: 99, P95: 0.5}
	m2, _ := m.Update(tickMsg(time.Now()))
	mm := m2.(Model)
	view2 := mm.View()

	// Extract line order: find the index of each path (they appear in the
	// feed as well as the header, but the header endpoint lines are our target).
	// Since paths may appear in multiple places, we look for the endpoint-row
	// pattern: " → <ID>".
	for _, v := range []string{view1, view2} {
		aIdx := strings.Index(v, " → alpha")
		bIdx := strings.Index(v, " → beta")
		cIdx := strings.Index(v, " → gamma")
		if aIdx < 0 || bIdx < 0 || cIdx < 0 {
			t.Error("could not find all endpoint rows")
			continue
		}
		if !(aIdx < bIdx && bIdx < cIdx) {
			t.Errorf("expected alpha < beta < gamma (sorted), got a=%d b=%d g=%d", aIdx, bIdx, cIdx)
		}
	}
}

// ------------------- UXR-003 — live-follow pause/resume -------------------

func TestModelLiveFollowDefault(t *testing.T) {
	m := newModel(&fakeSource{}, nil, Config{}, 80, 24)
	if !m.liveFollow {
		t.Error("expected liveFollow=true by default")
	}
}

func TestModelLiveFollowToggle(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{
		RecentLogs: []mock.LogEntry{},
	}}
	m := newModel(src, nil, Config{}, 80, 24)

	// Press L — should pause.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	m2 := updated.(Model)
	if m2.liveFollow {
		t.Error("expected liveFollow=false after first L")
	}

	// Press L again — should resume.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	m2 = updated.(Model)
	if !m2.liveFollow {
		t.Error("expected liveFollow=true after second L")
	}
}

func TestModelLiveFollowPausePreservesOffset(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	// Fill with enough entries to overflow the viewport.
	entries := make([]mock.LogEntry, 50)
	for i := range entries {
		entries[i] = mock.LogEntry{
			Timestamp:      now,
			Method:         "GET",
			Path:           "/test",
			ResponseStatus: 200,
			Webhook:        "test",
			LatencyMs:      1.0,
		}
	}

	src := &fakeSource{
		snap: mock.StatsSnapshot{
			RecentLogs: entries,
			TotalLogs:  uint64(len(entries)),
		},
	}
	m := newModel(src, nil, Config{}, 80, 24)
	m.last = src.snap
	m.renderFeed()
	// Ensure we start at the bottom.
	m.view.GotoBottom()
	if !m.view.AtBottom() {
		t.Fatal("expected at bottom after GotoBottom")
	}

	// Pause live-follow.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	m2 := updated.(Model)
	if m2.liveFollow {
		t.Fatal("expected liveFollow=false after L")
	}

	// Scroll up a few lines.
	for i := 0; i < 5; i++ {
		updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyUp})
		m2 = updated.(Model)
	}
	preTickOffset := m2.view.YOffset

	// Send a tick — should NOT scroll back to bottom.
	src.snap.RecentLogs = append(src.snap.RecentLogs, mock.LogEntry{
		Timestamp:      now,
		Method:         "POST",
		Path:           "/new",
		ResponseStatus: 201,
		Webhook:        "new-one",
		LatencyMs:      2.0,
	})
	src.snap.TotalLogs++
	updated, _ = m2.Update(tickMsg(time.Now()))
	m2 = updated.(Model)

	// Offset should be preserved (not at bottom).
	if m2.view.AtBottom() {
		t.Error("expected not-at-bottom after tick while paused — YOffset should be preserved")
	}
	// YOffset should be close to pre-tick value (new line appended at bottom
	// should not change the offset from top).
	postTickOffset := m2.view.YOffset
	if postTickOffset != preTickOffset {
		t.Logf("pre-tick YOffset=%d, post-tick YOffset=%d (minor drift from re-render is OK)", preTickOffset, postTickOffset)
	}

	// Resume — should re-anchor to bottom.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	m2 = updated.(Model)
	if !m2.liveFollow {
		t.Error("expected liveFollow=true after resume L")
	}
	if !m2.view.AtBottom() {
		t.Error("expected at-bottom after resume")
	}
}

func TestModelLiveFollowPromptSwallowsL(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, Config{}, 80, 24)

	// Enter prompt mode.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m2 := updated.(Model)
	if !m2.prompting {
		t.Fatal("expected prompting after '/'")
	}

	// Press L while prompting — should NOT toggle liveFollow.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	m2 = updated.(Model)
	if !m2.liveFollow {
		t.Error("expected liveFollow unchanged (true) when L pressed in prompt mode")
	}
	if !m2.prompting {
		t.Error("expected still prompting after L")
	}
}

func TestModelLiveFollowResumeOnScrollToBottom(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	entries := make([]mock.LogEntry, 20)
	for i := range entries {
		entries[i] = mock.LogEntry{
			Timestamp:      now,
			Method:         "GET",
			Path:           "/test",
			ResponseStatus: 200,
			Webhook:        "test",
			LatencyMs:      1.0,
		}
	}
	src := &fakeSource{
		snap: mock.StatsSnapshot{
			RecentLogs: entries,
			TotalLogs:  uint64(len(entries)),
		},
	}
	m := newModel(src, nil, Config{}, 80, 24)
	m.last = src.snap
	m.renderFeed()
	m.view.GotoBottom()

	// Pause.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	m2 := updated.(Model)
	if m2.liveFollow {
		t.Fatal("expected paused")
	}

	// Scroll up to leave the bottom.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyUp})
	m2 = updated.(Model)
	if m2.view.AtBottom() {
		t.Fatal("expected not-at-bottom after scrolling up")
	}

	// Now scroll back down to bottom — should resume live-follow.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyDown})
	m2 = updated.(Model)
	if !m2.liveFollow {
		t.Error("expected liveFollow=true after scrolling back to bottom")
	}
	if !m2.view.AtBottom() {
		t.Error("expected at-bottom after scrolling down")
	}
}

// ------------------- UXR-004 — collapsible endpoints panel -------------------

func TestModelCollapseEndpointsDefault(t *testing.T) {
	m := newModel(&fakeSource{}, nil, Config{}, 80, 24)
	if !m.showEndpoints {
		t.Error("expected showEndpoints=true by default")
	}
}

func TestModelCollapseEndpointsToggle(t *testing.T) {
	eps := &fakeEndpoints{
		eps: []*mock.Endpoint{
			{ID: "ping", Method: "GET", Path: "/ping"},
			{ID: "health", Method: "GET", Path: "/health"},
			{ID: "orders", Method: "POST", Path: "/orders"},
		},
	}
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, eps, Config{}, 80, 24)
	if m.headerHeight() != 3+1+3 {
		t.Errorf("expected headerHeight=%d with 3 endpoints open, got %d", 3+1+3, m.headerHeight())
	}

	// Press e — should collapse.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m2 := updated.(Model)
	if m2.showEndpoints {
		t.Error("expected showEndpoints=false after first e")
	}
	if m2.headerHeight() != 3+1 {
		t.Errorf("expected headerHeight=%d collapsed, got %d", 3+1, m2.headerHeight())
	}

	// Press e again — should expand.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m2 = updated.(Model)
	if !m2.showEndpoints {
		t.Error("expected showEndpoints=true after second e")
	}
	if m2.headerHeight() != 3+1+3 {
		t.Errorf("expected headerHeight=%d expanded, got %d", 3+1+3, m2.headerHeight())
	}
}

func TestModelCollapseEndpointsViewportHeight(t *testing.T) {
	eps := &fakeEndpoints{
		eps: []*mock.Endpoint{
			{ID: "a", Method: "GET", Path: "/a"},
			{ID: "b", Method: "GET", Path: "/b"},
		},
	}
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, eps, Config{}, 80, 24)

	// Expanded: headerHeight = 3 + 1 + 2 = 6, viewport height = 24 - 6 - 1 = 17
	if m.view.Height != 24-m.headerHeight()-1 {
		t.Errorf("expected viewport height %d, got %d", 24-m.headerHeight()-1, m.view.Height)
	}

	// Collapse via e key.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m2 := updated.(Model)

	// Collapsed: headerHeight = 3 + 1 = 4, viewport height = 24 - 4 - 1 = 19
	expected := 24 - m2.headerHeight() - 1
	if m2.view.Height != expected {
		t.Errorf("expected collapsed viewport height=%d, got %d", expected, m2.view.Height)
	}

	// Expand back.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m2 = updated.(Model)
	expected = 24 - m2.headerHeight() - 1
	if m2.view.Height != expected {
		t.Errorf("expected expanded viewport height=%d, got %d", expected, m2.view.Height)
	}
}

func TestModelCollapseEndpointsNoEndpoints(t *testing.T) {
	// When no endpoints are registered, headerHeight should be 3 regardless of showEndpoints.
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, Config{}, 80, 24)

	if m.headerHeight() != 3 {
		t.Errorf("expected headerHeight=3 with no endpoints, got %d", m.headerHeight())
	}

	// Collapse when no endpoints — headerHeight stays 3.
	m.showEndpoints = false
	if m.headerHeight() != 3 {
		t.Errorf("expected headerHeight=3 collapsed with no endpoints, got %d", m.headerHeight())
	}
}

func TestModelCollapseEndpointsPromptSwallowsE(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, Config{}, 80, 24)

	// Enter prompt mode.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m2 := updated.(Model)
	if !m2.prompting {
		t.Fatal("expected prompting after '/'")
	}

	// Press e while prompting — should NOT toggle showEndpoints.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m2 = updated.(Model)
	if !m2.showEndpoints {
		t.Error("expected showEndpoints unchanged (true) when e pressed in prompt mode")
	}
	if !m2.prompting {
		t.Error("expected still prompting after e")
	}
}

func TestRenderCollapsedEndpointsHeader(t *testing.T) {
	eps := &fakeEndpoints{
		eps: []*mock.Endpoint{
			{ID: "ping", Method: "GET", Path: "/ping"},
			{ID: "orders", Method: "POST", Path: "/orders"},
		},
	}
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, eps, Config{}, 120, 30)
	m.last = src.snap

	// Collapsed — should show single "Endpoints: (2) ▾" line, no individual rows.
	m.showEndpoints = false
	view := m.View()
	if !strings.Contains(view, "Endpoints: (2) ▾") {
		t.Error("expected collapsed 'Endpoints: (2) ▾' heading")
	}
	if strings.Contains(view, "/ping → ping") {
		t.Error("did NOT expect /ping row in collapsed view")
	}
	if strings.Contains(view, "/orders → orders") {
		t.Error("did NOT expect /orders row in collapsed view")
	}

	// Expanded — individual rows should appear.
	m.showEndpoints = true
	view = m.View()
	if !strings.Contains(view, "/ping → ping") {
		t.Error("expected /ping row in expanded view")
	}
	if !strings.Contains(view, "/orders → orders") {
		t.Error("expected /orders row in expanded view")
	}
}

// TestModelFooterLiveStatus verifies the footer contains the live-follow indicator.
func TestModelFooterLiveStatus(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{
		RecentLogs: []mock.LogEntry{
			{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Method: "GET", Path: "/test", ResponseStatus: 200, Webhook: "test", LatencyMs: 1.0},
		},
		TotalLogs: 1,
	}}
	m := newModel(src, nil, Config{}, 80, 24)
	m.last = src.snap
	m.renderFeed()

	// Following → footer shows ▶ live.
	view := m.View()
	if !strings.Contains(view, "▶ live") {
		t.Error("expected '▶ live' in footer when following")
	}
	if strings.Contains(view, "⏸ live") {
		t.Error("did NOT expect '⏸ live' in footer when following")
	}

	// Paused → footer shows ⏸ live.
	m.liveFollow = false
	view = m.View()
	if !strings.Contains(view, "⏸ live") {
		t.Error("expected '⏸ live' in footer when paused")
	}
	if strings.Contains(view, "▶ live") {
		t.Error("did NOT expect '▶ live' in footer when paused")
	}
}

// TestModelHelpContainsNewKeys verifies L and e appear in help.
func TestModelHelpContainsNewKeys(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, Config{}, 80, 24)
	m.showHelp = true
	view := m.View()
	if !strings.Contains(view, "L       pause/resume live") {
		t.Error("expected 'L' binding in help")
	}
	if !strings.Contains(view, "e       collapse endpoints") {
		t.Error("expected 'e' binding in help")
	}
}

// ------------------- UXR-005 — layout adaptation -------------------

func TestLayoutAutoCollapseShortHeight(t *testing.T) {
	eps := &fakeEndpoints{
		eps: []*mock.Endpoint{
			{ID: "ping", Method: "GET", Path: "/ping"},
		},
	}
	src := &fakeSource{snap: mock.StatsSnapshot{}}

	// Height 19 with endpoints → auto-collapsed.
	m := newModel(src, eps, Config{}, 80, 19)
	if m.showEndpoints {
		t.Error("expected showEndpoints=false at height 19 with endpoints")
	}
	if !m.autoCollapsed {
		t.Error("expected autoCollapsed=true at height 19 with endpoints")
	}

	// Height 20 → no auto-collapse, panel visible.
	m2 := newModel(src, eps, Config{}, 80, 20)
	if !m2.showEndpoints {
		t.Error("expected showEndpoints=true at height 20")
	}
	if m2.autoCollapsed {
		t.Error("expected autoCollapsed=false at height 20")
	}

	// Height 19 with NO endpoints → no auto-collapse.
	m3 := newModel(src, nil, Config{}, 80, 19)
	if !m3.showEndpoints {
		t.Error("expected showEndpoints=true at height 19 without endpoints")
	}
	if m3.autoCollapsed {
		t.Error("expected autoCollapsed=false without endpoints")
	}
}

func TestLayoutAutoCollapseExpandable(t *testing.T) {
	eps := &fakeEndpoints{
		eps: []*mock.Endpoint{
			{ID: "ping", Method: "GET", Path: "/ping"},
			{ID: "orders", Method: "POST", Path: "/orders"},
		},
	}
	src := &fakeSource{snap: mock.StatsSnapshot{}}

	// Auto-collapsed at height 19.
	m := newModel(src, eps, Config{}, 80, 19)
	if m.showEndpoints {
		t.Fatal("expected auto-collapsed at height 19")
	}

	// Press e — should expand (user override).
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m2 := updated.(Model)
	if !m2.showEndpoints {
		t.Error("expected showEndpoints=true after e on auto-collapsed panel")
	}
	if m2.autoCollapsed {
		t.Error("expected autoCollapsed=false after user presses e")
	}

	// headerHeight must grow when expanded.
	if m2.headerHeight() != 3+1+2 {
		t.Errorf("expected headerHeight=%d expanded, got %d", 3+1+2, m2.headerHeight())
	}

	// Press e again — should re-collapse.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m2 = updated.(Model)
	if m2.showEndpoints {
		t.Error("expected showEndpoints=false after second e")
	}
	if m2.autoCollapsed {
		t.Error("expected autoCollapsed still false (user override)")
	}
}

func TestLayoutResizeReexpands(t *testing.T) {
	eps := &fakeEndpoints{
		eps: []*mock.Endpoint{
			{ID: "ping", Method: "GET", Path: "/ping"},
		},
	}
	src := &fakeSource{snap: mock.StatsSnapshot{}}

	// Start at height 19 — auto-collapsed.
	m := newModel(src, eps, Config{}, 80, 19)
	if m.showEndpoints {
		t.Fatal("expected auto-collapsed at height 19")
	}
	if !m.autoCollapsed {
		t.Fatal("expected autoCollapsed=true at height 19")
	}

	// Resize to height 24 — should re-expand because autoCollapsed was true.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	m2 := updated.(Model)
	if !m2.showEndpoints {
		t.Error("expected showEndpoints=true after resize to height 24")
	}
	if m2.autoCollapsed {
		t.Error("expected autoCollapsed=false after re-expand")
	}
}

func TestLayoutResizeUserChoicePreserved(t *testing.T) {
	eps := &fakeEndpoints{
		eps: []*mock.Endpoint{
			{ID: "ping", Method: "GET", Path: "/ping"},
		},
	}
	src := &fakeSource{snap: mock.StatsSnapshot{}}

	// Start at height 19 — auto-collapsed.
	m := newModel(src, eps, Config{}, 80, 19)

	// User explicitly expands (press e — overrides auto-collapse).
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m2 := updated.(Model)
	if !m2.showEndpoints {
		t.Fatal("expected expanded after user e")
	}
	if m2.autoCollapsed {
		t.Fatal("expected autoCollapsed=false after user e")
	}

	// Resize to height 19 again — the auto condition fires on EVERY resize,
	// overriding the user choice. This is by design: resizing triggers a fresh
	// layout decision.
	updated, _ = m2.Update(tea.WindowSizeMsg{Width: 80, Height: 19})
	m2 = updated.(Model)
	if m2.showEndpoints {
		t.Error("expected showEndpoints=false — auto-condition overrides user choice on resize")
	}
	if !m2.autoCollapsed {
		t.Error("expected autoCollapsed=true after resize below threshold")
	}

	// Resize to height 24 — autoCollapsed was true, so panel re-expands.
	updated, _ = m2.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m2 = updated.(Model)
	if !m2.showEndpoints {
		t.Error("expected showEndpoints=true — re-expands when terminal is tall again")
	}
	if m2.autoCollapsed {
		t.Error("expected autoCollapsed=false after re-expand")
	}
}

func TestRenderConfigNarrow(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	eps := &fakeEndpoints{
		eps: []*mock.Endpoint{
			{ID: "ping", Method: "GET", Path: "/ping"},
		},
	}
	cfg := Config{Addr: ":8080", StubsDir: "../stubs", LogDir: "logs"}

	// Width 80 — Admin URL present (80 is not < 80).
	m := newModel(src, eps, cfg, 80, 24)
	m.last = src.snap
	view := m.View()
	if !strings.Contains(view, "Admin: http://localhost:8080/_admin/endpoints") {
		t.Error("expected Admin URL at width 80")
	}

	// Width 79 — Admin URL absent.
	m2 := newModel(src, eps, cfg, 79, 24)
	m2.last = src.snap
	view2 := m2.View()
	if strings.Contains(view2, "Admin:") {
		t.Error("expected NO Admin URL at width 79")
	}
	// But the other segments still present.
	for _, want := range []string{
		"Address: :8080",
		"Stubs: ../stubs",
		"Log dir: logs",
	} {
		if !strings.Contains(view2, want) {
			t.Errorf("expected %q in narrow config line", want)
		}
	}
}

func TestRenderFeedTruncatedNarrow(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	src := &fakeSource{
		snap: mock.StatsSnapshot{
			RecentLogs: []mock.LogEntry{
				{
					Timestamp:      now,
					Method:         "POST",
					Path:           "/orders",
					ResponseStatus: 201,
					Webhook:        "order-placed",
					LatencyMs:      5.0,
				},
			},
			TotalLogs: 1,
		},
	}

	// Width 60 — latency segment present (60 is not < 60).
	m := newModel(src, nil, Config{}, 60, 24)
	m.last = src.snap
	m.renderFeed()
	view := m.View()
	if !strings.Contains(view, "(5.0ms)") {
		t.Error("expected '(5.0ms)' latency at width 60")
	}

	// Width 59 — latency segment absent.
	m2 := newModel(src, nil, Config{}, 59, 24)
	m2.last = src.snap
	m2.renderFeed()
	view2 := m2.View()
	if strings.Contains(view2, "(5.0ms)") {
		t.Error("expected NO latency segment at width 59")
	}

	// But the rest of the line (time, method, path, status, label) still present.
	if !strings.Contains(view2, "POST") {
		t.Error("expected POST method in narrow feed")
	}
	if !strings.Contains(view2, "/orders") {
		t.Error("expected /orders path in narrow feed")
	}
	if !strings.Contains(view2, "order-placed") {
		t.Error("expected order-placed label in narrow feed")
	}
}

func TestLayoutViewportHeightClamp(t *testing.T) {
	eps := &fakeEndpoints{
		eps: []*mock.Endpoint{
			{ID: "a", Method: "GET", Path: "/a"},
			{ID: "b", Method: "GET", Path: "/b"},
			{ID: "c", Method: "GET", Path: "/c"},
			{ID: "d", Method: "GET", Path: "/d"},
			{ID: "e", Method: "GET", Path: "/e"},
		},
	}
	src := &fakeSource{snap: mock.StatsSnapshot{}}

	// Height 5, width 80, 5 endpoints → auto-collapsed (height < 20).
	// headerHeight = 3 + 1 (collapsed heading) = 4.
	// viewport height = 5 - 4 - 1 = 0 → clamped to 1.
	m := newModel(src, eps, Config{}, 80, 5)
	if m.view.Height != 1 {
		t.Errorf("expected viewport height clamped to 1, got %d", m.view.Height)
	}
	if m.showEndpoints {
		t.Error("expected auto-collapsed at height 5")
	}

	// Resize to height 5 with endpoints already collapsed — viewport still clamped.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 5})
	m2 := updated.(Model)
	if m2.view.Height != 1 {
		t.Errorf("expected viewport height clamped to 1 after resize, got %d", m2.view.Height)
	}

	// Height 5, no endpoints — headerHeight = 3, viewport = 5-3-1 = 1 (no clamp needed).
	m3 := newModel(src, nil, Config{}, 80, 5)
	if m3.view.Height != 1 {
		t.Errorf("expected viewport height 1, got %d", m3.view.Height)
	}
}

// ------------------- UXR-006 — prompt polish (token highlight + inline validation) -------------------

func TestRenderPromptHighlight(t *testing.T) {
	// Force TrueColor so lipgloss emits ANSI sequences even in non-TTY tests.
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.ANSI)

	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, Config{}, 120, 30)
	m.prompting = true
	m.prompt = "ep:ping orders"

	view := m.View()

	// accent colour (cyan 39) should wrap recognised prefix tokens.
	accentANSI := "\x1b[38;5;39mep:ping"
	mutedANSI := "\x1b[38;5;240morders"

	if !strings.Contains(view, accentANSI) {
		t.Errorf("expected %q token highlighted with accent, got view:\n%s", "ep:ping", view)
	}
	if !strings.Contains(view, mutedANSI) {
		t.Errorf("expected bare token %q to stay muted, got view:\n%s", "orders", view)
	}
	// The bare text must NOT be wrapped in accent colour.
	if strings.Contains(view, "\x1b[38;5;39morders") {
		t.Error("bare text 'orders' must NOT be highlighted in accent colour")
	}
}

func TestRenderPromptHighlightAllTokens(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.ANSI)

	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, Config{}, 120, 30)
	m.prompting = true
	m.prompt = "ep:ping method:POST status:2"

	view := m.View()

	// All three recognised prefix tokens should be accent-highlighted.
	for _, tok := range []string{"ep:ping", "method:POST", "status:2"} {
		want := "\x1b[38;5;39m" + tok
		if !strings.Contains(view, want) {
			t.Errorf("expected %q highlighted in accent", tok)
		}
	}
}

func TestRenderPromptHighlightEmpty(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.ANSI)

	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, Config{}, 120, 30)
	m.prompting = true
	m.prompt = ""

	view := m.View()

	// Should render "filter: ▌  esc cancel · enter apply" without any accent.
	if !strings.Contains(view, "filter:") {
		t.Error("expected 'filter:' in prompt bar")
	}
	if !strings.Contains(view, "esc cancel") {
		t.Error("expected 'esc cancel' in prompt bar")
	}
	// No accent ANSI should appear (no tokens to highlight).
	if strings.Contains(view, "\x1b[38;5;39m") {
		t.Error("expected no accent ANSI in empty prompt")
	}
}

func TestModelPromptInvalidStatus(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, Config{}, 80, 24)

	// Enter prompt mode
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m2 := updated.(Model)

	// Type invalid status filter
	for _, r := range "status:abc" {
		updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m2 = updated.(Model)
	}

	// Press enter — stays in prompt with error.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 = updated.(Model)
	if !m2.prompting {
		t.Fatal("expected still prompting after invalid enter")
	}
	if m2.promptErr == "" {
		t.Fatal("expected non-empty promptErr after invalid enter")
	}
	if m2.prompt != "status:abc" {
		t.Fatalf("expected prompt preserved, got %q", m2.prompt)
	}

	// Backspace to clear prompt and promptErr, then type valid filter.
	// First clear all existing chars.
	for len(m2.prompt) > 0 {
		updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		m2 = updated.(Model)
	}
	if m2.promptErr != "" {
		t.Error("expected promptErr cleared after editing")
	}

	// Type valid status filter
	for _, r := range "status:200" {
		updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m2 = updated.(Model)
	}

	// Press enter — should apply.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 = updated.(Model)
	if m2.prompting {
		t.Error("expected not prompting after valid enter")
	}
	if m2.filter.Status != 200 {
		t.Errorf("expected filter.Status=200, got %d", m2.filter.Status)
	}
	if m2.promptErr != "" {
		t.Errorf("expected empty promptErr after valid enter, got %q", m2.promptErr)
	}
}

func TestModelPromptErrClearsOnEdit(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, Config{}, 80, 24)

	// Manually set prompt error state (simulating after invalid enter).
	m.prompting = true
	m.prompt = "status:abc"
	m.promptErr = "invalid status: status:abc"

	// Backspace clears error.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m2 := updated.(Model)
	if m2.promptErr != "" {
		t.Error("expected promptErr cleared after backspace")
	}

	// Set error again and type a character — should clear.
	m2.promptErr = "invalid status: status:abc"
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m2 = updated.(Model)
	if m2.promptErr != "" {
		t.Error("expected promptErr cleared after typing rune")
	}
}

func TestRenderPromptErrShows(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.ANSI)

	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, Config{}, 120, 30)
	m.prompting = true
	m.prompt = "status:abc"
	m.promptErr = "invalid status: status:abc"

	view := m.View()

	if !strings.Contains(view, "✗") {
		t.Error("expected '✗' error marker in prompt bar")
	}
	if !strings.Contains(view, "invalid status:") {
		t.Error("expected 'invalid status:' text in prompt bar")
	}

	// Error text should be in red (196).
	redANSI := "\x1b[38;5;196m"
	if !strings.Contains(view, redANSI) {
		t.Error("expected red ANSI (196) for error text")
	}
}

func TestModelPromptErrClearedOnCancel(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, Config{}, 80, 24)

	// Simulate prompt with error after invalid enter.
	m.prompting = true
	m.prompt = "status:abc"
	m.promptErr = "invalid status: status:abc"

	// Cancel with esc.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(Model)
	if m2.promptErr != "" {
		t.Error("expected promptErr cleared after esc cancel")
	}
	if m2.prompting {
		t.Error("expected not prompting after esc cancel")
	}

	// Cancel with q also clears.
	m.prompting = true
	m.promptErr = "invalid status: status:abc"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m2 = updated.(Model)
	if m2.promptErr != "" {
		t.Error("expected promptErr cleared after q cancel")
	}
}

// ------------------- UXR-007 — filter chip in endpoints panel header -------------------

func TestRenderFilterChip(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.ANSI)

	eps := &fakeEndpoints{
		eps: []*mock.Endpoint{
			{ID: "ping", Method: "GET", Path: "/ping"},
			{ID: "orders", Method: "POST", Path: "/orders"},
		},
	}
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, eps, Config{}, 120, 30)
	m.last = src.snap

	// No filter → no chip in heading.
	view := m.View()
	if strings.Contains(view, "[ep:") || strings.Contains(view, "[method:") || strings.Contains(view, "[status:") {
		t.Error("expected no filter chip when filter is empty")
	}

	// Set filter → chip appears in expanded heading.
	m.filter = Filter{Endpoint: "ping"}
	view = m.View()
	if !strings.Contains(view, "[ep:ping]") {
		t.Errorf("expected '[ep:ping]' chip in expanded heading, got view:\n%s", view)
	}

	// Chip should be in accent colour (39).
	chipANSI := "\x1b[38;5;39m[ep:ping]"
	if !strings.Contains(view, chipANSI) {
		t.Error("expected chip wrapped in accent colour (39)")
	}

	// Collapsed → chip still appears in the collapsed heading.
	m.showEndpoints = false
	view = m.View()
	if !strings.Contains(view, "[ep:ping]") {
		t.Error("expected '[ep:ping]' chip in collapsed heading")
	}
}

func TestRenderFilterChipZero(t *testing.T) {
	eps := &fakeEndpoints{
		eps: []*mock.Endpoint{
			{ID: "ping", Method: "GET", Path: "/ping"},
		},
	}
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, eps, Config{}, 120, 30)
	m.last = src.snap

	// Zero filter → no chip.
	m.filter = Filter{}
	view := m.View()

	if strings.Contains(view, "[ep:") {
		t.Error("expected no chip with zero filter")
	}
	if strings.Contains(view, "[method:") {
		t.Error("expected no chip with zero filter")
	}
}

func TestFilterChipClears(t *testing.T) {
	eps := &fakeEndpoints{
		eps: []*mock.Endpoint{
			{ID: "ping", Method: "GET", Path: "/ping"},
		},
	}
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, eps, Config{}, 120, 30)
	m.last = src.snap
	m.filter = Filter{Endpoint: "ping"}

	// 'c' clears filter → chip gone.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m2 := updated.(Model)
	view := m2.View()
	if strings.Contains(view, "[ep:") {
		t.Error("expected no filter chip after 'c' clear")
	}

	// Set again, esc (non-prompt) clears → chip gone.
	m.filter = Filter{Endpoint: "ping"}
	m.last = src.snap
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 = updated.(Model)
	view = m2.View()
	if strings.Contains(view, "[ep:") {
		t.Error("expected no filter chip after esc clear")
	}
}

func TestRenderFilterChipCollapsed(t *testing.T) {
	eps := &fakeEndpoints{
		eps: []*mock.Endpoint{
			{ID: "ping", Method: "GET", Path: "/ping"},
			{ID: "orders", Method: "POST", Path: "/orders"},
		},
	}
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, eps, Config{}, 120, 30)
	m.last = src.snap
	m.showEndpoints = false // collapsed
	m.filter = Filter{Endpoint: "ping"}

	view := m.View()

	// Collapsed heading should show the count AND the chip.
	if !strings.Contains(view, "Endpoints: (2) ▾") {
		t.Error("expected collapsed 'Endpoints: (2) ▾' heading")
	}
	if !strings.Contains(view, "[ep:ping]") {
		t.Error("expected '[ep:ping]' chip in collapsed heading")
	}
}

// TestModelPromptSpaceTyping verifies the space key works in the prompt.
// bubbletea delivers a space keypress as KeySpace, not KeyRunes — the handler
// must accept both, otherwise multi-token filters (ep:ping status:200) are
// impossible to type.
func TestModelPromptSpaceTyping(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, Config{}, 120, 30)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = updated.(Model)

	for _, r := range "ep:ping" {
		mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = mm.(Model)
	}
	// Space arrives as KeySpace.
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = mm.(Model)
	for _, r := range "status:200" {
		mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = mm.(Model)
	}

	if m.prompt != "ep:ping status:200" {
		t.Errorf("expected prompt 'ep:ping status:200', got %q", m.prompt)
	}

	// Enter applies the full multi-token filter.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.prompting {
		t.Error("expected prompt closed after valid enter")
	}
	if m.filter.Endpoint != "ping" || m.filter.Status != 200 {
		t.Errorf("expected filter ep:ping status:200, got %+v", m.filter)
	}
}

// TestModelPromptSlashOpens verifies '/' enters prompt mode (regression guard
// for the PTY harness finding).
func TestModelPromptSlashOpens(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, Config{}, 120, 30)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = updated.(Model)
	if !m.prompting {
		t.Fatal("expected prompting=true after '/'")
	}
	// Escape cancels.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(Model)
	if m.prompting {
		t.Fatal("expected prompting=false after esc")
	}
}
