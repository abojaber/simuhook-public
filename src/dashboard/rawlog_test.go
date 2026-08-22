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

// fakeRawLogSource is a stub RawLogSource for testing.
type fakeRawLogSource struct {
	lines []string
}

func (f *fakeRawLogSource) RawLogLines() []string {
	return f.lines
}

// ------------------- renderRawLine tests -------------------

func TestRenderRawLine_Separator(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.ANSI)

	line := "--- [2026-08-22T10:00:00Z] conn 127.0.0.1:12345 rule /api=https://example.com ---"
	got := renderRawLine(line)
	// Should contain the gray (240) ANSI prefix.
	if !strings.Contains(got, "\x1b[38;5;240m") {
		t.Errorf("separator line should be styled with color 240, got: %s", got)
	}
	if !strings.Contains(got, line) {
		t.Errorf("styled line should contain original text")
	}
}

func TestRenderRawLine_Request(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.ANSI)

	line := ">>> GET /api/orders HTTP/1.1"
	got := renderRawLine(line)
	// Should contain the cyan (39) ANSI prefix.
	if !strings.Contains(got, "\x1b[38;5;39m") {
		t.Errorf("request line should be styled with color 39, got: %s", got)
	}
	if !strings.Contains(got, line) {
		t.Errorf("styled line should contain original text")
	}
}

func TestRenderRawLine_Response(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.ANSI)

	line := "<<< HTTP/1.1 200 OK"
	got := renderRawLine(line)
	// Should contain the green (76) ANSI prefix.
	if !strings.Contains(got, "\x1b[38;5;76m") {
		t.Errorf("response line should be styled with color 76, got: %s", got)
	}
	if !strings.Contains(got, line) {
		t.Errorf("styled line should contain original text")
	}
}

func TestRenderRawLine_Unknown(t *testing.T) {
	line := "some random log line"
	got := renderRawLine(line)
	// Should be returned unchanged (no ANSI styling).
	if got != line {
		t.Errorf("unknown prefix line should be unchanged, got: %s", got)
	}
}

// ------------------- View switching tests -------------------

func TestRawLogView_Toggle(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	rawSrc := &fakeRawLogSource{lines: []string{">>> GET /api HTTP/1.1"}}
	m := newModel(src, nil, rawSrc, Config{}, 80, 24)

	// Default: main dashboard view.
	if m.viewMode != 0 {
		t.Fatal("expected viewMode=0 by default")
	}

	// Press '2' — switch to proxy view.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m2 := updated.(Model)
	if m2.viewMode != 1 {
		t.Errorf("expected viewMode=1 after '2', got %d", m2.viewMode)
	}

	// Press '1' — switch back to main view.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m3 := updated.(Model)
	if m3.viewMode != 0 {
		t.Errorf("expected viewMode=0 after '1', got %d", m3.viewMode)
	}
}

func TestRawLogView_EscReturnsToMain(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	rawSrc := &fakeRawLogSource{lines: []string{">>> GET /api HTTP/1.1"}}
	m := newModel(src, nil, rawSrc, Config{}, 80, 24)

	// Switch to proxy view.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m2 := updated.(Model)
	if m2.viewMode != 1 {
		t.Fatal("expected viewMode=1")
	}

	// Press esc — should return to main view.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m3 := updated.(Model)
	if m3.viewMode != 0 {
		t.Errorf("expected viewMode=0 after esc, got %d", m3.viewMode)
	}
}

func TestRawLogView_QuitWorks(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	rawSrc := &fakeRawLogSource{lines: []string{">>> GET /api HTTP/1.1"}}
	m := newModel(src, nil, rawSrc, Config{}, 80, 24)

	// Switch to proxy view.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m2 := updated.(Model)

	// Press q — should quit.
	_, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("expected non-nil cmd (quit) from q in proxy view")
	}
}

// ------------------- Auto-scroll tests -------------------

func TestRawLogView_AutoScroll(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	rawSrc := &fakeRawLogSource{lines: []string{">>> GET /api HTTP/1.1"}}
	m := newModel(src, nil, rawSrc, Config{}, 80, 24)

	// Default: liveFollowRaw should be true.
	if !m.liveFollowRaw {
		t.Fatal("expected liveFollowRaw=true by default")
	}

	// Switch to proxy view.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m2 := updated.(Model)

	// Press L — should pause raw log follow.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	m3 := updated.(Model)
	if m3.liveFollowRaw {
		t.Error("expected liveFollowRaw=false after L in proxy view")
	}

	// Press L again — should resume.
	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	m4 := updated.(Model)
	if !m4.liveFollowRaw {
		t.Error("expected liveFollowRaw=true after second L in proxy view")
	}
}

