package mock

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// frozenClock returns a function that always returns the same time.
func frozenClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

// testHandler creates a Handler with one registered endpoint.
func testHandler(t *testing.T, ep *Endpoint, observers ...func(LogEntry)) (*Handler, *Registry) {
	t.Helper()
	r := NewRegistry()
	if err := r.Add(ep); err != nil {
		t.Fatalf("add endpoint: %v", err)
	}
	return NewHandler(HandlerConfig{
		Registry:  r,
		Observers: observers,
	}), r
}

// recordingObserver returns a func(LogEntry) that records entries and a
// channel that returns them.
func recordingObserver() (func(LogEntry), <-chan LogEntry) {
	ch := make(chan LogEntry, 100)
	return func(e LogEntry) { ch <- e }, ch
}

// ---------------------------------------------------------------------------
// TestHandler_ZeroWeightVariant_StatusRoundRobin
// ---------------------------------------------------------------------------
func TestHandler_ZeroWeightVariant_StatusRoundRobin(t *testing.T) {
	ep := &Endpoint{
		ID:   "zero-status",
		Path: "/zero",
		Responses: []Response{
			{StatusCode: 200, Body: []byte(`ok`), Weight: 0},
			{StatusCode: 500, Body: []byte(`err`), Weight: 0},
		},
	}
	h, _ := testHandler(t, ep)

	counts := map[int]int{}
	for i := 0; i < 4; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/zero", nil))
		counts[rec.Code]++
	}
	if counts[200] != 2 || counts[500] != 2 {
		t.Fatalf("expected 2x200 / 2x500, got %v", counts)
	}
}

// ---------------------------------------------------------------------------
// TestHandler_ZeroWeightVariant_BodyRoundRobin
// ---------------------------------------------------------------------------
func TestHandler_ZeroWeightVariant_BodyRoundRobin(t *testing.T) {
	ep := &Endpoint{
		ID:   "zero-bodies",
		Path: "/zb",
		Responses: []Response{
			{StatusCode: 200, Body: []byte(`a`), Weight: 0},
			{StatusCode: 200, Body: []byte(`b`), Weight: 0},
		},
	}
	h, _ := testHandler(t, ep)

	expected := []string{"a", "b", "a", "b"}
	for i, want := range expected {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/zb", nil))
		got, _ := io.ReadAll(rec.Result().Body)
		if string(got) != want {
			t.Fatalf("step %d: expected %q, got %q", i, want, string(got))
		}
	}
}

