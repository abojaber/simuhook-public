package dashboard

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"simuhook/mock"
)

// Package-level lipgloss styles.
var (
	styleHeader = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	styleMuted  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleAccent = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	styleError  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	// UXR-008 — feed row expansion styles.
	styleSelBG     = lipgloss.NewStyle().Background(lipgloss.Color("236"))
	styleSelMarker = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("212")).Bold(true)
)

// ------------------- UXR-001 helpers -------------------

// statusColor returns the lipgloss colour for a given HTTP status code by class:
// 2xx → green, 3xx → cyan, 4xx → yellow, 5xx → red, other → muted.
func statusColor(status int) lipgloss.Color {
	switch {
	case status >= 200 && status < 300:
		return lipgloss.Color("76")
	case status >= 300 && status < 400:
		return lipgloss.Color("39")
	case status >= 400 && status < 500:
		return lipgloss.Color("220")
	case status >= 500 && status < 600:
		return lipgloss.Color("196")
	default:
		return lipgloss.Color("240")
	}
}

// tint returns the status code string wrapped in a lipgloss colour by HTTP class.
// Delegates to statusColor so rendering is byte-identical with the extracted helper.
func tint(status int) string {
	return lipgloss.NewStyle().Foreground(statusColor(status)).Render(fmt.Sprintf("%d", status))
}

// ------------------- UXR-008 helpers -------------------

// sanitize replaces control characters (including \n, \r, \t) and DEL with a space.
// This prevents terminal corruption from user-supplied data in the feed.
func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r == 0x7f || r < 0x20 {
			return ' '
		}
		return r
	}, s)
}

// truncPlain truncates a plain string to at most w visible cells, appending "…"
// when truncation occurs. Width 0 returns ""; width 1 returns "…".
func truncPlain(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if used+rw > w-1 {
			break
		}
		b.WriteRune(r)
		used += rw
	}
	return b.String() + "…"
}

// feedRender bundles the result of building feed content.
type feedRender struct {
	content     string          // ready for viewport.SetContent
	entries     []mock.LogEntry // displayed (filtered) entries
	lineToEntry []int           // content line -> index into entries, or -1
	entryToLine []int           // entries index -> content line of its summary line
	expandedHit bool            // true if m.expandedID was found in entries
}

// ------------------- UXR-007 helper -------------------

// endpointHeading builds the endpoints panel heading line. When a filter is
// active, a chip (e.g. [ep:ping]) is appended in the accent colour.
func (m Model) endpointHeading(base string) string {
	if m.filter != (Filter{}) {
		return styleMuted.Render(base) + "  " + styleAccent.Render("["+m.filter.String()+"]")
	}
	return styleMuted.Render(base)
}

// View renders the full dashboard.
func (m Model) View() string {
	header := m.renderHeader()
	vp := m.view
	if sel := m.selectedIdx(); sel >= 0 {
		vp.SetContent(m.buildFeed(sel).content)
	}
	feed := vp.View()

	var footerBuilder strings.Builder
	if m.prompting {
		footerBuilder.WriteString(m.renderPromptBar())
	} else {
		footerBuilder.WriteString(m.renderFooter())
	}

	parts := []string{header, feed, footerBuilder.String()}

	if m.showHelp && !m.prompting {
		parts = append(parts, m.renderHelp())
	}

	return lipgloss.JoinVertical(lipgloss.Top, parts...)
}

// renderHeader builds the dashboard header: title, server config, live stats,
// and the list of registered endpoints.
func (m Model) renderHeader() string {
	title := styleHeader.Render("Simuhook Dashboard")

	// Config line (from startup banner).
	adminURL := "http://localhost" + m.cfg.Addr + "/_admin/endpoints"

	var configLine string
	if m.width < 80 {
		// UXR-005 — narrow terminals: drop Admin URL.
		configLine = fmt.Sprintf("Address: %s  Stubs: %s (%d files)  Log dir: %s",
			m.cfg.Addr, m.cfg.StubsDir, len(m.endpoints), m.cfg.LogDir)
	} else {
		configLine = fmt.Sprintf("Address: %s  Stubs: %s (%d files)  Log dir: %s  Admin: %s",
			m.cfg.Addr, m.cfg.StubsDir, len(m.endpoints), m.cfg.LogDir, adminURL)
	}
	config := styleMuted.Render(configLine)

	// Stats line
	uptime := m.last.Uptime.Truncate(time.Second).String()
	total := m.last.TotalRequests
	lat := m.last.Latency

	var statsLine string
	if lat.Count > 0 {
		statsLine = fmt.Sprintf("Uptime: %s  Requests: %d  Latency: avg=%.1fms min=%.1fms max=%.1fms p95=%.1fms",
			uptime, total, lat.Avg(), lat.Min, lat.Max, lat.P95)
	} else {
		statsLine = fmt.Sprintf("Uptime: %s  Requests: %d  Latency: (no requests yet)", uptime, total)
	}
	stats := styleMuted.Render(statsLine)

	parts := []string{title, config, stats}

	// Endpoints list
	if len(m.endpoints) > 0 {
		if m.showEndpoints {
			headingLine := m.endpointHeading("Endpoints:")
			epParts := make([]string, 0, len(m.endpoints))
			for _, ep := range m.endpoints {
				method := ep.Method
				if method == "" {
					method = "ANY "
				}
				left := fmt.Sprintf("    %-5s %s → %-20s", method, ep.Path, ep.ID)

				// UXR-002 — per-endpoint request count + p95
				count := m.last.HitCounts[ep.ID]
				epLat := m.last.EndpointLatency[ep.ID]
				if epLat.Count > 0 {
					left += fmt.Sprintf("  requests: %d  p95: %.1fms", count, epLat.P95)
				} else {
					left += fmt.Sprintf("  requests: %d", count)
				}

				epParts = append(epParts, left)
			}
			endpoints := headingLine + "\n" + styleMuted.Render(strings.Join(epParts, "\n"))
			parts = append(parts, endpoints)
		} else {
			// UXR-004 — collapsed endpoints panel: single "Endpoints: (N) ▾" line.
			// UXR-007 — filter chip appended when active.
			collapsed := m.endpointHeading(fmt.Sprintf("Endpoints: (%d) ▾", len(m.endpoints)))
			parts = append(parts, collapsed)
		}
	}

	return lipgloss.JoinVertical(lipgloss.Top, parts...)
}

