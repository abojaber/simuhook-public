package dashboard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/abojaber/simuhook-public/mock"
)

// RawLogSource provides recent raw log lines for the proxy view.
type RawLogSource interface {
	RawLogLines() []string
}

// Raw log rendering styles.
var (
	styleRawSep  = lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // gray
	styleRawReq  = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))  // cyan
	styleRawResp = lipgloss.NewStyle().Foreground(lipgloss.Color("76"))  // green
	styleRawInfo = lipgloss.NewStyle().Bold(true)                        // connection info
)

// renderRawLine applies colors to a single raw log line based on its prefix.
func renderRawLine(line string) string {
	switch {
	case strings.HasPrefix(line, "---"):
		return styleRawSep.Render(line)
	case strings.HasPrefix(line, ">>>"):
		return styleRawReq.Render(line)
	case strings.HasPrefix(line, "<<<"):
		return styleRawResp.Render(line)
	default:
		return line
	}
}

// proxyEntries filters m.last.RecentLogs to entries where Webhook == "_proxy".
func (m Model) proxyEntries() []mock.LogEntry {
	var entries []mock.LogEntry
	for _, e := range m.last.RecentLogs {
		if e.Webhook == "_proxy" {
			entries = append(entries, e)
		}
	}
	return entries
}

// buildProxyContent builds the full proxy view content: structured proxy log
// entries followed by a raw bytes section. Called from the tick handler to
// update the rawLogView viewport.
func (m *Model) buildProxyContent() {
	var sb strings.Builder

	// Structured proxy log entries.
	entries := m.proxyEntries()
	if len(entries) == 0 {
		sb.WriteString(styleMuted.Render("  (no proxy requests yet)"))
	} else {
		for _, e := range entries {
			sb.WriteString(m.summaryLine(e, false))
			sb.WriteByte('\n')
		}
	}

	// Raw bytes section.
	sb.WriteByte('\n')
	sb.WriteString(styleRawSep.Render("  ─── Raw Bytes ───"))
	sb.WriteByte('\n')
	if m.rawSrc != nil {
		lines := m.rawSrc.RawLogLines()
		for _, line := range lines {
			sb.WriteString("  ")
			sb.WriteString(renderRawLine(line))
			sb.WriteByte('\n')
		}
	}

	m.rawLogView.SetContent(sb.String())
	if m.liveFollowRaw {
		m.rawLogView.GotoBottom()
	}
}

// renderRawLogView renders the full-screen proxy view. Content is set by
// buildProxyContent during the tick handler; this method assembles header,
// viewport, and footer.
func (m Model) renderRawLogView() string {
	if m.rawSrc == nil && len(m.last.RecentLogs) == 0 {
		return styleMuted.Render("No proxy rules configured. Start with --proxy to see raw traffic.")
	}

	// Header
	header := stylePanelTitle.Render("  PROXY REQUESTS  ") + styleMuted.Render("  [1] Main  [2] Proxy  [L] Follow  [?] Help")

	// Footer
	proxyCount := len(m.proxyEntries())
	rawCount := 0
	if m.rawSrc != nil {
		rawCount = len(m.rawSrc.RawLogLines())
	}
	footer := styleMuted.Render(fmt.Sprintf("  %d requests, %d raw lines  ", proxyCount, rawCount))

	return lipgloss.JoinVertical(lipgloss.Top, header, m.rawLogView.View(), footer)
}