// ---------------------------------------------------------------------------
// TestHandler_NilResponseHeaders — Headers: nil => no panic.
// ---------------------------------------------------------------------------
func TestHandler_NilResponseHeaders(t *testing.T) {
	// 204 case: net/http suppresses body on 204, so assert status only.
	t.Run("204", func(t *testing.T) {
		ep := &Endpoint{
			ID:       "nil-hdrs",
			Path:     "/nil",
			Response: Response{StatusCode: 204, Headers: nil, Body: []byte(`ignored`)},
		}
		h, _ := testHandler(t, ep)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nil", nil))
		if rec.Code != 204 {
			t.Fatalf("expected 204, got %d", rec.Code)
		}
	})

	// 200 case: assert body was written.
	t.Run("200", func(t *testing.T) {
		ep := &Endpoint{
			ID:       "nil-hdrs-200",
			Path:     "/nil200",
			Response: Response{StatusCode: 200, Headers: nil, Body: []byte(`hello`)},
		}
		h, _ := testHandler(t, ep)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nil200", nil))
		if rec.Code != 200 {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if got := rec.Body.String(); got != "hello" {
			t.Fatalf("expected body 'hello', got %q", got)
		}
	})
}

// ---------------------------------------------------------------------------
// TestHandler_MatchedResponse
// ---------------------------------------------------------------------------
func TestHandler_MatchedResponse(t *testing.T) {
	ep := &Endpoint{
		ID:       "hello",
		Method:   "GET",
		Path:     "/hello",
		Response: Response{StatusCode: 201, Body: []byte(`{"msg":"hi"}`), Headers: map[string]string{"X-Custom": "yes"}},
	}
	h, _ := testHandler(t, ep)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/hello", nil))

	if rec.Code != 201 {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	if rec.Header().Get("X-Custom") != "yes" {
		t.Fatalf("expected X-Custom header")
	}
	if got := rec.Body.String(); got != `{"msg":"hi"}` {
		t.Fatalf("expected body, got %s", got)
	}
}

// ---------------------------------------------------------------------------
// TestHandler_Unmatched404
// ---------------------------------------------------------------------------
func TestHandler_Unmatched404(t *testing.T) {
	h := NewHandler(HandlerConfig{Registry: NewRegistry()})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nonexistent", nil))

	if rec.Code != 404 {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}
	if got := rec.Body.String(); got != `{"error":"no matching endpoint"}` {
		t.Fatalf("expected error body, got %s", got)
	}
}

// ---------------------------------------------------------------------------
// TestHandler_DefaultStatusCode
// ---------------------------------------------------------------------------
func TestHandler_DefaultStatusCode(t *testing.T) {
	ep := &Endpoint{
		ID:       "def",
		Path:     "/def",
		Response: Response{StatusCode: 0, Body: []byte(`ok`)},
	}
	h, _ := testHandler(t, ep)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/def", nil))

	if rec.Code != 200 {
		t.Fatalf("expected default 200, got %d", rec.Code)
	}
	if got := rec.Body.String(); got != "ok" {
		t.Fatalf("expected body 'ok', got %q", got)
	}
}

// ---------------------------------------------------------------------------
// TestHandler_EchoResponseBody
// ---------------------------------------------------------------------------
func TestHandler_EchoResponseBody(t *testing.T) {
	ep := &Endpoint{
		ID:     "echo-ep",
		Method: "POST",
		Path:   "/echo",
		Response: Response{
			StatusCode: 200,
			Body:       []byte(`should be ignored`),
			Echo:       true,
		},
	}
	h, _ := testHandler(t, ep)

	payload := `{"hello":"world"}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(payload)))

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Body.String(); got != payload {
		t.Fatalf("expected echoed body %q, got %q", payload, got)
	}
}

// ---------------------------------------------------------------------------
// TestHandler_EchoOverridesTemplate
// ---------------------------------------------------------------------------
func TestHandler_EchoOverridesTemplate(t *testing.T) {
	ep := &Endpoint{
		ID:     "echo-tmpl",
		Method: "POST",
		Path:   "/echo-tmpl",
		Response: Response{
			StatusCode: 200,
			Body:       []byte(`{{.Method}}`),
			Echo:       true,
			Template:   true,
		},
	}
	h, _ := testHandler(t, ep)

	payload := `{"sent":"back"}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/echo-tmpl", strings.NewReader(payload)))

	if got := rec.Body.String(); got != payload {
		t.Fatalf("expected echoed body %q, got %q", payload, got)
	}
}

// ---------------------------------------------------------------------------
// TestHandler_TemplateBody
// ---------------------------------------------------------------------------
func TestHandler_TemplateBody(t *testing.T) {
	ep := &Endpoint{
		ID:     "tmpl",
		Method: "POST",
		Path:   "/tmpl",
		Response: Response{
			StatusCode: 200,
			Body:       []byte(`{"id":"{{.UUID}}","method":"{{.Method}}","path":"{{.Path}}","body":"{{.Body}}"}`),
			Template:   true,
		},
	}
	h, _ := testHandler(t, ep)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/tmpl", strings.NewReader(`{"x":1}`)))

	bodyStr := rec.Body.String()
	if !strings.Contains(bodyStr, `"method":"POST"`) {
		t.Fatalf("expected method POST in body, got %s", bodyStr)
	}
	if !strings.Contains(bodyStr, `"path":"/tmpl"`) {
		t.Fatalf("expected path /tmpl in body, got %s", bodyStr)
	}
	if !strings.Contains(bodyStr, `"body":"{`) {
		t.Fatalf("expected body content in template, got %s", bodyStr)
	}
	if !strings.Contains(bodyStr, `"id":"`) {
		t.Fatalf("expected UUID in body, got %s", bodyStr)
	}
}

// ---------------------------------------------------------------------------
// TestHandler_TemplateHeader_HasHeaderAndQueryFuncs
// ---------------------------------------------------------------------------
func TestHandler_TemplateHeader_HasHeaderAndQueryFuncs(t *testing.T) {
	ep := &Endpoint{
		ID:     "hdr-tmpl",
		Method: "POST",
		Path:   "/hdr-tmpl",
		Response: Response{
			StatusCode: 200,
			Body:       []byte(`ok`),
			Template:   true,
			Headers: map[string]string{
				"X-Foo":  `{{Header "X-Foo"}}`,
				"X-From": `{{Query "k"}}`,
			},
		},
	}
	h, _ := testHandler(t, ep)

	req := httptest.NewRequest(http.MethodPost, "/hdr-tmpl?k=v42", nil)
	req.Header.Set("X-Foo", "bar")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Foo"); got != "bar" {
		t.Fatalf("expected X-Foo=bar, got %q", got)
	}
	if got := rec.Header().Get("X-From"); got != "v42" {
		t.Fatalf("expected X-From=v42, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// TestHandler_TemplateTimestampIsPreDelay
// ---------------------------------------------------------------------------
func TestHandler_TemplateTimestampIsPreDelay(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	ep := &Endpoint{
		ID:     "time-stamp",
		Method: "GET",
		Path:   "/ts",
		Response: Response{
			StatusCode: 200,
			Body:       []byte(`{{.Now}}`),
			Template:   true,
			Delay:      100 * time.Millisecond,
		},
	}
	r := NewRegistry()
	r.Add(ep)

	h := NewHandler(HandlerConfig{
		Registry: r,
		Now:      frozenClock(now),
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ts", nil))

	// {{.Now}} must equal the frozen clock, not a later time.
	expected := now.UTC().Format(time.RFC3339)
	if got := rec.Body.String(); got != expected {
		t.Fatalf("expected pre-delay time %q, got %q", expected, got)
	}
}

// ---------------------------------------------------------------------------
// TestHandler_DelayHonored
// ---------------------------------------------------------------------------
func TestHandler_DelayHonored(t *testing.T) {
	ep := &Endpoint{
		ID:       "slow",
		Method:   "GET",
		Path:     "/slow",
		Response: Response{StatusCode: 200, Body: []byte(`done`), Delay: 100 * time.Millisecond},
	}
	h, _ := testHandler(t, ep)

	start := time.Now()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/slow", nil))
	elapsed := time.Since(start)

	if elapsed < 100*time.Millisecond {
		t.Fatalf("expected at least 100ms delay, got %v", elapsed)
	}
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// TestHandler_WeightedVariants
// ---------------------------------------------------------------------------
func TestHandler_WeightedVariants(t *testing.T) {
	ep := &Endpoint{
		ID:   "weighted",
		Path: "/w",
		Responses: []Response{
			{StatusCode: 200, Body: []byte(`success`), Weight: 90},
			{StatusCode: 500, Body: []byte(`error`), Weight: 10},
		},
	}
	h, _ := testHandler(t, ep)

	counts := map[int]int{}
	for i := 0; i < 200; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/w", nil))
		counts[rec.Code]++
	}

	if counts[200] < 120 {
		t.Fatalf("expected mostly 200s, got %v", counts)
	}
	if counts[500] < 5 {
		t.Fatalf("expected some 500s, got %v", counts)
	}
}

// ---------------------------------------------------------------------------
// TestHandler_ObserverReceivesFullEntry (matched)
// ---------------------------------------------------------------------------
func TestHandler_ObserverReceivesFullEntry(t *testing.T) {
	ep := &Endpoint{
		ID:       "observe-me",
		Method:   "POST",
		Path:     "/obs",
		Response: Response{StatusCode: 201, Body: []byte(`created`), Headers: map[string]string{"X-A": "1"}},
	}
	obs, ch := recordingObserver()
	h, _ := testHandler(t, ep, obs)

	req := httptest.NewRequest(http.MethodPost, "/obs?t=1", strings.NewReader(`{"payload":true}`))
	req.Header.Set("X-Foo", "bar")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	select {
	case entry := <-ch:
		if entry.Webhook != "observe-me" {
			t.Fatalf("expected Webhook 'observe-me', got %q", entry.Webhook)
		}
		if !entry.Matched {
			t.Fatal("expected Matched=true")
		}
		if entry.ResponseStatus != 201 {
			t.Fatalf("expected ResponseStatus 201, got %d", entry.ResponseStatus)
		}
		if entry.Method != "POST" {
			t.Fatalf("expected Method POST, got %s", entry.Method)
		}
		if entry.Path != "/obs" {
			t.Fatalf("expected Path /obs, got %s", entry.Path)
		}
		if entry.Headers["X-Foo"] != "bar" {
			t.Fatalf("expected header X-Foo=bar, got %v", entry.Headers)
		}
		if entry.Body != `{"payload":true}` {
			t.Fatalf("expected body, got %q", entry.Body)
		}
		if entry.Query["t"] != "1" {
			t.Fatalf("expected query t=1, got %v", entry.Query)
		}
		if entry.RequestID == "" {
			t.Fatal("expected non-empty RequestID")
		}
		if entry.LatencyMs <= 0 {
			t.Fatal("expected LatencyMs > 0")
		}
	default:
		t.Fatal("expected an observer call")
	}

	// No second call
	select {
	case <-ch:
		t.Fatal("unexpected second observer call")
	default:
	}
}

// ---------------------------------------------------------------------------
// TestHandler_ObserverReceivesUnmatchedEntry
// ---------------------------------------------------------------------------
func TestHandler_ObserverReceivesUnmatchedEntry(t *testing.T) {
	obs, ch := recordingObserver()
	h := NewHandler(HandlerConfig{
		Registry:  NewRegistry(),
		Observers: []func(LogEntry){obs},
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))

	select {
	case entry := <-ch:
		if entry.Webhook != "" {
			t.Fatalf("expected empty Webhook for unmatched, got %q", entry.Webhook)
		}
		if entry.Matched {
			t.Fatal("expected Matched=false")
		}
		if entry.ResponseStatus != 404 {
			t.Fatalf("expected ResponseStatus 404, got %d", entry.ResponseStatus)
		}
	default:
		t.Fatal("expected an observer call")
	}
}

// ---------------------------------------------------------------------------
// TestHandler_LatencyIncludesDelay
// ---------------------------------------------------------------------------
func TestHandler_LatencyIncludesDelay(t *testing.T) {
	ep := &Endpoint{
		ID:       "lat-delay",
		Path:     "/ld",
		Response: Response{StatusCode: 200, Delay: 50 * time.Millisecond},
	}
	obs, ch := recordingObserver()
	h, _ := testHandler(t, ep, obs)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ld", nil))

	select {
	case entry := <-ch:
		if entry.LatencyMs < 50 {
			t.Fatalf("expected LatencyMs >= 50, got %f", entry.LatencyMs)
		}
	default:
		t.Fatal("expected an observer call")
	}
}

// ---------------------------------------------------------------------------
// TestHandler_Integration_LoggerAndStats
// ---------------------------------------------------------------------------
func TestHandler_Integration_LoggerAndStats(t *testing.T) {
	dir := t.TempDir()
	logger := NewLogger(dir)

	ep := &Endpoint{
		ID:       "it-ep",
		Path:     "/it",
		Response: Response{StatusCode: 200, Body: []byte(`hello`)},
	}
	r := NewRegistry()
	r.Add(ep)

	stats := NewStatsTracker(10)

	h := NewHandler(HandlerConfig{
		Registry: r,
		Observers: []func(LogEntry){
			func(e LogEntry) { _ = logger.Log(e) },
			stats.RecordRequest,
		},
	})

	// Matched
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/it", nil))
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Unmatched
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/nope", nil))
	if rec2.Code != 404 {
		t.Fatalf("expected 404, got %d", rec2.Code)
	}

	logger.Close()

	// Verify log files
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read log dir: %v", err)
	}
	found := map[string]bool{}
	for _, f := range files {
		found[f.Name()] = true
	}
	if !found["it-ep.log"] {
		t.Fatal("expected it-ep.log")
	}
	if !found["_unmatched.log"] {
		t.Fatal("expected _unmatched.log")
	}

	snap := stats.Snapshot()
	if snap.HitCounts["it-ep"] != 1 {
		t.Fatalf("expected hit count 1, got %d", snap.HitCounts["it-ep"])
	}
	if snap.TotalRequests != 2 {
		t.Fatalf("expected TotalRequests 2, got %d", snap.TotalRequests)
	}
}

// ---------------------------------------------------------------------------
// TestHandler_NilRequestBody
// ---------------------------------------------------------------------------
func TestHandler_NilRequestBody(t *testing.T) {
	ep := &Endpoint{
		ID:       "nil-body",
		Path:     "/nb",
		Response: Response{StatusCode: 200, Body: []byte(`ok`)},
	}
	h, _ := testHandler(t, ep)

	req := &http.Request{
		Method: "GET",
		URL:    &url.URL{Path: "/nb"},
		Header: make(http.Header),
	}

	rec := httptest.NewRecorder()
	// Should not panic.
	h.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// TestHandler_EmptyBody_NoWrite
// ---------------------------------------------------------------------------
func TestHandler_EmptyBody_NoWrite(t *testing.T) {
	ep := &Endpoint{
		ID:       "empty-body",
		Path:     "/eb",
		Response: Response{StatusCode: 204, Body: nil},
	}
	h, _ := testHandler(t, ep)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/eb", nil))

	if rec.Code != 204 {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	// Empty body: nothing written.
	if got := rec.Body.String(); got != "" {
		t.Fatalf("expected empty body, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// TestHandler_MultiValueQuery_UsesFirst
// ---------------------------------------------------------------------------
func TestHandler_MultiValueQuery_UsesFirst(t *testing.T) {
	ep := &Endpoint{
		ID:       "mvq",
		Path:     "/mvq",
		Response: Response{StatusCode: 200, Body: []byte(`ok`)},
	}
	obs, ch := recordingObserver()
	h, _ := testHandler(t, ep, obs)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/mvq?k=a&k=b", nil))

	select {
	case entry := <-ch:
		if entry.Query["k"] != "a" {
			t.Fatalf("expected .Get(first) semantics: 'a', got %q", entry.Query["k"])
		}
	default:
		t.Fatal("expected an observer call")
	}
}

// ---------------------------------------------------------------------------
// TestHandler_Concurrent_Race
// ---------------------------------------------------------------------------
func TestHandler_Concurrent_Race(t *testing.T) {
	ep := &Endpoint{
		ID:   "race-ep",
		Path: "/r",
		Responses: []Response{
			{StatusCode: 200, Body: []byte(`a`)},
			{StatusCode: 200, Body: []byte(`b`)},
		},
	}
	r := NewRegistry()
	r.Add(ep)
	h := NewHandler(HandlerConfig{Registry: r})

	var wg sync.WaitGroup
	var matched atomic.Int64
	var unmatched atomic.Int64

	for g := 0; g < 20; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/r", nil))
				if rec.Code == 200 {
					matched.Add(1)
				} else {
					unmatched.Add(1)
				}
			}
		}()
	}
	wg.Wait()

	total := matched.Load() + unmatched.Load()
	if total != 2000 {
		t.Fatalf("expected 2000 total, got %d", total)
	}
	if matched.Load() != 2000 {
		t.Fatalf("expected all matched to 200, got %d matched / %d unmatched", matched.Load(), unmatched.Load())
	}
}

// ---------------------------------------------------------------------------
// TestFormatRequestLine
// ---------------------------------------------------------------------------
func TestFormatRequestLine(t *testing.T) {
	t.Run("matched", func(t *testing.T) {
		e := LogEntry{
			Timestamp:      time.Date(2026, 1, 15, 14, 30, 0, 0, time.UTC).Format(time.RFC3339Nano),
			Webhook:        "orders",
			Method:         "POST",
			Path:           "/webhook",
			ResponseStatus: 201,
		}
		line := FormatRequestLine(e)
		if !strings.Contains(line, "(orders)") {
			t.Fatalf("expected (orders) in line, got %q", line)
		}
		if !strings.Contains(line, "POST") {
			t.Fatalf("expected POST in line, got %q", line)
		}
		if !strings.Contains(line, "/webhook") {
			t.Fatalf("expected /webhook in line, got %q", line)
		}
		if !strings.Contains(line, "201") {
			t.Fatalf("expected 201 in line, got %q", line)
		}
	})

	t.Run("unmatched", func(t *testing.T) {
		e := LogEntry{
			Timestamp:      time.Date(2026, 1, 15, 14, 30, 0, 0, time.UTC).Format(time.RFC3339Nano),
			Webhook:        "",
			Method:         "GET",
			Path:           "/nope",
			ResponseStatus: 404,
		}
		line := FormatRequestLine(e)
		if !strings.Contains(line, "(unmatched)") {
			t.Fatalf("expected (unmatched) in line, got %q", line)
		}
	})

	t.Run("local-time", func(t *testing.T) {
		// Timestamp in UTC. FormatRequestLine calls .Local() on the parsed time.
		e := LogEntry{
			Timestamp:      "2026-01-15T14:30:00Z",
			Webhook:        "t",
			Method:         "GET",
			Path:           "/t",
			ResponseStatus: 200,
		}
		line := FormatRequestLine(e)
		// The hour should be Local, not necessarily 14.
		if line == "" {
			t.Fatal("expected non-empty line")
		}
	})
}

// ---------------------------------------------------------------------------
// TestNewHandler_NilRegistryPanics
// ---------------------------------------------------------------------------
func TestNewHandler_NilRegistryPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil Registry")
		}
	}()
	NewHandler(HandlerConfig{})
}

// ---------------------------------------------------------------------------
// WS-018 / UXR-013 — response headers + body capture
// ---------------------------------------------------------------------------

func TestHandler_ResponseHeadersAndBody_Matched(t *testing.T) {
	ep := &Endpoint{
		ID:     "resp-cap",
		Method: "GET",
		Path:   "/resp",
		Response: Response{
			StatusCode: 201,
			Body:       []byte(`{"msg":"ok"}`),
			Headers:    map[string]string{"X-Resp": "val", "Content-Type": "application/json"},
		},
	}
	obs, ch := recordingObserver()
	_, _ = testHandler(t, ep, obs)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/resp", nil)

	// Need to go through Handler.ServeHTTP since the observer is wired there.
	// We'll use the handler from testHandler.
	// testHandler returns (Handler, Registry) — we need the handler.
	// Re-create with proper wiring.
	r := NewRegistry()
	r.Add(ep)
	h := NewHandler(HandlerConfig{
		Registry:  r,
		Observers: []func(LogEntry){obs},
	})
	h.ServeHTTP(rec, req)

	select {
	case entry := <-ch:
		if entry.ResponseStatus != 201 {
			t.Fatalf("expected ResponseStatus 201, got %d", entry.ResponseStatus)
		}
		if entry.ResponseBody != `{"msg":"ok"}` {
			t.Fatalf("expected ResponseBody '{\"msg\":\"ok\"}', got %q", entry.ResponseBody)
		}
		if entry.ResponseHeaders["X-Resp"] != "val" {
			t.Fatalf("expected ResponseHeaders[X-Resp]=val, got %q", entry.ResponseHeaders["X-Resp"])
		}
		if entry.ResponseHeaders["Content-Type"] != "application/json" {
			t.Fatalf("expected ResponseHeaders[Content-Type], got %v", entry.ResponseHeaders)
		}
	default:
		t.Fatal("expected an observer call")
	}
}

func TestHandler_ResponseHeadersAndBody_Echo(t *testing.T) {
	ep := &Endpoint{
		ID:     "echo-resp",
		Method: "POST",
		Path:   "/echo-resp",
		Response: Response{
			StatusCode: 200,
			Body:       []byte(`ignored`),
			Echo:       true,
			Headers:    map[string]string{"X-Mode": "echo"},
		},
	}
	obs, ch := recordingObserver()
	r := NewRegistry()
	r.Add(ep)
	h := NewHandler(HandlerConfig{
		Registry:  r,
		Observers: []func(LogEntry){obs},
	})

	payload := `{"hello":"world"}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/echo-resp", strings.NewReader(payload)))

	select {
	case entry := <-ch:
		if entry.ResponseBody != payload {
			t.Fatalf("echo response body must match request body: got %q, want %q", entry.ResponseBody, payload)
		}
		if entry.ResponseHeaders["X-Mode"] != "echo" {
			t.Fatalf("expected X-Mode=echo, got %q", entry.ResponseHeaders["X-Mode"])
		}
		if entry.Body != payload {
			t.Fatalf("request body should also be captured: got %q, want %q", entry.Body, payload)
		}
	default:
		t.Fatal("expected an observer call")
	}
}

func TestHandler_ResponseHeadersAndBody_Template(t *testing.T) {
	ep := &Endpoint{
		ID:     "tmpl-resp",
		Method: "POST",
		Path:   "/tmpl-resp",
		Response: Response{
			StatusCode: 200,
			Body:       []byte(`{{.Method}}`),
			Template:   true,
			Headers:    map[string]string{"X-Tmpl": "yes"},
		},
	}
	obs, ch := recordingObserver()
	r := NewRegistry()
	r.Add(ep)
	h := NewHandler(HandlerConfig{
		Registry:  r,
		Observers: []func(LogEntry){obs},
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/tmpl-resp", nil))

	select {
	case entry := <-ch:
		if entry.ResponseBody != "POST" {
			t.Fatalf("template response body must be rendered: got %q, want 'POST'", entry.ResponseBody)
		}
		if entry.ResponseHeaders["X-Tmpl"] != "yes" {
			t.Fatalf("expected X-Tmpl=yes, got %q", entry.ResponseHeaders["X-Tmpl"])
		}
	default:
		t.Fatal("expected an observer call")
	}
}

func TestHandler_ResponseHeadersAndBody_Variant(t *testing.T) {
	ep := &Endpoint{
		ID:   "var-resp",
		Path: "/var-resp",
		Responses: []Response{
			{StatusCode: 200, Body: []byte(`first`), Headers: map[string]string{"X-Var": "1"}},
			{StatusCode: 200, Body: []byte(`second`), Headers: map[string]string{"X-Var": "2"}},
		},
	}
	obs, ch := recordingObserver()
	r := NewRegistry()
	r.Add(ep)
	h := NewHandler(HandlerConfig{
		Registry:  r,
		Observers: []func(LogEntry){obs},
	})

	// Call twice to exercise both variants.
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/var-resp", nil))
	}
	// After two calls, we should have entries for both variants.
	results := make([]LogEntry, 0, 2)
	for i := 0; i < 2; i++ {
		select {
		case entry := <-ch:
			results = append(results, entry)
		default:
		}
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 observer calls, got %d", len(results))
	}
	// Each call must have a non-empty response body.
	for i, entry := range results {
		if entry.ResponseBody == "" {
			t.Errorf("variant %d: expected non-empty response body", i)
		}
		if entry.ResponseHeaders["X-Var"] == "" {
			t.Errorf("variant %d: expected X-Var header", i)
		}
	}
}

func TestHandler_ResponseHeadersAndBody_Unmatched(t *testing.T) {
	obs, ch := recordingObserver()
	h := NewHandler(HandlerConfig{
		Registry:  NewRegistry(),
		Observers: []func(LogEntry){obs},
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))

	select {
	case entry := <-ch:
		if entry.ResponseStatus != 404 {
			t.Fatalf("expected 404, got %d", entry.ResponseStatus)
		}
		if entry.ResponseBody != `{"error":"no matching endpoint"}` {
			t.Fatalf("expected error body, got %q", entry.ResponseBody)
		}
		if entry.ResponseHeaders["Content-Type"] != "application/json" {
			t.Fatalf("expected Content-Type in response headers, got %v", entry.ResponseHeaders)
		}
		if entry.Body != "" {
			t.Fatalf("request body should be empty, got %q", entry.Body)
		}
	default:
		t.Fatal("expected an observer call")
	}
}

func TestHandler_ResponseHeaders_DefaultStatusCodeHasHeaders(t *testing.T) {
	ep := &Endpoint{
		ID:       "def-hdr",
		Path:     "/def-hdr",
		Response: Response{StatusCode: 0, Body: []byte(`ok`), Headers: map[string]string{"X-Def": "1"}},
	}
	obs, ch := recordingObserver()
	r := NewRegistry()
	r.Add(ep)
	h := NewHandler(HandlerConfig{
		Registry:  r,
		Observers: []func(LogEntry){obs},
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/def-hdr", nil))

	select {
	case entry := <-ch:
		if entry.ResponseStatus != 200 {
			t.Fatalf("expected default 200, got %d", entry.ResponseStatus)
		}
		if entry.ResponseHeaders["X-Def"] != "1" {
			t.Fatalf("expected X-Def=1, got %q", entry.ResponseHeaders["X-Def"])
		}
	default:
		t.Fatal("expected an observer call")
	}
}

// ---------------------------------------------------------------------------
// --max-log-body / HandlerConfig.MaxBodyBytes (backlog: max-log-body-cap)
// ---------------------------------------------------------------------------

func TestHandler_MaxBodyBytes_TruncatesLoggedBodies(t *testing.T) {
	ep := &Endpoint{
		ID:     "cap-echo",
		Method: "POST",
		Path:   "/cap-echo",
		Response: Response{
			StatusCode: 200,
			Echo:       true,
		},
	}
	obs, ch := recordingObserver()
	r := NewRegistry()
	r.Add(ep)
	h := NewHandler(HandlerConfig{
		Registry:     r,
		Observers:    []func(LogEntry){obs},
		MaxBodyBytes: 5,
	})

	payload := "hello world, this is a long body"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/cap-echo", strings.NewReader(payload)))

	// The actual wire response (echo) must remain full-length — the cap is
	// capture-time-only and must never affect what the client receives.
	if rec.Body.String() != payload {
		t.Fatalf("wire response must be uncapped: got %q, want %q", rec.Body.String(), payload)
	}

	select {
	case entry := <-ch:
		if entry.Body != "hello" {
			t.Errorf("expected entry.Body capped to 5 bytes ('hello'), got %q", entry.Body)
		}
		if entry.ResponseBody != "hello" {
			t.Errorf("expected entry.ResponseBody capped to 5 bytes ('hello'), got %q", entry.ResponseBody)
		}
	default:
		t.Fatal("expected an observer call")
	}
}

func TestHandler_MaxBodyBytes_RuneSafeTruncation(t *testing.T) {
	// "café" is 5 bytes ("caf" + 2-byte 'é'); capping at 4 bytes would split
	// the 'é' mid-rune unless truncation walks back to the boundary.
	ep := &Endpoint{
		ID:     "cap-rune",
		Method: "POST",
		Path:   "/cap-rune",
		Response: Response{
			StatusCode: 200,
			Echo:       true,
		},
	}
	obs, ch := recordingObserver()
	r := NewRegistry()
	r.Add(ep)
	h := NewHandler(HandlerConfig{
		Registry:     r,
		Observers:    []func(LogEntry){obs},
		MaxBodyBytes: 4,
	})

	payload := "café"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/cap-rune", strings.NewReader(payload)))

	select {
	case entry := <-ch:
		if !utf8.ValidString(entry.Body) {
			t.Errorf("truncated body is not valid UTF-8: %q", entry.Body)
		}
		if len(entry.Body) > 4 {
			t.Errorf("truncated body exceeds cap: %q (%d bytes)", entry.Body, len(entry.Body))
		}
		if entry.Body != "caf" {
			t.Errorf("expected the incomplete trailing rune dropped ('caf'), got %q", entry.Body)
		}
	default:
		t.Fatal("expected an observer call")
	}
}

func TestHandler_MaxBodyBytes_ZeroIsUnlimited(t *testing.T) {
	ep := &Endpoint{
		ID:     "cap-unlimited",
		Method: "POST",
		Path:   "/cap-unlimited",
		Response: Response{
			StatusCode: 200,
			Echo:       true,
		},
	}
	obs, ch := recordingObserver()
	r := NewRegistry()
	r.Add(ep)
	h := NewHandler(HandlerConfig{
		Registry:     r,
		Observers:    []func(LogEntry){obs},
		MaxBodyBytes: 0, // explicit zero == unlimited (also the zero-value default)
	})

	payload := strings.Repeat("x", 10000)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/cap-unlimited", strings.NewReader(payload)))

	select {
	case entry := <-ch:
		if len(entry.Body) != len(payload) {
			t.Errorf("expected uncapped body length %d, got %d", len(payload), len(entry.Body))
		}
		if len(entry.ResponseBody) != len(payload) {
			t.Errorf("expected uncapped response body length %d, got %d", len(payload), len(entry.ResponseBody))
		}
	default:
		t.Fatal("expected an observer call")
	}
}

func TestTruncateRuneSafe(t *testing.T) {
	cases := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{"unlimited (max=0)", "hello", 0, "hello"},
		{"negative max treated as unlimited", "hello", -1, "hello"},
		{"already within limit", "hi", 10, "hi"},
		{"exact fit", "hello", 5, "hello"},
		{"ascii truncation", "hello world", 5, "hello"},
		{"empty string", "", 5, ""},
		{"multi-byte rune dropped at boundary", "café", 4, "caf"},
		{"multi-byte rune fits exactly", "café", 5, "café"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := truncateRuneSafe(c.s, c.max)
			if got != c.want {
				t.Errorf("truncateRuneSafe(%q, %d) = %q, want %q", c.s, c.max, got, c.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("truncateRuneSafe(%q, %d) = %q is not valid UTF-8", c.s, c.max, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// LogEntry.ResponseVariant (backlog: response-variant-index)
// ---------------------------------------------------------------------------

func TestHandler_ResponseVariant_NilForSingular(t *testing.T) {
	ep := &Endpoint{
		ID:       "singular",
		Method:   "GET",
		Path:     "/singular",
		Response: Response{StatusCode: 200, Body: []byte(`ok`)},
	}
	obs, ch := recordingObserver()
	h, _ := testHandler(t, ep, obs)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/singular", nil))

	select {
	case entry := <-ch:
		if entry.ResponseVariant != nil {
			t.Fatalf("expected ResponseVariant nil for a singular response, got %v", *entry.ResponseVariant)
		}
		data, err := json.Marshal(entry)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if strings.Contains(string(data), "response_variant") {
			t.Errorf("expected response_variant key omitted from JSON when nil, got: %s", data)
		}
	default:
		t.Fatal("expected an observer call")
	}
}

func TestHandler_ResponseVariant_RoundRobinIndices(t *testing.T) {
	ep := &Endpoint{
		ID:     "multi",
		Method: "GET",
		Path:   "/multi",
		Responses: []Response{
			{StatusCode: 200, Body: []byte(`first`)},
			{StatusCode: 201, Body: []byte(`second`)},
		},
	}
	obs, ch := recordingObserver()
	h, _ := testHandler(t, ep, obs)

	wantIdx := []int{0, 1, 0}
	for i, want := range wantIdx {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/multi", nil))

		select {
		case entry := <-ch:
			if entry.ResponseVariant == nil {
				t.Fatalf("step %d: expected non-nil ResponseVariant", i)
			}
			if *entry.ResponseVariant != want {
				t.Fatalf("step %d: expected ResponseVariant=%d, got %d", i, want, *entry.ResponseVariant)
			}
			data, err := json.Marshal(entry)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if !strings.Contains(string(data), fmt.Sprintf(`"response_variant":%d`, want)) {
				t.Errorf("step %d: expected response_variant=%d present in JSON, got: %s", i, want, data)
			}
		default:
			t.Fatalf("step %d: expected an observer call", i)
		}
	}
}

// ---------------------------------------------------------------------------
// Smarter matching through the real Handler (MAT-001/002/003)
// ---------------------------------------------------------------------------

// TestHandler_GlobAndPayloadContains_Match verifies a path_glob +
// payload_contains stub returns the configured response and logs
// Matched=true (MAT-003 handler integration).
func TestHandler_GlobAndPayloadContains_Match(t *testing.T) {
	ep := &Endpoint{
		ID:                 "filter",
		Method:             "POST",
		PathGlob:           "/orders/*",
		PayloadContains:    map[string]any{"event": "order.placed", "amount": float64(42)},
		hasPayloadContains: true,
		Response:           Response{StatusCode: 200, Body: []byte(`{"ok":true}`)},
	}
	obs, ch := recordingObserver()
	h, _ := testHandler(t, ep, obs)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/orders/12345",
		strings.NewReader(`{"event":"order.placed","amount":42,"extra":"ignored"}`)))

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	got, _ := io.ReadAll(rec.Result().Body)
	if string(got) != `{"ok":true}` {
		t.Fatalf("expected configured body, got %q", string(got))
	}
	entry := <-ch
	if !entry.Matched {
		t.Fatal("expected Matched=true in log entry")
	}
	if entry.Webhook != "filter" {
		t.Fatalf("expected webhook=filter, got %q", entry.Webhook)
	}
}

// TestHandler_GlobAndPayloadContains_NoMatch verifies a wrong body (and a
// non-JSON body) against the same stub still 404s.
func TestHandler_GlobAndPayloadContains_NoMatch(t *testing.T) {
	ep := &Endpoint{
		ID:                 "filter",
		Method:             "POST",
		PathGlob:           "/orders/*",
		PayloadContains:    map[string]any{"event": "order.placed"},
		hasPayloadContains: true,
		Response:           Response{StatusCode: 200, Body: []byte(`{"ok":true}`)},
	}
	obs, ch := recordingObserver()
	h, _ := testHandler(t, ep, obs)

	for _, body := range []string{
		`{"event":"order.cancelled"}`, // wrong required value
		`not-json`,                    // non-JSON body never matches
		``,                            // empty body never matches
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/orders/12345", strings.NewReader(body)))
		if rec.Code != 404 {
			t.Errorf("body %q: expected 404, got %d", body, rec.Code)
		}
	}
	entry := <-ch
	if entry.Matched {
		t.Error("expected Matched=false in log entry for the last unmatched request")
	}
}

// TestHandler_RegexPathMatch verifies a path_regex stub matches through the
// real Handler (MAT-002).
func TestHandler_RegexPathMatch(t *testing.T) {
	ep := &Endpoint{
		ID:        "orders-by-id",
		Method:    "GET",
		PathRegex: `^/orders/[0-9]{5}$`,
		Response:  Response{StatusCode: 200, Body: []byte(`order`)},
	}
	h, _ := testHandler(t, ep)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/orders/12345", nil))
	if rec.Code != 200 {
		t.Fatalf("expected 200 for matching regex path, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/orders/123", nil))
	if rec.Code != 404 {
		t.Fatalf("expected 404 for non-matching regex path, got %d", rec.Code)
	}
}
