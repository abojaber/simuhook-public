package dashboard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"simuhook/mock"
)

// Package-level lipgloss styles.
var (
	styleHeader     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	styleMuted      = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleAccent     = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	styleError      = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	stylePauseBtn   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	styleResumeBtn  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220"))
	styleTableHead  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("240"))
	stylePanelTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	panelStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))

	// UXR-008 — feed row expansion styles.
	styleSelBG     = lipgloss.NewStyle().Background(lipgloss.Color("236"))
	styleSelMarker = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("212")).Bold(true)

	// WS-018 / UXR-013 — request/response inspector.
	styleSection = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("240"))
)

// WS-018 / UXR-013 — inspect formatting constants.
const (
	maxInspectBodyBytes = 4096
	maxInspectBodyLines = 40
	maxInspectJSONBytes = 262576
	// maxRenderedHeaders caps header/query line counts in both detailLines
	// (UXR-008) and inspectLines (UXR-013) — attacker-controlled maps (a
	// client can send hundreds of headers) must not inflate the feed block
	// or feedLineToEntry without bound.
	maxRenderedHeaders = 30
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

// View renders the full dashboard. Dispatches to wide or narrow layout.
func (m Model) View() string {
	if m.wide() {
		return m.viewWide()
	}
	return m.viewNarrow()
}

// viewNarrow renders the current (pre-Phase-6) vertical layout, byte-identical
// to the original View() body.
func (m Model) viewNarrow() string {
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
		footerBuilder.WriteString(truncPlain(m.renderFooter(), m.width))
	}

	parts := []string{header, feed, footerBuilder.String()}

	if m.showHelp && !m.prompting {
		parts = append(parts, m.renderHelp())
	}

	return lipgloss.JoinVertical(lipgloss.Top, parts...)
}

// ------------------- Wide layout -------------------

// viewWide renders the two-column wide layout.
func (m Model) viewWide() string {
	g := m.geometry()

	title := truncPlain(m.renderTitleRow(g.rightTotal), m.width)
	info := truncPlain(m.renderServerInfo(m.width), m.width)
	detail := truncPlain(m.renderInfoDetail(m.width), m.width)
	metrics := truncPlain(m.renderMetrics(m.width), m.width)

	panels := m.renderPanels(g)

	var footerBuilder strings.Builder
	if m.prompting {
		footerBuilder.WriteString(m.renderPromptBar())
	} else {
		footerBuilder.WriteString(truncPlain(m.renderFooter(), m.width))
	}

	parts := []string{title, info, detail, metrics, panels, footerBuilder.String()}

	if m.showHelp && !m.prompting {
		parts = append(parts, m.renderHelp())
	}

	return lipgloss.JoinVertical(lipgloss.Top, parts...)
}

// renderPanels builds the side-by-side endpoints + stream panels.
func (m Model) renderPanels(g geom) string {
	// Stream panel (right).
	streamPanel := m.renderStreamPanel(g)

	// Endpoints panel (left).
	if g.showLeft {
		endpointsPanel := m.renderEndpointsPanel(g)
		return lipgloss.JoinHorizontal(lipgloss.Top, endpointsPanel, streamPanel)
	}

	return streamPanel
}

// renderStreamPanel builds the right panel with the feed + header.
func (m Model) renderStreamPanel(g geom) string {
	title := m.renderStreamTitle(g)
	headers := m.renderStreamHeaders()

	vp := m.view
	if sel := m.selectedIdx(); sel >= 0 {
		vp.SetContent(m.buildFeed(sel).content)
	}
	vpLines := strings.Split(vp.View(), "\n")

	// Ensure exact row count — note: View() always returns Height lines
	// truncated/padded, but let's pad just in case.
	for len(vpLines) < g.feedH {
		vpLines = append(vpLines, "")
	}
	if len(vpLines) > g.feedH {
		vpLines = vpLines[:g.feedH]
	}

	allLines := append([]string{title, headers}, vpLines...)
	allLines = fitBlock(allLines, g.panelContent, g.streamW)
	return panelStyle.Width(g.streamW).Render(strings.Join(allLines, "\n"))
}

// renderStreamTitle builds the stream panel title row.
func (m Model) renderStreamTitle(g geom) string {
	if m.filter != (Filter{}) && !g.showLeft {
		return stylePanelTitle.Render("LIVE REQUEST STREAM") + "  " + styleAccent.Render("["+m.filter.String()+"]")
	}
	return stylePanelTitle.Render("LIVE REQUEST STREAM")
}