// renderFooter builds the 1-line footer.
func (m Model) renderFooter() string {
	total := uint64(len(m.last.RecentLogs))
	totalDisplayed := total
	if m.filter != (Filter{}) {
		// Count filtered entries
		var count uint64
		for _, e := range m.last.RecentLogs {
			if m.filter.Matches(e) {
				count++
			}
		}
		totalDisplayed = count
	}

	footer := fmt.Sprintf("showing %d of %d recent · ", totalDisplayed, total)

	// UXR-003 — live-follow status indicator.
	if m.liveFollow {
		footer += "▶ live · "
	} else {
		footer += "⏸ live · "
	}

	footer += "q quit · r refresh · enter detail"

	if m.filter != (Filter{}) {
		footer += " · filter: " + m.filter.String()
	}

	return styleMuted.Render(footer)
}

// renderPromptBar builds the prompt line shown when the user is typing a filter.
// UXR-006 — recognised prefixes (ep:, method:, status:) are highlighted in the
// accent colour; bare text stays muted. An inline ✗ error is shown when the
// last parse failed, so the user can fix the input without leaving the prompt.
func (m Model) renderPromptBar() string {
	var parts []string
	parts = append(parts, styleMuted.Render("filter:"))

	// Tokenize and highlight each token.
	tokens := strings.Fields(m.prompt)
	for _, tok := range tokens {
		if strings.HasPrefix(tok, "ep:") || strings.HasPrefix(tok, "method:") || strings.HasPrefix(tok, "status:") {
			parts = append(parts, styleAccent.Render(tok))
		} else {
			parts = append(parts, styleMuted.Render(tok))
		}
	}

	result := strings.Join(parts, " ") + "▌"

	if m.promptErr != "" {
		result += "  " + styleError.Render("✗ "+m.promptErr)
	}

	result += "  " + styleMuted.Render("esc cancel · enter apply")
	return result
}

// renderHelp builds a help panel showing key bindings.
func (m Model) renderHelp() string {
	lines := []string{
		styleMuted.Render("╭── Key bindings ─────────────────────────────────────────────╮"),
		styleMuted.Render("│  / f     enter filter mode    esc     cancel / clear filter │"),
		styleMuted.Render("│  r       refresh now          ?       toggle this help      │"),
		styleMuted.Render("│  c       clear filter         q       quit                  │"),
		styleMuted.Render("│  L       pause/resume live    e       collapse endpoints    │"),
		styleMuted.Render("│  enter   expand/collapse row                                │"),
		styleMuted.Render("│  ↑/↓     scroll feed          pgup/dn page                  │"),
		styleMuted.Render("╰─────────────────────────────────────────────────────────────╯"),
	}
	return strings.Join(lines, "\n")
}

