package mock

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// MatchProxy
// ---------------------------------------------------------------------------

func TestMatchProxy_ExactMatch(t *testing.T) {
	rules := []ProxyRule{
		{Path: "/api/stripe", Destination: "https://api.stripe.com"},
		{Path: "/api/github", Destination: "https://api.github.com"},
	}
	rule := MatchProxy("/api/stripe/v1/charges", rules)
	if rule == nil {
		t.Fatal("expected match for /api/stripe/v1/charges")
	}
	if rule.Destination != "https://api.stripe.com" {
		t.Fatalf("got destination %q, want https://api.stripe.com", rule.Destination)
	}
}

func TestMatchProxy_NoMatch(t *testing.T) {
	rules := []ProxyRule{
		{Path: "/api/stripe", Destination: "https://api.stripe.com"},
	}
	if rule := MatchProxy("/other/path", rules); rule != nil {
		t.Fatalf("expected no match, got %q", rule.Destination)
	}
}

func TestMatchProxy_LongestPrefixWins(t *testing.T) {
	rules := []ProxyRule{
		{Path: "/api", Destination: "https://api.example.com"},
		{Path: "/api/stripe", Destination: "https://api.stripe.com"},
		{Path: "/api/stripe/v1", Destination: "https://api.stripe.com/v1"},
	}
	rule := MatchProxy("/api/stripe/v1/charges", rules)
	if rule == nil || rule.Destination != "https://api.stripe.com/v1" {
		t.Fatalf("expected longest prefix match, got %+v", rule)
	}
}

func TestMatchProxy_RootCatchAll(t *testing.T) {
	rules := []ProxyRule{
		{Path: "/api", Destination: "https://api.example.com"},
		{Path: "/", Destination: "https://catchall.example.com"},
	}
	rule := MatchProxy("/anything/else", rules)
	if rule == nil || rule.Destination != "https://catchall.example.com" {
		t.Fatalf("expected root catch-all, got %+v", rule)
	}
}

func TestMatchProxy_EmptyRules(t *testing.T) {
	if rule := MatchProxy("/anything", nil); rule != nil {
		t.Fatalf("expected nil with empty rules, got %+v", rule)
	}
}

// ---------------------------------------------------------------------------
// ForwardRequest
// ---------------------------------------------------------------------------

// TestForwardRequest_Passthrough verifies the upstream receives the same
// method, path, query, headers, and body as the incoming request.
func TestForwardRequest_Passthrough(t *testing.T) {
	var gotMethod, gotPath, gotQuery, gotHeader, gotBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotHeader = r.Header.Get("X-Custom")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/test?id=1", strings.NewReader(`{"a":1}`))
	req.Header.Set("X-Custom", "hello")

	resp, _, dur, err := ForwardRequest(req, upstream.URL, []byte(`{"a":1}`))
	if err != nil {
		t.Fatalf("ForwardRequest: %v", err)
	}
	defer resp.Body.Close()

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/test" {
		t.Errorf("path = %q, want /api/test", gotPath)
	}
	if gotQuery != "id=1" {
		t.Errorf("query = %q, want id=1", gotQuery)
	}
	if gotHeader != "hello" {
		t.Errorf("header = %q, want hello", gotHeader)
	}
	if gotBody != `{"a":1}` {
		t.Errorf("body = %q, want {\"a\":1}", gotBody)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if dur <= 0 {
		t.Errorf("duration = %v, want > 0", dur)
	}
}

// TestForwardRequest_HopByHopStripped verifies connection-scoped headers are
// not forwarded upstream.
func TestForwardRequest_HopByHopStripped(t *testing.T) {
	var gotConn, gotTransfer string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotConn = r.Header.Get("Connection")
		gotTransfer = r.Header.Get("Transfer-Encoding")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Transfer-Encoding", "chunked")

	_, _, _, err := ForwardRequest(req, upstream.URL, nil)
	if err != nil {
		t.Fatalf("ForwardRequest: %v", err)
	}

	if gotConn != "" {
		t.Errorf("Connection header forwarded: %q, want empty", gotConn)
	}
	if gotTransfer != "" {
		t.Errorf("Transfer-Encoding forwarded: %q, want empty", gotTransfer)
	}
}

// TestForwardRequest_DestinationUnreachable verifies a non-empty error is
// returned when the destination cannot be reached.
func TestForwardRequest_DestinationUnreachable(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, _, _, err := ForwardRequest(req, "http://127.0.0.1:1", nil)
	if err == nil {
		t.Fatal("expected error for unreachable destination")
	}
}

// ---------------------------------------------------------------------------
// Handler integration: proxy fallback
// ---------------------------------------------------------------------------