// renderStreamHeaders builds the stream table column headers.
func (m Model) renderStreamHeaders() string {
	return styleTableHead.Render(fmt.Sprintf("TIME     METHOD PATH%-15s STUB       STATUS",
		strings.Repeat(" ", max(0, m.width-60))))
}

// renderEndpointsPanel builds the full left-panel block (title + headers + rows).
func (m Model) renderEndpointsPanel(g geom) string {
	title := m.renderEndpointsTitle(g)
	header := m.renderEndpointsHeaderRow(g)

	vpLines := strings.Split(m.endpointsView.View(), "\n")
	for len(vpLines) < g.epsH {
		vpLines = append(vpLines, "")
	}
	if len(vpLines) > g.epsH {
		vpLines = vpLines[:g.epsH]
	}

	allLines := append([]string{title, header}, vpLines...)
	allLines = fitBlock(allLines, g.panelContent, g.leftContent)
	return panelStyle.Width(g.leftContent).Render(strings.Join(allLines, "\n"))
}

// renderEndpointsTitle builds the endpoints panel title line.
func (m Model) renderEndpointsTitle(g geom) string {
	base := fmt.Sprintf("REGISTERED ENDPOINTS (%d)", len(m.endpoints))
	if m.filter != (Filter{}) && g.showLeft {
		return stylePanelTitle.Render(base) + "  " + styleAccent.Render("["+m.filter.String()+"]")
	}
	return stylePanelTitle.Render(base)
}

// renderEndpointsHeaderRow builds the column-header line for the endpoints panel.
func (m Model) renderEndpointsHeaderRow(g geom) string {
	return styleTableHead.Render(fmt.Sprintf("METHOD ENDPOINT%-*s STUB      REQ   P95",
		max(0, g.epW-8), ""))
}

// endpointsRows builds the content lines for the endpoints panel viewport.
func (m Model) endpointsRows(g geom) []string {
	if len(m.endpoints) == 0 {
		return []string{styleMuted.Render("  (no endpoints)")}
	}

	rows := make([]string, len(m.endpoints))
	for i, ep := range m.endpoints {
		rows[i] = m.endpointRow(ep, g.epW)
	}
	return rows
}

// endpointRow builds one table row for a registered endpoint.
func (m Model) endpointRow(ep *mock.Endpoint, epW int) string {
	method := ep.Method
	if method == "" {
		method = "ANY"
	}
	method = padRight(truncPlain(method, 6), 6)

	epPath := padRight(truncPlain(sanitize(ep.Path), epW), epW)
	stub := padRight(truncPlain(sanitize(ep.ID), 8), 8)

	count := m.last.HitCounts[ep.ID]
	req := padLeft(compactCount(count), 4)

	var p95 string
	if lat, ok := m.last.EndpointLatency[ep.ID]; ok && lat.Count > 0 {
		p95 = compactMs(lat.P95)
	} else {
		p95 = "—"
	}
	p95 = padLeft(p95, 6)

	return " " + styleMuted.Render(method) + " " + epPath + " " + styleMuted.Render(stub) + " " + req + " " + p95
}

