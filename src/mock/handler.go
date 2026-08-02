package mock

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// Handler serves the mock surface: it matches each request against the
// registry and writes the configured response. It is the single source of
// truth for request handling — main and tests must both go through it.
type Handler struct {
	registry  *Registry
	observers []func(LogEntry)
	now       func() time.Time
}

// HandlerConfig wires a Handler to its collaborators.
type HandlerConfig struct {
	Registry  *Registry        // required; NewHandler panics if nil
	Observers []func(LogEntry) // called in order, synchronously, after the response is written. Must not block or panic.
	Now       func() time.Time // optional; defaults to time.Now (test seam)
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
		registry:  cfg.Registry,
		observers: append(make([]func(LogEntry), 0, len(cfg.Observers)), cfg.Observers...),
		now:       now,
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
		responseStatus int
		webhookName    string
		responseBody   []byte
	)

	if ep == nil {
		responseStatus = http.StatusNotFound
		responseBody = []byte(`{"error":"no matching endpoint"}`)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(responseStatus)
		w.Write(responseBody)
	} else {
		webhookName = ep.ID

		// Pick response variant (singular or from responses array).
		// PickResponse returns a pointer into the ep's responses slice —
		// never mutate through it.
		resp := PickResponse(ep)

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

	entry := LogEntry{
		Timestamp:      start.UTC().Format(time.RFC3339Nano),
		RequestID:      requestID,
		Webhook:        webhookName,
		Method:         r.Method,
		Path:           r.URL.Path,
		Headers:        headers,
		Body:           string(body),
		Query:          query,
		ResponseStatus: responseStatus,
		LatencyMs:      float64(latency.Microseconds()) / 1000.0,
		Matched:        ep != nil,
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