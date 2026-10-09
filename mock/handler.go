package mock

import (
	"fmt"
	"io"
	"net/http"
	"time"
	"unicode/utf8"
)

// Handler serves the mock surface: it matches each request against the
// registry and writes the configured response. It is the single source of
// truth for request handling — main and tests must both go through it.
type Handler struct {
	registry     *Registry
	observers    []func(LogEntry)
	now          func() time.Time
	maxBodyBytes int
	proxyRules   []ProxyRule
}

// HandlerConfig wires a Handler to its collaborators.
type HandlerConfig struct {
	Registry  *Registry        // required; NewHandler panics if nil
	Observers []func(LogEntry) // called in order, synchronously, after the response is written. Must not block or panic.
	Now       func() time.Time // optional; defaults to time.Now (test seam)

	// MaxBodyBytes caps the length of LogEntry.Body / LogEntry.ResponseBody
	// (rune-safe truncation) before observers run. 0 (default) = unlimited.
	// This is a capture-time-only cap: it never affects the bytes written
	// to the actual HTTP response, nor echo/template rendering — both use
	// the full, uncapped body.
	MaxBodyBytes int

	// ProxyRules enable transparent forwarding: when no stub matches a
	// request and a rule's Path prefix matches, the request is forwarded
	// as-is to the rule's Destination, recorded, and the response returned
	// unchanged. Empty (default) disables proxying.
	ProxyRules []ProxyRule
}