// ------------------- Nil source tests -------------------

func TestRawLogView_NilSource(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, nil, Config{}, 80, 24)

	// Switch to proxy view with nil rawSrc and no recent logs.
	m.viewMode = 1
	view := m.View()
	if !strings.Contains(view, "No proxy rules configured") {
		t.Error("expected 'no proxy rules' message when rawSrc is nil and no logs")
	}
}

func TestRawLogView_NilSource_ViewSwitch(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, nil, Config{}, 80, 24)

	// Press '2' with nil rawSrc — should still switch to proxy view.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m2 := updated.(Model)
	if m2.viewMode != 1 {
		t.Errorf("expected viewMode=1, got %d", m2.viewMode)
	}

	// View should show the "no proxy rules" message.
	view := m2.View()
	if !strings.Contains(view, "No proxy rules configured") {
		t.Error("expected 'no proxy rules' message")
	}
}

// ------------------- Structured proxy entries -------------------

func TestProxyView_StructuredEntries(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.ANSI)

	now := time.Now().UTC().Format(time.RFC3339Nano)
	src := &fakeSource{snap: mock.StatsSnapshot{
		RecentLogs: []mock.LogEntry{
			{Timestamp: now, Method: "GET", Path: "/health", Webhook: "health-check", ResponseStatus: 200, LatencyMs: 1.2},
			{Timestamp: now, Method: "POST", Path: "/api/webhook", Webhook: "_proxy", ResponseStatus: 201, LatencyMs: 45.6},
			{Timestamp: now, Method: "GET", Path: "/api/orders", Webhook: "_proxy", ResponseStatus: 200, LatencyMs: 12.3},
		},
	}}
	rawSrc := &fakeRawLogSource{lines: []string{">>> POST /api/webhook HTTP/1.1"}}
	m := newModel(src, nil, rawSrc, Config{}, 80, 24)

	// Switch to proxy view first, then trigger a tick so buildProxyContent runs.
	m.viewMode = 1
	updated, _ := m.Update(tickMsg(time.Now()))
	m2 := updated.(Model)

	view := m2.View()

	// Should show "PROXY REQUESTS" header.
	if !strings.Contains(view, "PROXY REQUESTS") {
		t.Error("expected 'PROXY REQUESTS' in header")
	}

	// Should show only _proxy entries (not health-check).
	if !strings.Contains(view, "/api/webhook") {
		t.Error("expected /api/webhook in proxy view")
	}
	if !strings.Contains(view, "/api/orders") {
		t.Error("expected /api/orders in proxy view")
	}
	if strings.Contains(view, "/health") {
		t.Error("should NOT show non-proxy entry /health")
	}

	// Should show raw bytes section.
	if !strings.Contains(view, "Raw Bytes") {
		t.Error("expected 'Raw Bytes' section separator")
	}
	if !strings.Contains(view, ">>> POST /api/webhook HTTP/1.1") {
		t.Error("expected raw line in view")
	}

	// Footer should show counts.
	if !strings.Contains(view, "2 requests") {
		t.Error("expected '2 requests' in footer")
	}
	if !strings.Contains(view, "1 raw lines") {
		t.Error("expected '1 raw lines' in footer")
	}
}

func TestProxyView_NoProxyEntries(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{
		RecentLogs: []mock.LogEntry{
			{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Method: "GET", Path: "/health", Webhook: "health-check", ResponseStatus: 200},
		},
	}}
	rawSrc := &fakeRawLogSource{lines: []string{">>> GET /api HTTP/1.1"}}
	m := newModel(src, nil, rawSrc, Config{}, 80, 24)

	// Switch to proxy view first, then trigger tick.
	m.viewMode = 1
	updated, _ := m.Update(tickMsg(time.Now()))
	m2 := updated.(Model)

	view := m2.View()

	// Should show "no proxy requests yet" placeholder.
	if !strings.Contains(view, "(no proxy requests yet)") {
		t.Error("expected '(no proxy requests yet)' when no _proxy entries exist")
	}

	// Should still show raw bytes section.
	if !strings.Contains(view, "Raw Bytes") {
		t.Error("expected 'Raw Bytes' section even with no proxy entries")
	}

	// Footer should show 0 requests.
	if !strings.Contains(view, "0 requests") {
		t.Error("expected '0 requests' in footer")
	}
}