// TestHandler_ProxyFallback verifies a request that matches no stub but
// matches a proxy rule is forwarded, logged with proxy fields, and the
// upstream response is returned unchanged.
func TestHandler_ProxyFallback(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/echo" {
			t.Errorf("upstream path = %q, want /api/echo", r.URL.Path)
		}
		w.Header().Set("X-Upstream", "yes")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"proxied":true}`))
	}))
	defer upstream.Close()

	obs, ch := recordingObserver()
	h := NewHandler(HandlerConfig{
		Registry:  NewRegistry(),
		Observers: []func(LogEntry){obs},
		ProxyRules: []ProxyRule{
			{Path: "/api", Destination: upstream.URL},
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/echo?x=1", strings.NewReader(`{"ping":true}`))
	req.Header.Set("X-Test", "abc")

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if rec.Header().Get("X-Upstream") != "yes" {
		t.Errorf("X-Upstream header missing, got %q", rec.Header().Get("X-Upstream"))
	}
	if rec.Body.String() != `{"proxied":true}` {
		t.Errorf("body = %q, want {\"proxied\":true}", rec.Body.String())
	}

	select {
	case entry := <-ch:
		if entry.ProxyDestination != upstream.URL {
			t.Errorf("ProxyDestination = %q, want %q", entry.ProxyDestination, upstream.URL)
		}
		if !entry.Matched {
			t.Error("Matched = false, want true (proxy handled the request)")
		}
		if entry.Webhook != "_proxy" {
			t.Errorf("Webhook = %q, want _proxy", entry.Webhook)
		}
		if entry.ResponseStatus != http.StatusCreated {
			t.Errorf("ResponseStatus = %d, want 201", entry.ResponseStatus)
		}
		if entry.ResponseBody != `{"proxied":true}` {
			t.Errorf("ResponseBody = %q, want {\"proxied\":true}", entry.ResponseBody)
		}
		if entry.Path != "/api/echo" {
			t.Errorf("Path = %q, want /api/echo", entry.Path)
		}
		if entry.Query["x"] != "1" {
			t.Errorf("Query = %v, want x=1", entry.Query)
		}
		if entry.Headers["X-Test"] != "abc" {
			t.Errorf("Headers[X-Test] = %q, want abc", entry.Headers["X-Test"])
		}
		if entry.Body != `{"ping":true}` {
			t.Errorf("Body = %q, want {\"ping\":true}", entry.Body)
		}
		if entry.LatencyMs <= 0 {
			t.Errorf("LatencyMs = %v, want > 0", entry.LatencyMs)
		}
	default:
		t.Fatal("observer was not called")
	}
}

// TestHandler_ProxyError502 verifies an unreachable destination produces a
// 502 with a logged error body and ProxyDestination still set.
func TestHandler_ProxyError502(t *testing.T) {
	obs, ch := recordingObserver()
	h := NewHandler(HandlerConfig{
		Registry:  NewRegistry(),
		Observers: []func(LogEntry){obs},
		ProxyRules: []ProxyRule{
			{Path: "/dead", Destination: "http://127.0.0.1:1"},
		},
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/dead", nil))

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "proxy error") {
		t.Errorf("body = %q, want contains 'proxy error'", rec.Body.String())
	}

	select {
	case entry := <-ch:
		if entry.ResponseStatus != http.StatusBadGateway {
			t.Errorf("ResponseStatus = %d, want 502", entry.ResponseStatus)
		}
		if entry.ProxyDestination != "http://127.0.0.1:1" {
			t.Errorf("ProxyDestination = %q, want http://127.0.0.1:1", entry.ProxyDestination)
		}
		if !entry.Matched {
			t.Error("Matched = false, want true")
		}
	default:
		t.Fatal("observer was not called")
	}
}

// TestHandler_ProxyNoMatchStill404 verifies requests that match no stub and
// no proxy rule still return the standard 404.
func TestHandler_ProxyNoMatchStill404(t *testing.T) {
	h := NewHandler(HandlerConfig{
		Registry: NewRegistry(),
		ProxyRules: []ProxyRule{
			{Path: "/api", Destination: "http://127.0.0.1:1"},
		},
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/other", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// TestHandler_StubTakesPriorityOverProxy verifies a matching stub is served
// even when a proxy rule also matches the path.
func TestHandler_StubTakesPriorityOverProxy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream should not be called; stub must win")
	}))
	defer upstream.Close()

	ep := &Endpoint{
		ID:     "local",
		Method: http.MethodGet,
		Path:   "/api/local",
		Response: Response{
			StatusCode: http.StatusOK,
			Body:       []byte(`{"from":"stub"}`),
		},
	}
	h, _ := testHandler(t, ep)
	h.proxyRules = []ProxyRule{{Path: "/api", Destination: upstream.URL}}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/local", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != `{"from":"stub"}` {
		t.Errorf("body = %q, want {\"from\":\"stub\"}", rec.Body.String())
	}
}

// TestMatchProxy_TimeBoundary is a compile-time guard that the package's
// time import is used (latency is reported from ForwardRequest).
func TestMatchProxy_TimeBoundary(t *testing.T) {
	if time.Second <= 0 {
		t.Fatal("time broken")
	}
}

// ---------------------------------------------------------------------------
// Proxy log fidelity: RawHeaders, UpstreamRequest, UpstreamResponse
// ---------------------------------------------------------------------------

// TestHandler_ProxyLogFidelity_MultiValueHeaders verifies the proxy log
// preserves every header value the client sent (RawHeaders is
// map[string][]string), while the legacy Headers field keeps its
// first-value shape for backward compatibility.
func TestHandler_ProxyLogFidelity_MultiValueHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Upstream-Multi", "first")
		w.Header().Add("X-Upstream-Multi", "second")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"proxied":true}`))
	}))
	defer upstream.Close()

	obs, ch := recordingObserver()
	h := NewHandler(HandlerConfig{
		Registry:  NewRegistry(),
		Observers: []func(LogEntry){obs},
		ProxyRules: []ProxyRule{
			{Path: "/api", Destination: upstream.URL},
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/test?x=1", strings.NewReader(`{"ping":true}`))
	req.Header.Add("X-Multi", "one")
	req.Header.Add("X-Multi", "two")

	h.ServeHTTP(rec, req)

	// Sanity: response written back unchanged.
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	select {
	case entry := <-ch:
		// Legacy first-value shape stays intact.
		if entry.Headers["X-Multi"] != "one" {
			t.Errorf("Headers[X-Multi] = %q, want one", entry.Headers["X-Multi"])
		}
		// New fidelity field keeps ALL values.
		got := entry.RawHeaders["X-Multi"]
		if len(got) != 2 || got[0] != "one" || got[1] != "two" {
			t.Errorf("RawHeaders[X-Multi] = %v, want [one two]", got)
		}
		// Upstream response multi-value headers preserved too.
		upHeaders := entry.UpstreamResponse.Headers["X-Upstream-Multi"]
		if len(upHeaders) != 2 || upHeaders[0] != "first" || upHeaders[1] != "second" {
			t.Errorf("UpstreamResponse.Headers[X-Upstream-Multi] = %v, want [first second]", upHeaders)
		}
	default:
		t.Fatal("observer was not called")
	}
}

// TestHandler_ProxyLogFidelity_InvalidJSONBody verifies an invalid JSON
// payload (as a real client might send while debugging) is logged verbatim
// in Body, UpstreamRequest.Body, and UpstreamResponse.Body.
func TestHandler_ProxyLogFidelity_InvalidJSONBody(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"echoed":` + string(b) + `}`))
	}))
	defer upstream.Close()

	obs, ch := recordingObserver()
	h := NewHandler(HandlerConfig{
		Registry:  NewRegistry(),
		Observers: []func(LogEntry){obs},
		ProxyRules: []ProxyRule{
			{Path: "/api", Destination: upstream.URL},
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/broken", strings.NewReader(`{"broken": json`))
	h.ServeHTTP(rec, req)

	select {
	case entry := <-ch:
		if entry.Body != `{"broken": json` {
			t.Errorf("Body = %q, want invalid JSON verbatim", entry.Body)
		}
		if entry.UpstreamRequest == nil {
			t.Fatal("UpstreamRequest = nil, want non-nil")
		}
		if entry.UpstreamRequest.Body != `{"broken": json` {
			t.Errorf("UpstreamRequest.Body = %q, want verbatim", entry.UpstreamRequest.Body)
		}
		if entry.UpstreamResponse == nil {
			t.Fatal("UpstreamResponse = nil, want non-nil")
		}
		if entry.UpstreamResponse.Body != `{"echoed":{"broken": json}` {
			t.Errorf("UpstreamResponse.Body = %q, want echoed invalid JSON", entry.UpstreamResponse.Body)
		}
	default:
		t.Fatal("observer was not called")
	}
}

// TestHandler_ProxyLogFidelity_UpstreamRequestCapture verifies the exact
// outbound request is logged: full URL (destination + original path +
// query), method, hop-by-hop headers stripped, body verbatim, and
// X-Forwarded-For records the client address.
func TestHandler_ProxyLogFidelity_UpstreamRequestCapture(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	obs, ch := recordingObserver()
	h := NewHandler(HandlerConfig{
		Registry:  NewRegistry(),
		Observers: []func(LogEntry){obs},
		ProxyRules: []ProxyRule{
			{Path: "/api", Destination: upstream.URL},
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/items?a=1&b=2", strings.NewReader(`{"n":1}`))
	req.Header.Set("X-Keep", "yes")
	req.Header.Set("Connection", "close") // hop-by-hop, must NOT be forwarded
	req.RemoteAddr = "203.0.113.7:12345"
	h.ServeHTTP(rec, req)

	select {
	case entry := <-ch:
		ur := entry.UpstreamRequest
		if ur == nil {
			t.Fatal("UpstreamRequest = nil, want non-nil")
		}
		if ur.Method != http.MethodPut {
			t.Errorf("UpstreamRequest.Method = %q, want PUT", ur.Method)
		}
		wantURL := upstream.URL + "/api/items?a=1&b=2"
		if ur.URL != wantURL {
			t.Errorf("UpstreamRequest.URL = %q, want %q", ur.URL, wantURL)
		}
		if ur.Headers["X-Keep"] == nil || len(ur.Headers["X-Keep"]) != 1 || ur.Headers["X-Keep"][0] != "yes" {
			t.Errorf("UpstreamRequest.Headers[X-Keep] = %v, want [yes]", ur.Headers["X-Keep"])
		}
		if _, ok := ur.Headers["Connection"]; ok {
			t.Errorf("hop-by-hop Connection header forwarded: %v", ur.Headers["Connection"])
		}
		if ur.Body != `{"n":1}` {
			t.Errorf("UpstreamRequest.Body = %q, want {\"n\":1}", ur.Body)
		}
		xff := ur.Headers["X-Forwarded-For"]
		if len(xff) != 1 || xff[0] != "203.0.113.7" {
			t.Errorf("X-Forwarded-For = %v, want [203.0.113.7]", xff)
		}
	default:
		t.Fatal("observer was not called")
	}
}

// TestHandler_ProxyLogFidelity_ErrorStillCapturesUpstreamRequest verifies
// that when the round trip fails (unreachable destination), the log still
// records what was attempted in UpstreamRequest but leaves UpstreamResponse
// nil.
func TestHandler_ProxyLogFidelity_ErrorStillCapturesUpstreamRequest(t *testing.T) {
	obs, ch := recordingObserver()
	h := NewHandler(HandlerConfig{
		Registry:  NewRegistry(),
		Observers: []func(LogEntry){obs},
		ProxyRules: []ProxyRule{
			{Path: "/dead", Destination: "http://127.0.0.1:1"},
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/dead/endpoint?q=1", strings.NewReader(`{"x":1}`))
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}

	select {
	case entry := <-ch:
		ur := entry.UpstreamRequest
		if ur == nil {
			t.Fatal("UpstreamRequest = nil, want the attempted request")
		}
		if ur.Method != http.MethodPost {
			t.Errorf("UpstreamRequest.Method = %q, want POST", ur.Method)
		}
		if !strings.HasPrefix(ur.URL, "http://127.0.0.1:1/dead/endpoint") {
			t.Errorf("UpstreamRequest.URL = %q, want prefix http://127.0.0.1:1/dead/endpoint", ur.URL)
		}
		if ur.Body != `{"x":1}` {
			t.Errorf("UpstreamRequest.Body = %q, want {\"x\":1}", ur.Body)
		}
		if entry.UpstreamResponse != nil {
			t.Errorf("UpstreamResponse = %+v, want nil on round-trip failure", entry.UpstreamResponse)
		}
	default:
		t.Fatal("observer was not called")
	}
}

// TestHandler_ProxyLogFidelity_RawHeadersAbsentForStub verifies the new
// fidelity fields are NOT populated for normal (non-proxied) mock
// responses — they are proxy-scoped logging only.
func TestHandler_ProxyLogFidelity_RawHeadersAbsentForStub(t *testing.T) {
	ep := &Endpoint{
		ID:     "local",
		Method: http.MethodGet,
		Path:   "/hello",
		Response: Response{
			StatusCode: http.StatusOK,
			Body:       []byte("hi"),
		},
	}
	obs, ch := recordingObserver()
	h, _ := testHandler(t, ep)
	h.observers = []func(LogEntry){obs}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/hello", nil))

	select {
	case entry := <-ch:
		if entry.RawHeaders != nil {
			t.Errorf("RawHeaders populated for stub: %v", entry.RawHeaders)
		}
		if entry.UpstreamRequest != nil {
			t.Errorf("UpstreamRequest populated for stub: %+v", entry.UpstreamRequest)
		}
		if entry.UpstreamResponse != nil {
			t.Errorf("UpstreamResponse populated for stub: %+v", entry.UpstreamResponse)
		}
	default:
		t.Fatal("observer was not called")
	}
}
