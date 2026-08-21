package mock

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// ProxyRule maps a request path prefix to a destination URL. When a request
// arrives whose path starts with Path, the handler forwards it (as-is) to
// Destination, records both request and response, and returns the response
// unchanged to the caller — a transparent man-in-the-middle for debugging
// integrations with external services.
type ProxyRule struct {
	Path        string // path prefix to match, e.g. "/api/stripe"
	Destination string // destination URL, e.g. "https://api.stripe.com"
}

// MatchProxy returns the proxy rule whose path prefix best matches the given
// request path, or nil if none match. When multiple rules share a prefix, the
// longest (most specific) match wins — e.g. "/api/stripe/v1" beats
// "/api/stripe". The root rule "/" matches everything and acts as a catch-all.
func MatchProxy(path string, rules []ProxyRule) *ProxyRule {
	best := -1
	bestLen := -1
	for i := range rules {
		rule := &rules[i]
		if rule.Path == "/" {
			if bestLen < 1 {
				best, bestLen = i, 1
			}
			continue
		}
		if strings.HasPrefix(path, rule.Path) {
			if len(rule.Path) > bestLen {
				best, bestLen = i, len(rule.Path)
			}
		}
	}
	if best == -1 {
		return nil
	}
	return &rules[best]
}

// hopByHopHeaders are the HTTP/1.1 hop-by-hop headers (RFC 7230 §6.1) that
// must not be forwarded when acting as a proxy — they are meaningful only for
// a single transport-level connection.
var hopByHopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Proxy-Connection",
	"TE",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

// ForwardRequest clones the incoming request, rewrites its URL to the
// destination, strips hop-by-hop headers, and executes it via
// http.DefaultTransport. It returns the upstream response, the outbound
// request as actually built (useful for logging what was sent — including
// when the round trip fails), and the round-trip duration. The caller is
// responsible for closing resp.Body.
func ForwardRequest(r *http.Request, destination string, body []byte) (*http.Response, *http.Request, time.Duration, error) {
	start := time.Now()

	out, err := buildUpstreamRequest(r, destination, body)
	if err != nil {
		return nil, nil, time.Since(start), err
	}

	resp, err := http.DefaultTransport.RoundTrip(out)
	if err != nil {
		return nil, out, time.Since(start), fmt.Errorf("proxy round trip: %w", err)
	}
	return resp, out, time.Since(start), nil
}

// buildUpstreamRequest constructs the outbound request for a proxy
// forward: the destination is treated as a base URL (scheme + host), and
// the incoming request's path and query are carried over verbatim, so the
// upstream sees the original route. Headers are copied with all values,
// stripping hop-by-hop ones (RFC 7230 §6.1), and the caller's remote
// address is appended to X-Forwarded-For. The body is replayed verbatim
// via bytes.Reader, so it can be re-read for logging after the round trip.
func buildUpstreamRequest(r *http.Request, destination string, body []byte) (*http.Request, error) {
	if !strings.Contains(destination, "://") {
		destination = "https://" + destination
	}

	out, err := http.NewRequest(r.Method, destination, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build proxy request: %w", err)
	}

	// Carry over the request path and query from the incoming request.
	// The destination URL supplies scheme + host; the path is replaced
	// with the caller's path so the upstream sees the original route.
	out.URL.Path = r.URL.Path
	out.URL.RawQuery = r.URL.RawQuery

	// Copy headers, stripping hop-by-hop ones.
	for k, values := range r.Header {
		if isHopByHop(k) {
			continue
		}
		for _, v := range values {
			out.Header.Add(k, v)
		}
	}

	// Preserve the caller's remote address for upstream debugging.
	if clientIP, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		prior := out.Header.Get("X-Forwarded-For")
		if prior != "" {
			out.Header.Set("X-Forwarded-For", prior+", "+clientIP)
		} else {
			out.Header.Set("X-Forwarded-For", clientIP)
		}
	}

	return out, nil
}

// isHopByHop reports whether the header is a hop-by-hop header that must be
// consumed by the proxy and not forwarded upstream.
func isHopByHop(name string) bool {
	for _, h := range hopByHopHeaders {
		if strings.EqualFold(name, h) {
			return true
		}
	}
	return false
}

// cloneHeaderValues deep-copies an http.Header (map[string][]string) so the
// logged entry owns its slice values and never aliases the request or
// response headers that the transport may mutate.
func cloneHeaderValues(h http.Header) map[string][]string {
	if h == nil {
		return nil
	}
	out := make(map[string][]string, len(h))
	for k, values := range h {
		out[k] = append([]string(nil), values...)
	}
	return out
}

// firstValues collapses an http.Header to its first value per name,
// matching the historical map[string]string shape of LogEntry.Headers and
// LogEntry.ResponseHeaders.
func firstValues(h http.Header) map[string]string {
	if h == nil {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, values := range h {
		if len(values) > 0 {
			out[k] = values[0]
		}
	}
	return out
}
