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
	footer := m.renderFooter()
	return lipgloss.JoinVertical(lipgloss.Top, header, feed, footer)
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
	totalLogs := m.last.TotalLogs
	footer := fmt.Sprintf("showing %d of %d recent · q quit · r refresh", total, totalLogs)
	return styleMuted.Render(footer)
}

// buildFeed formats recent log entries into a string for the viewport.
func (m Model) buildFeed() string {
	if len(m.last.RecentLogs) == 0 {
		return styleMuted.Render("(no requests yet)")
	}

	var sb strings.Builder
	for _, entry := range m.last.RecentLogs {
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
	return sb.String()
}