// renderTitleRow builds the top title row with pause button.
func (m Model) renderTitleRow(w int) string {
	left := styleHeader.Render("Simuhook Dashboard")
	right := m.renderPauseButton()
	gap := max(1, w-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat(" ", gap) + right
}

// renderServerInfo builds the SERVER INFO chip row.
func (m Model) renderServerInfo(w int) string {
	chips := []string{
		chip("Address", m.cfg.Addr),
		chip("Stubs", fmt.Sprintf("%s (%d)", m.cfg.StubsDir, len(m.endpoints))),
		chip("Uptime", m.last.Uptime.Truncate(time.Second).String()),
		chip("Req", fmt.Sprintf("%d", m.last.TotalRequests)),
	}

	// Append endpoints chip when left panel hidden.
	if !m.showEndpoints && m.wide() {
		chips = append(chips, styleMuted.Render("[ Endpoints (")+
			fmt.Sprintf("%d", len(m.endpoints))+
			styleMuted.Render(") ▸ e ]"))
	}

	return styleMuted.Render("SERVER INFO  ") + strings.Join(chips, "  ")
}

// renderInfoDetail builds the log-dir + admin URL line.
func (m Model) renderInfoDetail(w int) string {
	adminURL := "http://localhost" + m.cfg.Addr + "/_admin/endpoints"
	return styleMuted.Render(fmt.Sprintf("Log dir: %s   Admin: %s", m.cfg.LogDir, adminURL))
}

// renderMetrics builds the METRICS & LATENCY chip row.
func (m Model) renderMetrics(w int) string {
	lat := m.last.Latency
	if lat.Count == 0 {
		return styleMuted.Render("METRICS & LATENCY  [ (no requests yet) ]")
	}
	return styleMuted.Render(fmt.Sprintf("METRICS & LATENCY  [ Avg %.1fms ]  [ Min %.1fms ]  [ Max %.1fms ]  [ p95 %.1fms ]",
		lat.Avg(), lat.Min, lat.Max, lat.P95))
}

// renderPauseButton builds the pause/resume visual control.
func (m Model) renderPauseButton() string {
	if m.liveFollow {
		return stylePauseBtn.Render("[ Pause Stream ]")
	}
	return styleResumeBtn.Render("[ Resume Stream ]")
}

// chip formats a "[ Label value ]" pill: label muted, value default.
func chip(label, value string) string {
	return styleMuted.Render("[ "+label+" ") + value + styleMuted.Render(" ]")
}

// ------------------- Stream table row rendering -------------------

// renderRow dispatches to the appropriate row renderer based on layout.
func (m Model) renderRow(e mock.LogEntry, sel bool) string {
	if m.wide() {
		return m.streamRow(e, sel)
	}
	return m.summaryLine(e, sel)
}

// streamRow renders a table row for the wide stream panel.
// Columns: MARK(2) TIME(8) SP METHOD(6) SP PATH(pathW) SP STUB(10) SP STATUS(6, right-aligned).
func (m Model) streamRow(e mock.LogEntry, sel bool) string {
	g := m.geometry()
	if g.pathW < 1 {
		return m.summaryLine(e, sel)
	}

	t, err := time.Parse(time.RFC3339Nano, e.Timestamp)
	timeStr := "??:??:??"
	if err == nil {
		timeStr = t.Local().Format("15:04:05")
	}

	method := padRight(truncPlain(sanitize(e.Method), 6), 6)
	path := padRight(truncPlain(sanitize(e.Path), g.pathW), g.pathW)

	label := sanitize(e.Webhook)
	if label == "" {
		label = "unmatched"
	}
	stub := padRight(truncPlain(label, 10), 10)

	statusStr := fmt.Sprintf("%d", e.ResponseStatus)

	// Right-align status to 6 cells.
	statusPad := strings.Repeat(" ", max(0, 6-lipgloss.Width(statusStr)))

	pre := timeStr + " " + method + " " + path + " " + stub + " " + statusPad

	if !sel {
		return "  " + pre + tint(e.ResponseStatus)
	}

	// Selected: segment-wise background.
	var sb strings.Builder
	sb.WriteString(styleSelMarker.Render("▍ "))
	sb.WriteString(styleSelBG.Render(pre))
	sb.WriteString(styleSelBG.Copy().Foreground(statusColor(e.ResponseStatus)).Render(statusStr))

	visibleWidth := lipgloss.Width("▍ ") + lipgloss.Width(pre) + lipgloss.Width(statusStr)
	if pad := g.streamW - visibleWidth; pad > 0 {
		sb.WriteString(styleSelBG.Render(strings.Repeat(" ", pad)))
	}
	return sb.String()
}

// ------------------- Narrow render methods (UNTOUCHED) -------------------

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

	footer += "q quit · r refresh · enter detail · i inspect"

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
		styleMuted.Render("│  enter   expand/collapse row  i       inspect req/resp      │"),
		styleMuted.Render("│  ↑/↓     scroll feed          pgup/dn page                  │"),
		styleMuted.Render("│  g/G     jump to top / bottom of feed                       │"),
		styleMuted.Render("│  shift+↑/↓ K/J scroll endpoints esc   cancel / clear filter │"),
		styleMuted.Render("╰─────────────────────────────────────────────────────────────╯"),
	}
	return strings.Join(lines, "\n")
}

// ------------------- UXR-008 — detail + feed (UNTOUCHED) -------------------

