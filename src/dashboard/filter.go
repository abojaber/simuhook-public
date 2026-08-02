package dashboard

import (
	"fmt"
	"strconv"
	"strings"

	"simuhook/mock"
)

// Filter selects which recent log entries to display. The zero value matches
// everything (no filtering). All fields are independent AND conditions.
type Filter struct {
	Endpoint string // substring match against LogEntry.Webhook
	Method   string // exact match (case-insensitive) against LogEntry.Method
	Status   int    // 0 = any; otherwise exact match on ResponseStatus
	Text     string // case-insensitive substring against Path (and Webhook)
}

// String renders the filter for display in the prompt/footer. Empty filter returns "".
func (f Filter) String() string {
	var parts []string
	if f.Endpoint != "" {
		parts = append(parts, "ep:"+f.Endpoint)
	}
	if f.Method != "" {
		parts = append(parts, "method:"+f.Method)
	}
	if f.Status != 0 {
		parts = append(parts, fmt.Sprintf("status:%d", f.Status))
	}
	if f.Text != "" {
		parts = append(parts, f.Text)
	}
	return strings.Join(parts, " ")
}

// ParseFilter parses a filter expression. Supported syntax:
//
//	ep:<substring>  → Endpoint
//	method:<m>      → Method (case-insensitive, exact)
//	status:<n>      → Status (integer; invalid → ignored with ok=false)
//	bare token      → Text
//
// Tokens are whitespace-separated. Unknown prefixes like foo: are treated as Text.
func ParseFilter(expr string) (Filter, bool) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return Filter{}, true
	}

	var f Filter
	tokens := strings.Fields(expr)
	for _, tok := range tokens {
		switch {
		case strings.HasPrefix(tok, "ep:"):
			f.Endpoint = tok[3:]
		case strings.HasPrefix(tok, "method:"):
			f.Method = tok[7:]
		case strings.HasPrefix(tok, "status:"):
			n, err := strconv.Atoi(tok[7:])
			if err != nil {
				return Filter{}, false
			}
			f.Status = n
		default:
			f.Text = tok
		}
	}
	return f, true
}

// BadStatusToken returns the first status:token whose value is not a valid
// integer. Returns "" if all status tokens are valid or none exist.
func BadStatusToken(expr string) string {
	tokens := strings.Fields(expr)
	for _, tok := range tokens {
		if strings.HasPrefix(tok, "status:") {
			if _, err := strconv.Atoi(tok[7:]); err != nil {
				return tok
			}
		}
	}
	return ""
}

// Matches reports whether the entry passes the filter.
func (f Filter) Matches(e mock.LogEntry) bool {
	if f.Method != "" && !strings.EqualFold(f.Method, e.Method) {
		return false
	}
	if f.Status != 0 && f.Status != e.ResponseStatus {
		return false
	}
	if f.Endpoint != "" && !strings.Contains(strings.ToLower(e.Webhook), strings.ToLower(f.Endpoint)) {
		return false
	}
	if f.Text != "" {
		textLower := strings.ToLower(f.Text)
		if !strings.Contains(strings.ToLower(e.Path), textLower) &&
			!strings.Contains(strings.ToLower(e.Webhook), textLower) {
			return false
		}
	}
	return true
}
