package dashboard

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

	// Press enter — invalid filter, should be ignored (ok=false)
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 = updated.(Model)
	if m2.filter != (Filter{}) {
		t.Error("expected unchanged filter when status:abc is invalid")
	}
	if m2.prompting {
		t.Error("expected not prompting after enter with invalid filter")
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