// summaryLine renders a single feed line for the given log entry. When sel is
// true the row uses the selection highlight with segment-wise background so the
// status tint's inner reset does not break the highlight.
func (m Model) summaryLine(e mock.LogEntry, sel bool) string {
	t, err := time.Parse(time.RFC3339Nano, e.Timestamp)
	timeStr := "??:??:??"
	if err == nil {
		timeStr = t.Local().Format("15:04:05")
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
// m.view.Width and rendered in the muted style. Request headers and query
// params are each capped at maxRenderedHeaders with a "(N more ...)" marker
// (attacker-controlled maps must not inflate this block unbounded).
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

	// Headers block. Capped at maxRenderedHeaders — request headers are
	// attacker-controlled (a client can send hundreds) and must not inflate
	// this block or feedLineToEntry without bound.
	if len(e.Headers) == 0 {
		plain = append(plain, fmt.Sprintf("%-12s %s", "headers:", "(none)"))
	} else {
		plain = append(plain, "headers:")
		keys := make([]string, 0, len(e.Headers))
		for k := range e.Headers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		count := 0
		for _, k := range keys {
			if count >= maxRenderedHeaders {
				plain = append(plain, fmt.Sprintf("  (%d more headers)", len(keys)-maxRenderedHeaders))
				break
			}
			plain = append(plain, fmt.Sprintf("  %s: %s", sanitize(k), sanitize(e.Headers[k])))
			count++
		}
	}

	// Query block. Same cap/rationale as headers — query params are equally
	// attacker-controlled.
	if len(e.Query) == 0 {
		plain = append(plain, fmt.Sprintf("%-12s %s", "query:", "(none)"))
	} else {
		plain = append(plain, "query:")
		keys := make([]string, 0, len(e.Query))
		for k := range e.Query {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		count := 0
		for _, k := range keys {
			if count >= maxRenderedHeaders {
				plain = append(plain, fmt.Sprintf("  (%d more query)", len(keys)-maxRenderedHeaders))
				break
			}
			plain = append(plain, fmt.Sprintf("  %s: %s", sanitize(k), sanitize(e.Query[k])))
			count++
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

// ------------------- WS-018 / UXR-013 — request/response inspector -------------------

// formatBody renders a raw response body for the inspector. Returns nil when
// raw is empty. Pretty-prints valid JSON up to maxInspectJSONBytes; otherwise
// renders as plain text rune-safe truncated to maxInspectBodyBytes. Lines are
// capped to maxInspectBodyLines with a "… truncated (N bytes)" marker.
func formatBody(raw string) []string {
	if raw == "" {
		return nil
	}

	// JSON pretty-print path (within size cap).
	if len(raw) <= maxInspectJSONBytes {
		var buf bytes.Buffer
		if json.Indent(&buf, []byte(raw), "", "  ") == nil && buf.Len() > 0 {
			trimmed := strings.TrimRight(buf.String(), "\n")
			if trimmed == "" {
				trimmed = buf.String()
			}
			lines := strings.Split(trimmed, "\n")
			return capLines(lines, maxInspectBodyLines, maxInspectBodyBytes)
		}
	}

	// Plain text path — rune-safe truncation.
	return plainBody(raw, maxInspectBodyBytes, maxInspectBodyLines)
}

// capLines enforces a per-line byte cap and a total line cap. Lines exceeding
// maxLineBytes are truncated rune-safe with "…". If the number of lines exceeds
// maxLines, extra lines are dropped and a "… truncated (...)" marker is appended.
func capLines(lines []string, maxLines, maxLineBytes int) []string {
	var out []string
	truncated := false
	for i, line := range lines {
		if i >= maxLines {
			truncated = true
			break
		}
		// Strip trailing \r (Windows line endings).
		line = strings.TrimRight(line, "\r")
		// Rune-safe truncation to maxLineBytes.
		if len(line) > maxLineBytes {
			line = truncPlain(line, maxLineBytes)
		}
		out = append(out, sanitize(line))
	}
	if truncated {
		out = append(out, fmt.Sprintf("… truncated (%d lines)", len(lines)))
	}
	return out
}

// plainBody renders a non-JSON body as plain text, rune-safe truncated.
func plainBody(raw string, maxBytes, maxLines int) []string {
	if len(raw) > maxBytes {
		raw = raw[:maxBytes]
	}
	// Strip trailing \r\n and \r.
	raw = strings.TrimRight(raw, "\r\n")
	rawLines := strings.Split(raw, "\n")
	// Drop trailing empty lines from split.
	for len(rawLines) > 0 && rawLines[len(rawLines)-1] == "" {
		rawLines = rawLines[:len(rawLines)-1]
	}
	origLen := len(rawLines)
	if len(rawLines) > maxLines {
		rawLines = rawLines[:maxLines]
	}
	var out []string
	for _, l := range rawLines {
		l = strings.TrimRight(l, "\r")
		l = sanitize(l)
		out = append(out, l)
	}
	if origLen > maxLines {
		out = append(out, fmt.Sprintf("… truncated (%d lines)", origLen))
	}
	return out
}

// inspectLines returns the inspector-detail lines for a log entry. The gutter
// is "  │ " (gutterWidth=4). Keys are sorted for determinism. Includes both
// request and response sections. Every returned line is guaranteed to contain
// no \n characters (line-to-entry mapping invariant).
func (m Model) inspectLines(e mock.LogEntry) []string {
	const gutterWidth = 4
	gutter := styleMuted.Render("  │ ")
	contentW := m.view.Width - gutterWidth
	if contentW < 1 {
		contentW = 1
	}

	var plain []string

	// --- REQUEST section ---
	// Fixed fields.
	plain = append(plain, fmt.Sprintf("%-12s %s %s", "request:", sanitize(e.Method), sanitize(e.Path)))
	webhook := e.Webhook
	if webhook == "" {
		webhook = "unmatched"
	}
	plain = append(plain, fmt.Sprintf("%-12s %s", "endpoint:", sanitize(webhook)))
	plain = append(plain, fmt.Sprintf("%-12s %s", "request_id:", e.RequestID))
	matched := "false"
	if e.Matched {
		matched = "true"
	}
	plain = append(plain, fmt.Sprintf("%-12s %s", "matched:", matched))
	plain = append(plain, fmt.Sprintf("%-12s %s", "timestamp:", e.Timestamp))
	plain = append(plain, fmt.Sprintf("%-12s %.1fms", "latency:", e.LatencyMs))

	// Request headers block.
	if len(e.Headers) == 0 {
		plain = append(plain, fmt.Sprintf("%-12s %s", "headers:", "(none)"))
	} else {
		plain = append(plain, "headers:")
		keys := make([]string, 0, len(e.Headers))
		for k := range e.Headers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		count := 0
		for _, k := range keys {
			if count >= maxRenderedHeaders {
				plain = append(plain, styleMuted.Render(fmt.Sprintf("  (%d more headers)", len(keys)-maxRenderedHeaders)))
				break
			}
			plain = append(plain, fmt.Sprintf("  %s: %s", sanitize(k), sanitize(e.Headers[k])))
			count++
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

	// Request body.
	if e.Body == "" {
		plain = append(plain, fmt.Sprintf("%-12s %s", "body:", "(empty)"))
	} else {
		body := sanitize(e.Body)
		plain = append(plain, fmt.Sprintf("%-12s %s", "body:", body))
	}

	// --- RESPONSE section ---

	// Build RESPONSE section header with status tint (segment-wise).
	respHead := styleSection.Render(gutter+fmt.Sprintf("%-12s", "RESPONSE")) + tint(e.ResponseStatus)
	if lipgloss.Width(respHead) > m.view.Width {
		// Degraded: width overflow, render untinted.
		respHead = styleMuted.Render(gutter + fmt.Sprintf("%-12s %d", "RESPONSE", e.ResponseStatus))
	}
	// respHead is already styled, put it directly into the result.

	// For plain lines we'll build them and then apply gutter/mute at the end.
	// But respHead bypasses the normal gutter application since it has a tinted
	// segment. We handle it specially.

	// Response headers block.
	var respPlain []string
	// Variant index (backlog: response-variant-index) — only shown when the
	// endpoint used a weighted/round-robin Responses slice; nil (singular
	// Response, no variants configured) renders nothing here.
	if e.ResponseVariant != nil {
		respPlain = append(respPlain, fmt.Sprintf("%-12s %d", "variant:", *e.ResponseVariant))
	}
	if len(e.ResponseHeaders) == 0 {
		respPlain = append(respPlain, fmt.Sprintf("%-12s %s", "headers:", "(none)"))
	} else {
		respPlain = append(respPlain, "headers:")
		keys := make([]string, 0, len(e.ResponseHeaders))
		for k := range e.ResponseHeaders {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		count := 0
		for _, k := range keys {
			if count >= maxRenderedHeaders {
				respPlain = append(respPlain, styleMuted.Render(fmt.Sprintf("  (%d more headers)", len(keys)-maxRenderedHeaders)))
				break
			}
			respPlain = append(respPlain, fmt.Sprintf("  %s: %s", sanitize(k), sanitize(e.ResponseHeaders[k])))
			count++
		}
	}

	// Response body.
	var bodyLines []string
	if e.ResponseBody == "" {
		respPlain = append(respPlain, fmt.Sprintf("%-12s %s", "body:", "(empty)"))
	} else {
		bodyLines = formatBody(e.ResponseBody)
		if len(bodyLines) == 0 {
			respPlain = append(respPlain, fmt.Sprintf("%-12s %s", "body:", "(empty)"))
		} else {
			firstLine := bodyLines[0]
			respPlain = append(respPlain, fmt.Sprintf("%-12s %s", "body:", firstLine))
			for i := 1; i < len(bodyLines); i++ {
				respPlain = append(respPlain, fmt.Sprintf("  %s", bodyLines[i]))
			}
		}
	}

	// Assemble result: plain request lines (with gutter), RESPONSE header (pre-styled), response lines (with gutter).
	result := make([]string, 0, len(plain)+1+len(respPlain))

	for _, line := range plain {
		truncated := truncPlain(line, contentW)
		result = append(result, styleMuted.Render("  │ "+truncated))
	}

	// RESPONSE section header (already styled).
	result = append(result, truncPlain(respHead, m.view.Width))

	for _, line := range respPlain {
		truncated := truncPlain(line, contentW)
		result = append(result, styleMuted.Render("  │ "+truncated))
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
		sb.WriteString(m.renderRow(e, sel))
		sb.WriteByte('\n')
		lineToEntry = append(lineToEntry, idx)

		if e.RequestID != "" && e.RequestID == m.expandedID {
			expandedHit = true
			var dl []string
			if m.inspect {
				dl = m.inspectLines(e)
			} else {
				dl = m.detailLines(e)
			}
			for _, ln := range dl {
				sb.WriteString(ln)
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

// ------------------- Layout helpers -------------------

// fitBlock ensures a list of lines has exactly n rows, each one truncated to
// w cells and blank-padded. ANSI-aware.
func fitBlock(lines []string, n, w int) []string {
	result := make([]string, n)
	for i := 0; i < n; i++ {
		if i < len(lines) {
			line := lines[i]
			if lipgloss.Width(line) > w {
				// Truncate ANSI-aware: keep only what fits.
				line = truncPlain(line, w)
			}
			// Pad to exact width.
			if lw := lipgloss.Width(line); lw < w {
				line += strings.Repeat(" ", w-lw)
			}
			result[i] = line
		} else {
			result[i] = strings.Repeat(" ", w)
		}
	}
	return result
}

// padRight pads a string to the given width with trailing spaces.
func padRight(s string, w int) string {
	lw := lipgloss.Width(s)
	if lw >= w {
		return s
	}
	return s + strings.Repeat(" ", w-lw)
}

// padLeft pads a string to the given width with leading spaces.
func padLeft(s string, w int) string {
	lw := lipgloss.Width(s)
	if lw >= w {
		return s
	}
	return strings.Repeat(" ", w-lw) + s
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ------------------- Formatters -------------------

// compactCount formats a count fitting in ≤4 cells:
// 0..9999, 10k..999k, 1M..
func compactCount(n uint64) string {
	if n < 10000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%dk", n/1000)
	}
	m := float64(n) / 1000000.0
	// Truncate to ≤4 cells: "1M", "12M", "999M"
	if m >= 1000 {
		return fmt.Sprintf("%.0fM", math.Floor(m))
	}
	if m >= 100 {
		return fmt.Sprintf("%.0fM", m)
	}
	if m >= 10 {
		return fmt.Sprintf("%.0fM", m)
	}
	return fmt.Sprintf("%.0fM", m)
}

// compactMs formats milliseconds to ≤6 cells:
// <100 "%.1fms", <10000 "%.0fms", <100000 "%.1fs", else "%.0fs".
func compactMs(ms float64) string {
	if ms < 100 {
		return fmt.Sprintf("%.1fms", ms)
	}
	if ms < 10000 {
		return fmt.Sprintf("%.0fms", ms)
	}
	if ms < 100000 {
		return fmt.Sprintf("%.1fs", ms/1000.0)
	}
	return fmt.Sprintf("%.0fs", ms/1000.0)
}