func TestProxyView_EmptyRawLines(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	rawSrc := &fakeRawLogSource{lines: []string{}}
	m := newModel(src, nil, rawSrc, Config{}, 80, 24)

	// Switch to proxy view first, then trigger tick.
	m.viewMode = 1
	updated, _ := m.Update(tickMsg(time.Now()))
	m2 := updated.(Model)

	view := m2.View()

	if !strings.Contains(view, "0 requests") {
		t.Error("expected '0 requests' in footer")
	}
	if !strings.Contains(view, "0 raw lines") {
		t.Error("expected '0 raw lines' in footer")
	}
}

func TestProxyView_FiltersOnlyProxyWebhook(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	src := &fakeSource{snap: mock.StatsSnapshot{
		RecentLogs: []mock.LogEntry{
			{Timestamp: now, Method: "GET", Path: "/a", Webhook: "_proxy", ResponseStatus: 200},
			{Timestamp: now, Method: "POST", Path: "/b", Webhook: "stripe", ResponseStatus: 201},
			{Timestamp: now, Method: "DELETE", Path: "/c", Webhook: "_proxy", ResponseStatus: 204},
			{Timestamp: now, Method: "PUT", Path: "/d", Webhook: "github", ResponseStatus: 200},
		},
	}}
	rawSrc := &fakeRawLogSource{lines: []string{}}
	m := newModel(src, nil, rawSrc, Config{}, 80, 24)

	// Trigger tick.
	updated, _ := m.Update(tickMsg(time.Now()))
	m2 := updated.(Model)

	entries := m2.proxyEntries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 proxy entries, got %d", len(entries))
	}
	if entries[0].Path != "/a" || entries[1].Path != "/c" {
		t.Errorf("expected /a and /c, got %s and %s", entries[0].Path, entries[1].Path)
	}
}

func TestProxyView_NilSourceWithProxyLogs(t *testing.T) {
	// When rawSrc is nil but there ARE proxy log entries, the view should
	// still render (not show the "no proxy rules" message).
	now := time.Now().UTC().Format(time.RFC3339Nano)
	src := &fakeSource{snap: mock.StatsSnapshot{
		RecentLogs: []mock.LogEntry{
			{Timestamp: now, Method: "POST", Path: "/webhook", Webhook: "_proxy", ResponseStatus: 200},
		},
	}}
	m := newModel(src, nil, nil, Config{}, 80, 24)

	// Switch to proxy view first, then trigger tick.
	m.viewMode = 1
	updated, _ := m.Update(tickMsg(time.Now()))
	m2 := updated.(Model)

	view := m2.View()

	// Should NOT show "no proxy rules" — there are proxy entries.
	if strings.Contains(view, "No proxy rules configured") {
		t.Error("should not show 'no proxy rules' when proxy entries exist")
	}
	// Should show the structured entry.
	if !strings.Contains(view, "/webhook") {
		t.Error("expected /webhook in proxy view")
	}
}

// ------------------- Tick updates proxy view -------------------

func TestRawLogView_TickUpdates(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	rawSrc := &fakeRawLogSource{
		lines: []string{">>> GET /api HTTP/1.1"},
	}
	m := newModel(src, nil, rawSrc, Config{}, 80, 24)
	m.viewMode = 1

	// Simulate new lines arriving.
	rawSrc.lines = []string{
		">>> GET /api HTTP/1.1",
		"<<< HTTP/1.1 200 OK",
		">>> POST /api/orders HTTP/1.1",
	}

	// Send a tick.
	updated, _ := m.Update(tickMsg(time.Now()))
	m2 := updated.(Model)

	// The proxy view should have been updated.
	if m2.viewMode != 1 {
		t.Error("expected viewMode to remain 1 after tick")
	}
}

// ------------------- Key forwarding in proxy view -------------------

func TestRawLogView_KeysForwardedToViewport(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	rawSrc := &fakeRawLogSource{
		lines: []string{">>> GET /api HTTP/1.1"},
	}
	m := newModel(src, nil, rawSrc, Config{}, 80, 24)
	m.viewMode = 1

	// Press 'r' — should NOT trigger main view refresh (forwarded to viewport).
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m2 := updated.(Model)
	if m2.viewMode != 1 {
		t.Error("expected viewMode to remain 1 after 'r' key")
	}
	_ = cmd
}

func TestRawLogView_FilterKeysNotActive(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	rawSrc := &fakeRawLogSource{
		lines: []string{">>> GET /api HTTP/1.1"},
	}
	m := newModel(src, nil, rawSrc, Config{}, 80, 24)
	m.viewMode = 1

	// Press '/' — should NOT enter prompt mode in proxy view.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m2 := updated.(Model)
	if m2.prompting {
		t.Error("expected prompting=false — '/' should not enter prompt in proxy view")
	}
	if m2.viewMode != 1 {
		t.Error("expected viewMode to remain 1")
	}
}

