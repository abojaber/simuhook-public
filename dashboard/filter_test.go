package dashboard

import (
	"testing"

	"github.com/abojaber/simuhook-public/mock"
)

func TestFilter_EmptyMatchesAll(t *testing.T) {
	f := Filter{}
	entry := mock.LogEntry{Method: "POST", Path: "/test", ResponseStatus: 200, Webhook: "test"}
	if !f.Matches(entry) {
		t.Error("empty filter should match anything")
	}
}

func TestFilter_Parse(t *testing.T) {
	tests := []struct {
		expr   string
		expect Filter
		ok     bool
	}{
		{"", Filter{}, true},
		{"ep:order", Filter{Endpoint: "order"}, true},
		{"method:POST", Filter{Method: "POST"}, true},
		{"status:200", Filter{Status: 200}, true},
		{"hello", Filter{Text: "hello"}, true},
		{"ep:order method:POST status:200 text", Filter{Endpoint: "order", Method: "POST", Status: 200, Text: "text"}, true},
		{"status:abc", Filter{}, false},
		{"ep:order   method:GET", Filter{Endpoint: "order", Method: "GET"}, true},
		{"  status:404  ", Filter{Status: 404}, true},
		{"STATUS:200", Filter{Text: "STATUS:200"}, true},       // case-sensitive prefix
		{"unknown:thing", Filter{Text: "unknown:thing"}, true}, // unknown prefix → Text
	}
	for _, tt := range tests {
		got, ok := ParseFilter(tt.expr)
		if ok != tt.ok {
			t.Errorf("ParseFilter(%q) ok=%v, want %v", tt.expr, ok, tt.ok)
			continue
		}
		if !tt.ok {
			continue
		}
		if got.Endpoint != tt.expect.Endpoint || got.Method != tt.expect.Method ||
			got.Status != tt.expect.Status || got.Text != tt.expect.Text {
			t.Errorf("ParseFilter(%q) = %+v, want %+v", tt.expr, got, tt.expect)
		}
	}
}

func TestFilter_Matches(t *testing.T) {
	// Endpoint substring match (case-insensitive)
	f := Filter{Endpoint: "order"}
	if !f.Matches(mock.LogEntry{Webhook: "order-placed"}) {
		t.Error("endpoint substring should match")
	}
	if !f.Matches(mock.LogEntry{Webhook: "ORDER-CREATED"}) {
		t.Error("endpoint substring should be case-insensitive")
	}
	if f.Matches(mock.LogEntry{Webhook: "user-created"}) {
		t.Error("endpoint substring should not match unrelated")
	}

	// Method match (case-insensitive exact)
	f = Filter{Method: "post"}
	if !f.Matches(mock.LogEntry{Method: "POST"}) {
		t.Error("method should be case-insensitive")
	}
	if f.Matches(mock.LogEntry{Method: "GET"}) {
		t.Error("method should not match wrong method")
	}

	// Status match
	f = Filter{Status: 200}
	if !f.Matches(mock.LogEntry{ResponseStatus: 200}) {
		t.Error("status should match exact")
	}
	if f.Matches(mock.LogEntry{ResponseStatus: 404}) {
		t.Error("status should not match wrong status")
	}
	// Zero status matches any
	f = Filter{}
	if !f.Matches(mock.LogEntry{ResponseStatus: 500}) {
		t.Error("zero status should match any")
	}

	// Text match against Webhook
	f = Filter{Text: "webhook"}
	if !f.Matches(mock.LogEntry{Webhook: "my-webhook", Path: "/foo"}) {
		t.Error("text should match webhook field")
	}
	// Text match against Path
	if !f.Matches(mock.LogEntry{Webhook: "x", Path: "/webhook/test"}) {
		t.Error("text should match path field")
	}
	// Text case-insensitive
	if !f.Matches(mock.LogEntry{Webhook: "", Path: "/WEBHOOK"}) {
		t.Error("text should be case-insensitive")
	}
	// Text matches neither
	if f.Matches(mock.LogEntry{Webhook: "foo", Path: "/bar"}) {
		t.Error("text should not match when absent from both fields")
	}

	// AND combination: all conditions must be met
	f = Filter{Method: "POST", Status: 200, Endpoint: "order"}
	if !f.Matches(mock.LogEntry{Method: "POST", ResponseStatus: 200, Webhook: "order-placed"}) {
		t.Error("all conditions met should match")
	}
	if f.Matches(mock.LogEntry{Method: "POST", ResponseStatus: 201, Webhook: "order-placed"}) {
		t.Error("one condition failing should not match")
	}
	if f.Matches(mock.LogEntry{Method: "GET", ResponseStatus: 200, Webhook: "order-placed"}) {
		t.Error("wrong method should not match")
	}
	if f.Matches(mock.LogEntry{Method: "POST", ResponseStatus: 200, Webhook: "user-created"}) {
		t.Error("wrong endpoint should not match")
	}
}

func TestFilter_String(t *testing.T) {
	// Round-trip
	f := Filter{Endpoint: "order", Method: "POST", Status: 200, Text: "hello"}
	s := f.String()
	if s == "" {
		t.Error("non-empty filter should produce non-empty string")
	}
	f2, ok := ParseFilter(s)
	if !ok {
		t.Fatal("parsed filter string should be valid")
	}
	if f2.Endpoint != f.Endpoint || f2.Method != f.Method ||
		f2.Status != f.Status || f2.Text != f.Text {
		t.Errorf("String round-trip: got %+v, want %+v", f2, f)
	}

	// Empty filter
	if zero := (Filter{}).String(); zero != "" {
		t.Errorf("empty filter should produce empty string, got %q", zero)
	}

	// Partial filter
	f3 := Filter{Method: "GET"}
	s3 := f3.String()
	if s3 != "method:GET" {
		t.Errorf("partial filter string: got %q, want %q", s3, "method:GET")
	}
}
