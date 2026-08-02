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
)

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

// renderHeader builds the 2-line header.
func (m Model) renderHeader() string {
	title := styleHeader.Render("Simuhook Dashboard")

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

	return lipgloss.JoinVertical(lipgloss.Left, title, stats)
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

	footer := fmt.Sprintf("showing %d of %d recent · q quit · r refresh", totalDisplayed, total)

	if m.filter != (Filter{}) {
		footer += " · filter: " + m.filter.String()
	}

	return styleMuted.Render(footer)
}

// renderPromptBar builds the prompt line shown when the user is typing a filter.
func (m Model) renderPromptBar() string {
	return styleMuted.Render("filter: " + m.prompt + "▌" + "  esc cancel · enter apply")
}

// renderHelp builds a help panel showing key bindings.
func (m Model) renderHelp() string {
	lines := []string{
		styleMuted.Render("╭── Key bindings ─────────────────────────────────────────────╮"),
		styleMuted.Render("│  / f     enter filter mode    esc     cancel / clear filter │"),
		styleMuted.Render("│  r       refresh now          ?       toggle this help      │"),
		styleMuted.Render("│  c       clear filter         q       quit                  │"),
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

		fmt.Fprintf(&sb, "%s  %-5s %s → %d  %s  (%.1fms)\n",
			timeStr,
			entry.Method,
			entry.Path,
			entry.ResponseStatus,
			label,
			entry.LatencyMs,
		)
	}

	if !anyMatched && m.filter != (Filter{}) {
		return styleMuted.Render("(no entries match filter)")
	}

	return sb.String()
}
