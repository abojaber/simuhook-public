package mock

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// reqLineBudget is the maximum number of bytes accumulated from the start of a
// connection to parse the first request line. If no newline is found within
// this budget, the connection enters stateFailed (no capture).
const reqLineBudget = 8 * 1024 // 8 KB

// captureCap is the maximum number of bytes captured per connection (shared
// across request and response). Once exhausted, teeing stops for that
// connection but the actual data still flows to Go's parser unchanged.
const captureCap = 64 * 1024 // 64 KB

// ---------------------------------------------------------------------------
// rawLogRing — in-memory ring buffer for recent raw log lines
// ---------------------------------------------------------------------------

type rawLogRing struct {
	mu    sync.Mutex
	lines []string
	max   int
}

func newRawLogRing(max int) *rawLogRing {
	return &rawLogRing{
		lines: make([]string, 0, max),
		max:   max,
	}
}

func (r *rawLogRing) append(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.lines) >= r.max {
		// Remove oldest entry.
		copy(r.lines, r.lines[1:])
		r.lines = r.lines[:len(r.lines)-1]
	}
	r.lines = append(r.lines, line)
}

func (r *rawLogRing) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.lines))
	copy(out, r.lines)
	return out
}

// ---------------------------------------------------------------------------
// rawTeeSink
// ---------------------------------------------------------------------------

type rawTeeSink struct {
	mu   sync.Mutex
	f    *os.File
	dir  string
	ring *rawLogRing
}

func newRawTeeSink(dir string) *rawTeeSink {
	return &rawTeeSink{
		dir:  dir,
		ring: newRawLogRing(200),
	}
}

func (s *rawTeeSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.f == nil {
		if err := os.MkdirAll(s.dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "rawtee: mkdir %s: %v\n", s.dir, err)
			return 0, err
		}
		path := filepath.Join(s.dir, "_raw.log")
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "rawtee: open %s: %v\n", path, err)
			return 0, err
		}
		s.f = f
	}

	n, err := s.f.Write(p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rawtee: write: %v\n", err)
	}

	// Append each non-empty line to the in-memory ring buffer.
	for _, line := range strings.Split(string(p), "\n") {
		if line != "" {
			s.ring.append(line)
		}
	}

	return n, err
}

func (s *rawTeeSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f != nil {
		err := s.f.Close()
		s.f = nil
		return err
	}
	return nil
}

// RawLogLines returns a snapshot of recent raw log lines from the in-memory
// ring buffer. Used by the TUI dashboard to display proxy traffic.
func (s *rawTeeSink) RawLogLines() []string {
	return s.ring.snapshot()
}

// ---------------------------------------------------------------------------
// teeState
// ---------------------------------------------------------------------------

type teeState int

const (
	stateUndecided teeState = iota
	stateCapturing
	statePassthrough
	stateFailed
)

// ---------------------------------------------------------------------------
// RawTeeConn
// ---------------------------------------------------------------------------

// RawTeeConn wraps a net.Conn and tees raw bytes to a shared sink when the
// first request line matches a configured proxy prefix. The capture decision
// is made once per connection (D6); keep-alive reuse inherits it.
type RawTeeConn struct {
	net.Conn

	mu      sync.Mutex
	state   teeState
	rules   []ProxyRule
	sink    *rawTeeSink
	budget  int64
	hdrBuf  []byte
	decided bool
}

// Read intercepts incoming bytes. During the undecided state it accumulates
// bytes into hdrBuf looking for the first newline. Once the request line is
// parsed, the capture decision is made.
func (c *RawTeeConn) Read(b []byte) (int, error) {
	c.mu.Lock()
	state := c.state
	decided := c.decided
	c.mu.Unlock()

	if decided {
		if state == stateCapturing {
			return c.captureRead(b)
		}
		return c.Conn.Read(b)
	}

	// Undecided: accumulate bytes, look for first newline.
	n, err := c.Conn.Read(b)
	if n <= 0 {
		return n, err
	}

	c.mu.Lock()

	// Re-check after reacquiring lock.
	if c.decided {
		st := c.state
		c.mu.Unlock()
		if st == stateCapturing {
			return c.teeData(b, n, err)
		}
		return n, err
	}

	// Accumulate new bytes into hdrBuf.
	c.hdrBuf = append(c.hdrBuf, b[:n]...)
	totalLen := len(c.hdrBuf)

	// Budget check: if accumulated data exceeds the limit, fail
	// regardless of whether a newline was found.
	if totalLen > reqLineBudget {
		c.decided = true
		c.state = stateFailed
		c.hdrBuf = nil
		c.mu.Unlock()
		return n, err
	}

	// Scan the newly added portion for newline.
	newStart := totalLen - n
	nlIdx := bytes.IndexByte(c.hdrBuf[newStart:], '\n')

	if nlIdx >= 0 {
		// Found end of request line within budget.
		lineEnd := newStart + nlIdx + 1
		c.decided = true

		path := parseRequestLinePath(c.hdrBuf[:lineEnd])
		var matchedRule *ProxyRule
		if path != "" {
			matchedRule = MatchProxy(path, c.rules)
		}

		if matchedRule != nil {
			c.state = stateCapturing
			ruleCopy := *matchedRule
			// Capture any remaining bytes after the request line.
			restLen := totalLen - lineEnd
			var rest []byte
			if restLen > 0 {
				rest = make([]byte, restLen)
				copy(rest, c.hdrBuf[lineEnd:])
			}
			c.mu.Unlock()

			// Write separator + request line bytes only (up to lineEnd).
			c.writeSink(&ruleCopy, lineEnd)
			if len(rest) > 0 {
				c.captureData(rest)
			}
			return n, err
		}

		c.state = statePassthrough
		c.hdrBuf = nil
		c.mu.Unlock()
		return n, err
	}

	// No newline yet, stay undecided.
	c.mu.Unlock()
	return n, err
}

