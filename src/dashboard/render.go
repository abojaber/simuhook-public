package dashboard

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Package-level lipgloss styles.
var (
	styleHeader = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	styleMuted  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleAccent = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	styleError  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

// ------------------- UXR-001 helpers -------------------

// tint returns the status code string wrapped in a lipgloss colour by HTTP
// class: 2xx → green, 3xx → cyan, 4xx → yellow, 5xx → red, other → muted.
func tint(status int) string {
	s := fmt.Sprintf("%d", status)
	switch {
	case status >= 200 && status < 300:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("76")).Render(s)
	case status >= 300 && status < 400:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render(s)
	case status >= 400 && status < 500:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render(s)
	case status >= 500 && status < 600:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(s)
	default:
		return styleMuted.Render(s)
	}
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
	feed := m.view.View()

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

	footer += "q quit · r refresh"

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
		styleMuted.Render("│  ↑/↓     scroll feed          pgup/dn page                  │"),
		styleMuted.Render("╰─────────────────────────────────────────────────────────────╯"),
	}
	return strings.Join(lines, "\n")
}

// buildFeed formats recent log entries into a string for the viewport.
func (m Model) buildFeed() string {
	if len(m.last.RecentLogs) == 0 {
		return styleMuted.Render("(no requests yet)")
	}

	var sb strings.Builder
	anyMatched := false
	for _, entry := range m.last.RecentLogs {
		if m.filter != (Filter{}) && !m.filter.Matches(entry) {
			continue
		}
		anyMatched = true

		// Parse timestamp to extract the time portion
		t, err := time.Parse(time.RFC3339Nano, entry.Timestamp)
		timeStr := "??:??:??"
		if err == nil {
			timeStr = t.Format("15:04:05")
		}

		label := entry.Webhook
		if label == "" {
			label = "unmatched"
		}

		if m.width < 60 {
			// UXR-005 — narrow terminals: drop latency segment.
			fmt.Fprintf(&sb, "%s  %-5s %s → %s  %s\n",
				timeStr,
				entry.Method,
				entry.Path,
				tint(entry.ResponseStatus),
				label,
			)
		} else {
			fmt.Fprintf(&sb, "%s  %-5s %s → %s  %s  (%.1fms)\n",
				timeStr,
				entry.Method,
				entry.Path,
				tint(entry.ResponseStatus),
				label,
				entry.LatencyMs,
			)
		}
	}

	if !anyMatched && m.filter != (Filter{}) {
		return styleMuted.Render("(no entries match filter)")
	}

	return sb.String()
}