// summaryLine renders a single feed line for the given log entry. When sel is
// true the row uses the selection highlight with segment-wise background so the
// status tint's inner reset does not break the highlight.
func (m Model) summaryLine(e mock.LogEntry, sel bool) string {
	t, err := time.Parse(time.RFC3339Nano, e.Timestamp)
	timeStr := "??:??:??"
	if err == nil {
		timeStr = t.Format("15:04:05")
	}

	label := sanitize(e.Webhook)
	if label == "" {
		label = "unmatched"
	}
	path := sanitize(e.Path)

	// Build segments: pre (time+method+path→), status, post (label+latency).
	pre := fmt.Sprintf("%s  %-5s %s → ", timeStr, e.Method, path)
	statusStr := fmt.Sprintf("%d", e.ResponseStatus)
	post := "  " + label
	if m.width >= 60 {
		post += fmt.Sprintf("  (%.1fms)", e.LatencyMs)
	}

	if !sel {
		// Non-selected: 2-space gutter + same output as before (byte-identical
		// except for the 2 leading spaces).
		return "  " + pre + tint(e.ResponseStatus) + post
	}

	// Selected: segment-wise background so the highlight survives tint's inner
	// reset. Each segment carries its own Background.
	var sb strings.Builder
	sb.WriteString(styleSelMarker.Render("▍ "))
	sb.WriteString(styleSelBG.Render(pre))
	sb.WriteString(styleSelBG.Copy().Foreground(statusColor(e.ResponseStatus)).Render(statusStr))
	sb.WriteString(styleSelBG.Render(post))

	// Pad to viewport width so the highlight spans the full row.
	visibleWidth := lipgloss.Width("▍ ") + lipgloss.Width(pre) + lipgloss.Width(statusStr) + lipgloss.Width(post)
	if pad := m.view.Width - visibleWidth; pad > 0 {
		sb.WriteString(styleSelBG.Render(strings.Repeat(" ", pad)))
	}
	return sb.String()
}

// detailLines returns the expanded-detail lines for a log entry. The gutter is
// "  │ " and keys are sorted for determinism. Every line is truncated to
// m.view.Width and rendered in the muted style.
func (m Model) detailLines(e mock.LogEntry) []string {
	var plain []string

	// Fixed fields.
	plain = append(plain, fmt.Sprintf("%-12s %s", "request_id:", e.RequestID))
	matched := "false"
	if e.Matched {
		matched = "true"
	}
	plain = append(plain, fmt.Sprintf("%-12s %s", "matched:", matched))
	plain = append(plain, fmt.Sprintf("%-12s %s", "timestamp:", e.Timestamp))

	// Headers block.
	if len(e.Headers) == 0 {
		plain = append(plain, fmt.Sprintf("%-12s %s", "headers:", "(none)"))
	} else {
		plain = append(plain, "headers:")
		keys := make([]string, 0, len(e.Headers))
		for k := range e.Headers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			plain = append(plain, fmt.Sprintf("  %s: %s", sanitize(k), sanitize(e.Headers[k])))
		}
	}

	// Query block.
	if len(e.Query) == 0 {
		plain = append(plain, fmt.Sprintf("%-12s %s", "query:", "(none)"))
	} else {
		plain = append(plain, "query:")
		keys := make([]string, 0, len(e.Query))
		for k := range e.Query {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			plain = append(plain, fmt.Sprintf("  %s: %s", sanitize(k), sanitize(e.Query[k])))
		}
	}

	// Body (always one line — sanitize turns \n into spaces).
	if e.Body == "" {
		plain = append(plain, fmt.Sprintf("%-12s %s", "body:", "(empty)"))
	} else {
		body := sanitize(e.Body)
		plain = append(plain, fmt.Sprintf("%-12s %s", "body:", body))
	}

	// Apply gutter, truncation, and muted style to each line.
	// gutterWidth is the visible width of "  │ " (2 spaces + box char + space = 4).
	const gutterWidth = 4
	result := make([]string, len(plain))
	for i, line := range plain {
		truncated := truncPlain(line, m.view.Width-gutterWidth)
		result[i] = styleMuted.Render("  │ " + truncated)
	}
	return result
}

// buildFeed formats recent log entries into a feedRender for the viewport.
// Pure (value receiver, no mutation). When selected >= 0, the corresponding
// entry's summary line is rendered with highlight styling.
func (m Model) buildFeed(selected int) feedRender {
	if len(m.last.RecentLogs) == 0 {
		return feedRender{
			content:     styleMuted.Render("(no requests yet)"),
			lineToEntry: []int{-1},
		}
	}

	var sb strings.Builder
	var entries []mock.LogEntry
	var lineToEntry []int
	var entryToLine []int
	expandedHit := false
	anyMatched := false

	for _, e := range m.last.RecentLogs {
		if m.filter != (Filter{}) && !m.filter.Matches(e) {
			continue
		}
		anyMatched = true
		idx := len(entries)
		entries = append(entries, e)
		entryToLine = append(entryToLine, len(lineToEntry))

		sel := idx == selected
		sb.WriteString(m.summaryLine(e, sel))
		sb.WriteByte('\n')
		lineToEntry = append(lineToEntry, idx)

		if e.RequestID != "" && e.RequestID == m.expandedID {
			expandedHit = true
			for _, dl := range m.detailLines(e) {
				sb.WriteString(dl)
				sb.WriteByte('\n')
				lineToEntry = append(lineToEntry, idx)
			}
		}
	}

	if !anyMatched && m.filter != (Filter{}) {
		return feedRender{
			content:     styleMuted.Render("(no entries match filter)"),
			lineToEntry: []int{-1},
		}
	}

	// Trailing -1 accounts for the blank line produced by SetContent's
	// strings.Split on the final "\n".
	lineToEntry = append(lineToEntry, -1)

	return feedRender{
		content:     sb.String(),
		entries:     entries,
		lineToEntry: lineToEntry,
		entryToLine: entryToLine,
		expandedHit: expandedHit,
	}
}