// NewHandler creates a new Handler. It panics if cfg.Registry is nil.
func NewHandler(cfg HandlerConfig) *Handler {
	if cfg.Registry == nil {
		panic("mock.NewHandler: Registry must not be nil")
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Handler{
		registry:     cfg.Registry,
		observers:    append(make([]func(LogEntry), 0, len(cfg.Observers)), cfg.Observers...),
		now:          now,
		maxBodyBytes: cfg.MaxBodyBytes,
		proxyRules:   append(make([]ProxyRule, 0, len(cfg.ProxyRules)), cfg.ProxyRules...),
	}
}

// ServeHTTP implements http.Handler. It matches the request against the
// registry, writes the configured response, and calls all registered
// observers with the resulting LogEntry.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := h.now()
	requestID := NewRequestID()

	body := make([]byte, 0)
	if r.Body != nil {
		body, _ = io.ReadAll(r.Body)
		defer r.Body.Close()
	}

	headers := make(map[string]string)
	for k := range r.Header {
		headers[k] = r.Header.Get(k)
	}
	query := make(map[string]string)
	for k := range r.URL.Query() {
		query[k] = r.URL.Query().Get(k)
	}

	ep := h.registry.Match(r.Method, r.URL.Path, headers, query, body)

	var (
		responseStatus  int
		webhookName     string
		responseBody    []byte
		responseVariant *int
	)

	if ep == nil {
		if rule := MatchProxy(r.URL.Path, h.proxyRules); rule != nil {
			// Proxy fallback: forward the request as-is to the
			// destination, record the round trip, and return the
			// upstream response unchanged.
			webhookName = "_proxy"
			responseStatus = http.StatusBadGateway
			responseBody = []byte("")
			upstreamHeaders := make(map[string]string)

			var upstreamReq *UpstreamRequestLog
			var upstreamResp *UpstreamResponseLog

			resp, out, dur, ferr := ForwardRequest(r, rule.Destination, body)
			if out != nil {
				// Capture the exact outbound request (as built, before
				// the round trip) so a debugging partner can verify
				// what was/would have been sent upstream — even when
				// the round trip fails (unreachable destination, TLS
				// error, etc.).
				upstreamReq = &UpstreamRequestLog{
					Method:  out.Method,
					URL:     out.URL.String(),
					Headers: cloneHeaderValues(out.Header),
					Body:    string(body),
				}
			}
			if ferr != nil {
				responseBody = []byte(fmt.Sprintf("proxy error: %v", ferr))
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(responseStatus)
				w.Write(responseBody)
			} else {
				// Capture upstream response headers (multi-value
				// fidelity for the log) and write them back to the
				// caller unchanged.
				upstreamHeaders = firstValues(resp.Header)
				upstreamResp = &UpstreamResponseLog{
					Status:  resp.StatusCode,
					Headers: cloneHeaderValues(resp.Header),
					Body:    "",
				}
				responseStatus = resp.StatusCode
				if resp.Body != nil {
					responseBody, _ = io.ReadAll(resp.Body)
					resp.Body.Close()
					upstreamResp.Body = string(responseBody)
				}
				for k, v := range upstreamHeaders {
					w.Header().Set(k, v)
				}
				w.WriteHeader(responseStatus)
				w.Write(responseBody)
			}

			latency := dur
			if latency == 0 {
				latency = h.now().Sub(start)
			}
			entry := LogEntry{
				Timestamp:        start.UTC().Format(time.RFC3339Nano),
				RequestID:        requestID,
				Webhook:          webhookName,
				Method:           r.Method,
				Path:             r.URL.Path,
				Headers:          headers,
				Body:             truncateRuneSafe(string(body), h.maxBodyBytes),
				Query:            query,
				ResponseStatus:   responseStatus,
				LatencyMs:        float64(latency.Microseconds()) / 1000.0,
				Matched:          true,
				ResponseHeaders:  upstreamHeaders,
				ResponseBody:     truncateRuneSafe(string(responseBody), h.maxBodyBytes),
				ProxyDestination: rule.Destination,
				RawHeaders:       cloneHeaderValues(r.Header),
				UpstreamRequest:  upstreamReq,
				UpstreamResponse: upstreamResp,
			}
			for _, o := range h.observers {
				o(entry)
			}
			return
		}
		responseStatus = http.StatusNotFound
		responseBody = []byte(`{"error":"no matching endpoint"}`)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(responseStatus)
		w.Write(responseBody)
	} else {
		webhookName = ep.ID

		// Pick response variant (singular or from responses array).
		// PickResponse returns a pointer into the ep's responses slice —
		// never mutate through it. variantIdx is -1 for the singular
		// ep.Response (no variants configured); see LogEntry.ResponseVariant.
		resp, variantIdx := PickResponse(ep)
		if variantIdx >= 0 {
			responseVariant = &variantIdx
		}

		// Build template data BEFORE the delay sleep so that
		// {{.Now}} == request-receipt time.
		var tmplData *TemplateData
		if resp.Template {
			tmplData = BuildTemplateData(requestID, r.Method, r.URL.Path, body)
			// Override Now/Timestamp with the handler's clock so tests
			// can freeze time. In production h.now() == time.Now(),
			// so the values are equivalent.
			tmplData.Now = h.now().UTC().Format(time.RFC3339)
			tmplData.Timestamp = h.now().Unix()
		}

		if resp.Delay > 0 {
			time.Sleep(resp.Delay)
		}

		// Set response headers (with optional template execution).
		for k, v := range resp.Headers {
			if resp.Template {
				v = ExecuteHeaderTemplate(v, tmplData, headers, query)
			}
			w.Header().Set(k, v)
		}

		// StatusCode of 0 defaults to 200 OK.
		responseStatus = resp.StatusCode
		if responseStatus == 0 {
			responseStatus = http.StatusOK
		}
		w.WriteHeader(responseStatus)

		// Determine response body: echo > template > configured.
		if resp.Echo {
			responseBody = body
		} else if resp.Template && len(resp.Body) > 0 {
			rendered := ExecuteBodyTemplate(string(resp.Body), tmplData, headers, query)
			responseBody = []byte(rendered)
		} else {
			responseBody = resp.Body
		}

		if len(responseBody) > 0 {
			w.Write(responseBody)
		}
	}

	latency := h.now().Sub(start)

	// Capture response headers and body for logging (WS-018 / UXR-013).
	respHeaders := make(map[string]string)
	for k := range w.Header() {
		respHeaders[k] = w.Header().Get(k)
	}

	entry := LogEntry{
		Timestamp:       start.UTC().Format(time.RFC3339Nano),
		RequestID:       requestID,
		Webhook:         webhookName,
		Method:          r.Method,
		Path:            r.URL.Path,
		Headers:         headers,
		Body:            truncateRuneSafe(string(body), h.maxBodyBytes),
		Query:           query,
		ResponseStatus:  responseStatus,
		LatencyMs:       float64(latency.Microseconds()) / 1000.0,
		Matched:         ep != nil,
		ResponseHeaders: respHeaders,
		ResponseBody:    truncateRuneSafe(string(responseBody), h.maxBodyBytes),
		ResponseVariant: responseVariant,
	}

	for _, o := range h.observers {
		o(entry)
	}
}

// FormatRequestLine renders the WS-016 one-line request summary from a
// LogEntry. Unmatched entries render "(unmatched)". The timestamp is
// parsed from the UTC Timestamp field and displayed in local time.
func FormatRequestLine(e LogEntry) string {
	name := e.Webhook
	if name == "" {
		name = "unmatched"
	}

	t, err := time.Parse(time.RFC3339Nano, e.Timestamp)
	if err != nil {
		t = time.Now()
	}
	timeStr := t.Local().Format("15:04:05")

	methodPad := fmt.Sprintf("%-5s", e.Method)
	statusLine := fmt.Sprintf("%d", e.ResponseStatus)

	return fmt.Sprintf("%s  %s %s → %s  (%s)", timeStr, methodPad, e.Path, statusLine, name)
}

// truncateRuneSafe returns s unchanged when max <= 0 (unlimited) or s is
// already within the limit. Otherwise it truncates s to at most max bytes,
// walking back to the nearest rune boundary so a multi-byte UTF-8 sequence
// straddling the cap is never split — the trailing partial rune is dropped
// entirely rather than emitting invalid bytes.
func truncateRuneSafe(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	i := max
	for i > 0 && !utf8.RuneStart(s[i]) {
		i--
	}
	return s[:i]
}