// ------------------- Help in proxy view -------------------

func TestRawLogView_HelpToggle(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	rawSrc := &fakeRawLogSource{
		lines: []string{">>> GET /api HTTP/1.1"},
	}
	m := newModel(src, nil, rawSrc, Config{}, 80, 24)
	m.viewMode = 1

	// Press '?' — should toggle help.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m2 := updated.(Model)
	if !m2.showHelp {
		t.Error("expected showHelp=true after '?' in proxy view")
	}

	// Press '?' again — should toggle off.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m3 := updated.(Model)
	if m3.showHelp {
		t.Error("expected showHelp=false after second '?'")
	}
}

// ------------------- Help contains new bindings -------------------

func TestHelpContainsViewSwitchBindings(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	m := newModel(src, nil, nil, Config{}, 80, 24)
	m.showHelp = true
	view := m.View()
	if !strings.Contains(view, "1/2") {
		t.Error("expected '1/2' view switch binding in help")
	}
	if !strings.Contains(view, "switch view") {
		t.Error("expected 'switch view' in help")
	}
	if !strings.Contains(view, "main/proxy") {
		t.Error("expected 'main/proxy' in help text")
	}
}

// ------------------- WindowSizeMsg in proxy view -------------------

func TestRawLogView_WindowResize(t *testing.T) {
	src := &fakeSource{snap: mock.StatsSnapshot{}}
	rawSrc := &fakeRawLogSource{
		lines: []string{">>> GET /api HTTP/1.1"},
	}
	m := newModel(src, nil, rawSrc, Config{}, 80, 24)
	m.viewMode = 1

	// Resize.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m2 := updated.(Model)

	if m2.width != 120 {
		t.Errorf("expected width=120, got %d", m2.width)
	}
	if m2.height != 40 {
		t.Errorf("expected height=40, got %d", m2.height)
	}
	// Raw log viewport should be resized.
	if m2.rawLogView.Width != 120 {
		t.Errorf("expected rawLogView.Width=120, got %d", m2.rawLogView.Width)
	}
	if m2.rawLogView.Height != 37 {
		t.Errorf("expected rawLogView.Height=37 (40-3), got %d", m2.rawLogView.Height)
	}
}

// ------------------- Raw bytes section -------------------

func TestProxyView_RawBytesSection(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.ANSI)

	src := &fakeSource{snap: mock.StatsSnapshot{}}
	rawSrc := &fakeRawLogSource{
		lines: []string{
			"--- separator ---",
			">>> GET /api HTTP/1.1",
			"<<< HTTP/1.1 200 OK",
		},
	}
	m := newModel(src, nil, rawSrc, Config{}, 80, 24)

	// Switch to proxy view first, then trigger tick.
	m.viewMode = 1
	updated, _ := m.Update(tickMsg(time.Now()))
	m2 := updated.(Model)

	view := m2.View()

	// Verify raw bytes section exists with all line types.
	if !strings.Contains(view, "Raw Bytes") {
		t.Error("expected 'Raw Bytes' section separator")
	}
	if !strings.Contains(view, "--- separator ---") {
		t.Error("expected separator line in view")
	}
	if !strings.Contains(view, ">>> GET /api HTTP/1.1") {
		t.Error("expected request line in view")
	}
	if !strings.Contains(view, "<<< HTTP/1.1 200 OK") {
		t.Error("expected response line in view")
	}
	if !strings.Contains(view, "3 raw lines") {
		t.Error("expected '3 raw lines' in footer")
	}
}

// ------------------- proxyEntries helper -------------------

func TestProxyEntries_Empty(t *testing.T) {
	m := Model{last: mock.StatsSnapshot{}}
	entries := m.proxyEntries()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestProxyEntries_Mixed(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	m := Model{last: mock.StatsSnapshot{
		RecentLogs: []mock.LogEntry{
			{Webhook: "stripe", Path: "/a"},
			{Webhook: "_proxy", Path: "/b"},
			{Webhook: "_proxy", Path: "/c"},
			{Webhook: "github", Path: "/d"},
		},
	}}
	entries := m.proxyEntries()
	if len(entries) != 2 {
		t.Fatalf("expected 2, got %d", len(entries))
	}
	if entries[0].Path != "/b" || entries[1].Path != "/c" {
		t.Errorf("unexpected paths: %s, %s", entries[0].Path, entries[1].Path)
	}
	_ = now
}