func (c *RawTeeConn) captureRead(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	if n <= 0 {
		return n, err
	}
	return c.teeData(b, n, err)
}

func (c *RawTeeConn) teeData(b []byte, n int, readErr error) (int, error) {
	c.mu.Lock()
	if c.budget <= 0 {
		c.mu.Unlock()
		return n, readErr
	}
	take := int64(n)
	if take > c.budget {
		take = c.budget
	}
	c.budget -= take
	c.mu.Unlock()

	writePrefixedLines(c.sink, ">>> ", b[:take])
	return n, readErr
}

func (c *RawTeeConn) captureData(data []byte) {
	c.mu.Lock()
	if c.budget <= 0 {
		c.mu.Unlock()
		return
	}
	take := int64(len(data))
	if take > c.budget {
		take = c.budget
	}
	c.budget -= take
	c.mu.Unlock()

	writePrefixedLines(c.sink, ">>> ", data[:take])
}

func (c *RawTeeConn) writeSink(rule *ProxyRule, reqLineEnd int) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "--- [%s] conn %s rule %s=%s ---\n",
		time.Now().UTC().Format(time.RFC3339Nano),
		c.RemoteAddr().String(),
		rule.Path,
		rule.Destination,
	)
	// Write only the request line bytes (up to reqLineEnd) with >>> prefix.
	hdrEscaped := hexEscape(c.hdrBuf[:reqLineEnd])
	for _, line := range strings.Split(hdrEscaped, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		buf.WriteString(">>> ")
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	c.sink.Write(buf.Bytes())
	c.hdrBuf = nil
}

// Write intercepts outgoing bytes and tees them to the sink with <<<
// markers when in stateCapturing.
func (c *RawTeeConn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	if n <= 0 {
		return n, err
	}

	c.mu.Lock()
	if c.state == stateCapturing && c.budget > 0 {
		take := int64(n)
		if take > c.budget {
			take = c.budget
		}
		c.budget -= take
		c.mu.Unlock()

		writePrefixedLines(c.sink, "<<< ", b[:take])
	} else {
		c.mu.Unlock()
	}
	return n, err
}

// ---------------------------------------------------------------------------
// RawTeeListener
// ---------------------------------------------------------------------------

// RawTeeListener wraps a net.Listener so every accepted connection's raw
// bytes are tee'd to a shared _raw.log file when the first request line
// matches a configured proxy prefix.
type RawTeeListener struct {
	net.Listener
	rules []ProxyRule
	sink  *rawTeeSink
}

// NewRawTeeListener creates a RawTeeListener.
func NewRawTeeListener(ln net.Listener, rules []ProxyRule, logDir string) *RawTeeListener {
	return &RawTeeListener{
		Listener: ln,
		rules:    rules,
		sink:     newRawTeeSink(logDir),
	}
}

func (l *RawTeeListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &RawTeeConn{
		Conn:   conn,
		state:  stateUndecided,
		rules:  l.rules,
		sink:   l.sink,
		budget: captureCap,
	}, nil
}

func (l *RawTeeListener) Close() error {
	firstErr := l.Listener.Close()
	if err := l.sink.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

// RawLogLines returns a snapshot of recent raw log lines from the in-memory
// ring buffer. Implements dashboard.RawLogSource so the TUI can display
// proxy traffic without reading from the file.
func (l *RawTeeListener) RawLogLines() []string {
	return l.sink.RawLogLines()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// parseRequestLinePath extracts the path from an HTTP request line:
// "METHOD SP PATH SP HTTP/x.x\r\n". Returns "" if malformed.
func parseRequestLinePath(line []byte) string {
	first := bytes.IndexByte(line, ' ')
	if first < 0 {
		return ""
	}
	second := bytes.IndexByte(line[first+1:], ' ')
	if second < 0 {
		return ""
	}
	return string(line[first+1 : first+1+second])
}

// hexEscape returns a string where non-printable bytes (outside 0x20-0x7E
// and not \t, \r, \n) are replaced with \xNN.
func hexEscape(data []byte) string {
	var buf strings.Builder
	buf.Grow(len(data))
	for _, b := range data {
		if (b >= 0x20 && b <= 0x7E) || b == '\t' || b == '\r' || b == '\n' {
			buf.WriteByte(b)
		} else {
			fmt.Fprintf(&buf, "\\x%02X", b)
		}
	}
	return buf.String()
}

// writePrefixedLines hex-escapes data, splits on newlines, and writes each
// line to the sink with the given prefix (">>> " or "<<< "). Trailing \r
// before \n is stripped for readability.
func writePrefixedLines(sink *rawTeeSink, prefix string, data []byte) {
	escaped := hexEscape(data)
	var buf bytes.Buffer
	for _, line := range strings.Split(escaped, "\n") {
		line = strings.TrimRight(line, "\r")
		buf.WriteString(prefix)
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	sink.Write(buf.Bytes())
}
