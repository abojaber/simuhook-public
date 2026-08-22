package mock

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Test 1: Accept wraps conn as net.Conn
// ---------------------------------------------------------------------------

func TestRawTeeListener_Accept_WrapsConn(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	rtl := NewRawTeeListener(ln, []ProxyRule{{Path: "/api", Destination: "https://example.com"}}, t.TempDir())
	defer rtl.Close()

	go func() {
		conn, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			return
		}
		conn.Close()
	}()

	conn, err := rtl.Accept()
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	defer conn.Close()

	// Verify it satisfies net.Conn.
	var _ net.Conn = conn
	if conn.RemoteAddr() == nil {
		t.Error("RemoteAddr() returned nil")
	}
}

// ---------------------------------------------------------------------------
// Test 2: Proxy-path request captures with >>> markers
// ---------------------------------------------------------------------------

func TestRawTeeConn_ProxyPath_CapturesRequest(t *testing.T) {
	logDir := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	rules := []ProxyRule{{Path: "/api", Destination: "https://api.example.com"}}
	rtl := NewRawTeeListener(ln, rules, logDir)
	defer rtl.Close()

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})}
	go srv.Serve(rtl)
	defer srv.Close()

	resp, err := http.Get("http://" + ln.Addr().String() + "/api/orders")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	// Give the sink time to flush.
	time.Sleep(100 * time.Millisecond)

	raw, err := os.ReadFile(filepath.Join(logDir, "_raw.log"))
	if err != nil {
		t.Fatalf("read _raw.log: %v", err)
	}
	content := string(raw)

	if !strings.Contains(content, ">>> GET /api/orders HTTP/1.1") {
		t.Errorf("_raw.log missing request line with >>> marker:\n%s", content)
	}
	if !strings.Contains(content, "<<< ") {
		t.Errorf("_raw.log missing response with <<< marker:\n%s", content)
	}
	if !strings.Contains(content, "rule /api=https://api.example.com") {
		t.Errorf("_raw.log missing connection separator with rule:\n%s", content)
	}
}

// ---------------------------------------------------------------------------
// Test 3: Non-proxy path produces no capture
// ---------------------------------------------------------------------------

func TestRawTeeConn_NonProxyPath_Passthrough(t *testing.T) {
	logDir := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	rules := []ProxyRule{{Path: "/api", Destination: "https://api.example.com"}}
	rtl := NewRawTeeListener(ln, rules, logDir)
	defer rtl.Close()

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})}
	go srv.Serve(rtl)
	defer srv.Close()

	resp, err := http.Get("http://" + ln.Addr().String() + "/health")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	time.Sleep(100 * time.Millisecond)

	rawPath := filepath.Join(logDir, "_raw.log")
	if _, err := os.Stat(rawPath); err == nil {
		raw, _ := os.ReadFile(rawPath)
		t.Errorf("_raw.log should not exist for non-proxy path, got:\n%s", string(raw))
	}
}

// ---------------------------------------------------------------------------
// Test 4: Malformed header — raw TCP with invalid header line
// ---------------------------------------------------------------------------

func TestRawTeeConn_MalformedHeader_StillCaptured(t *testing.T) {
	logDir := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	rules := []ProxyRule{{Path: "/api", Destination: "https://api.example.com"}}
	rtl := NewRawTeeListener(ln, rules, logDir)
	defer rtl.Close()

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})}
	go srv.Serve(rtl)
	defer srv.Close()

	// Send a request with a malformed header (no colon).
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	raw := "POST /api/x HTTP/1.1\r\nAccept; json\r\nHost: 127.0.0.1\r\n\r\n"
	if _, err := conn.Write([]byte(raw)); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read the response (should be 400 from Go's parser).
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}

	time.Sleep(100 * time.Millisecond)

	data, err := os.ReadFile(filepath.Join(logDir, "_raw.log"))
	if err != nil {
		t.Fatalf("read _raw.log: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, ">>> POST /api/x HTTP/1.1") {
		t.Errorf("_raw.log missing malformed request:\n%s", content)
	}
	if !strings.Contains(content, "<<< ") {
		t.Errorf("_raw.log missing 400 response:\n%s", content)
	}
}

// ---------------------------------------------------------------------------
// Test 5: Request line exceeds 8KB budget → stateFailed, no capture
// ---------------------------------------------------------------------------

func TestRawTeeConn_RequestLineExceeds8KB(t *testing.T) {
	logDir := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	rules := []ProxyRule{{Path: "/api", Destination: "https://example.com"}}
	rtl := NewRawTeeListener(ln, rules, logDir)
	defer rtl.Close()

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})}
	go srv.Serve(rtl)
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Build a 9KB request line.
	bigPath := "/api/" + strings.Repeat("a", 9*1024)
	reqLine := "GET " + bigPath + " HTTP/1.1\r\nHost: x\r\n\r\n"
	if _, err := conn.Write([]byte(reqLine)); err != nil {
		t.Fatalf("write: %v", err)
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		// Go may reject the oversized request line, which is fine.
		return
	}
	resp.Body.Close()

	time.Sleep(100 * time.Millisecond)

	rawPath := filepath.Join(logDir, "_raw.log")
	if _, err := os.Stat(rawPath); err == nil {
		raw, _ := os.ReadFile(rawPath)
		content := string(raw)
		if strings.Contains(content, ">>>") {
			t.Errorf("_raw.log should not contain request data for oversized line:\n%s", content)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 6: Cap exceeded mid-request — first 64KB captured
// ---------------------------------------------------------------------------

func TestRawTeeConn_CapExceeded_MidRequest(t *testing.T) {
	logDir := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	rules := []ProxyRule{{Path: "/api", Destination: "https://example.com"}}
	rtl := NewRawTeeListener(ln, rules, logDir)
	defer rtl.Close()

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	})}
	go srv.Serve(rtl)
	defer srv.Close()

	// Build a 70KB body.
	body := strings.Repeat("x", 70*1024)
	req, _ := http.NewRequest("POST", "http://"+ln.Addr().String()+"/api/upload", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	time.Sleep(200 * time.Millisecond)

	data, err := os.ReadFile(filepath.Join(logDir, "_raw.log"))
	if err != nil {
		t.Fatalf("read _raw.log: %v", err)
	}

	// Total captured bytes (excluding markers and separators) should be <= 64KB.
	totalCaptured := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, ">>> ") {
			totalCaptured += len(line) - 4 // subtract ">>> " prefix
		}
		if strings.HasPrefix(line, "<<< ") {
			totalCaptured += len(line) - 4
		}
	}

	if totalCaptured > captureCap+4096 {
		t.Errorf("captured %d bytes, want <= %d + headers overhead", totalCaptured, captureCap)
	}
}

// ---------------------------------------------------------------------------
// Test 7: Write() captures response with <<< markers
// ---------------------------------------------------------------------------

func TestRawTeeConn_Write_CapturesResponse(t *testing.T) {
	logDir := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	rules := []ProxyRule{{Path: "/api", Destination: "https://example.com"}}
	rtl := NewRawTeeListener(ln, rules, logDir)
	defer rtl.Close()

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "hello")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"123"}`))
	})}
	go srv.Serve(rtl)
	defer srv.Close()

	resp, err := http.Get("http://" + ln.Addr().String() + "/api/test")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	time.Sleep(100 * time.Millisecond)

	data, err := os.ReadFile(filepath.Join(logDir, "_raw.log"))
	if err != nil {
		t.Fatalf("read _raw.log: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "<<< 201") && !strings.Contains(content, "<<< HTTP/1.1 201") {
		t.Errorf("_raw.log missing response status line with <<<:\n%s", content)
	}
	if !strings.Contains(content, "<<< ") {
		t.Errorf("_raw.log missing <<< response marker:\n%s", content)
	}
}

// ---------------------------------------------------------------------------
// Test 8: Concurrent Read/Write with -race
// ---------------------------------------------------------------------------

func TestRawTeeConn_ReadWrite_Concurrent(t *testing.T) {
	logDir := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	rules := []ProxyRule{{Path: "/api", Destination: "https://example.com"}}
	rtl := NewRawTeeListener(ln, rules, logDir)
	defer rtl.Close()

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})}
	go srv.Serve(rtl)
	defer srv.Close()

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			url := fmt.Sprintf("http://%s/api/g%d", ln.Addr().String(), n)
			resp, err := http.Get(url)
			if err != nil {
				return
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}(i)
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// Test 9: Keep-alive — second request inherits decision
// ---------------------------------------------------------------------------

func TestRawTeeConn_KeepAlive_InheritsDecision(t *testing.T) {
	logDir := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	rules := []ProxyRule{{Path: "/api", Destination: "https://example.com"}}
	rtl := NewRawTeeListener(ln, rules, logDir)
	defer rtl.Close()

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})}
	go srv.Serve(rtl)
	defer srv.Close()

	// Use a persistent client (keep-alive).
	client := &http.Client{Transport: &http.Transport{
		MaxIdleConnsPerHost: 1,
	}}

	// First request — captures.
	resp1, err := client.Get("http://" + ln.Addr().String() + "/api/first")
	if err != nil {
		t.Fatalf("first GET: %v", err)
	}
	resp1.Body.Close()

	// Second request on same connection — should also be captured.
	resp2, err := client.Get("http://" + ln.Addr().String() + "/api/second")
	if err != nil {
		t.Fatalf("second GET: %v", err)
	}
	resp2.Body.Close()
	client.CloseIdleConnections()

	time.Sleep(200 * time.Millisecond)

	data, err := os.ReadFile(filepath.Join(logDir, "_raw.log"))
	if err != nil {
		t.Fatalf("read _raw.log: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "/api/first") {
		t.Errorf("_raw.log missing first request:\n%s", content)
	}
	// Second request inherits capture (D6 — decide once per connection).
	if !strings.Contains(content, "/api/second") {
		t.Errorf("_raw.log missing second request (keep-alive inheritance):\n%s", content)
	}
}

// ---------------------------------------------------------------------------
// Test 10: No proxy rules → listener is NOT wrapped (identity)
// ---------------------------------------------------------------------------

func TestRawTeeListener_NoProxy_NoWrap(t *testing.T) {
	// When len(proxyRules) == 0, main.go does NOT wrap. This test verifies
	// NewRawTeeListener still works but would produce no captures. In
	// production, the guard is in main.go.
	logDir := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	rtl := NewRawTeeListener(ln, nil, logDir)
	defer rtl.Close()

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})}
	go srv.Serve(rtl)
	defer srv.Close()

	resp, err := http.Get("http://" + ln.Addr().String() + "/anything")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	time.Sleep(100 * time.Millisecond)

	rawPath := filepath.Join(logDir, "_raw.log")
	if _, err := os.Stat(rawPath); err == nil {
		raw, _ := os.ReadFile(rawPath)
		if len(raw) > 0 {
			t.Errorf("_raw.log should be empty with no proxy rules:\n%s", string(raw))
		}
	}
}

// ---------------------------------------------------------------------------
// Test 11: Concurrent sink writes
// ---------------------------------------------------------------------------

func TestRawTeeSink_ConcurrentWrites(t *testing.T) {
	logDir := t.TempDir()
	sink := newRawTeeSink(logDir)
	defer sink.Close()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			data := []byte(fmt.Sprintf("goroutine %d\n", n))
			sink.Write(data)
		}(i)
	}
	wg.Wait()

	data, err := os.ReadFile(filepath.Join(logDir, "_raw.log"))
	if err != nil {
		t.Fatalf("read _raw.log: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 10 {
		t.Errorf("got %d lines, want 10:\n%s", len(lines), string(data))
	}
}

// ---------------------------------------------------------------------------
// Test 12: Non-printable bytes hex-escaped as \xNN
// ---------------------------------------------------------------------------

func TestRawTeeConn_NonPrintableHexEscaped(t *testing.T) {
	logDir := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	rules := []ProxyRule{{Path: "/api", Destination: "https://example.com"}}
	rtl := NewRawTeeListener(ln, rules, logDir)
	defer rtl.Close()

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Write response with non-printable bytes.
		w.Write([]byte{0x00, 0x01, 0x7F, 0xFF})
	})}
	go srv.Serve(rtl)
	defer srv.Close()

	resp, err := http.Get("http://" + ln.Addr().String() + "/api/test")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	time.Sleep(100 * time.Millisecond)

	data, err := os.ReadFile(filepath.Join(logDir, "_raw.log"))
	if err != nil {
		t.Fatalf("read _raw.log: %v", err)
	}
	content := string(data)

	// The response body should contain hex escapes for non-printable bytes.
	if !strings.Contains(content, "\\x00") {
		t.Errorf("_raw.log missing \\x00 escape:\n%s", content)
	}
	if !strings.Contains(content, "\\x7F") {
		t.Errorf("_raw.log missing \\x7F escape:\n%s", content)
	}
}

// ---------------------------------------------------------------------------
// Unit tests for helpers
// ---------------------------------------------------------------------------

func TestParseRequestLinePath(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{"standard", "GET /api/orders HTTP/1.1\r\n", "/api/orders"},
		{"post", "POST /webhook HTTP/1.1\r\n", "/webhook"},
		{"root", "GET / HTTP/1.1\r\n", "/"},
		{"with query", "GET /api?q=1 HTTP/1.1\r\n", "/api?q=1"},
		{"no version", "GET /path \r\n", "/path"},
		{"single space", "GET /only\r\n", ""},
		{"empty", "", ""},
		{"no space", "GET\r\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRequestLinePath([]byte(tt.line))
			if got != tt.want {
				t.Errorf("parseRequestLinePath(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestHexEscape(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{"printable", []byte("hello world"), "hello world"},
		{"tab", []byte("a\tb"), "a\tb"},
		{"newline", []byte("a\nb"), "a\nb"},
		{"cr", []byte("a\rb"), "a\rb"},
		{"null", []byte{0x00}, "\\x00"},
		{"del", []byte{0x7F}, "\\x7F"},
		{"high", []byte{0xFF}, "\\xFF"},
		{"mixed", []byte{0x00, 'a', 0xFF}, "\\x00a\\xFF"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hexEscape(tt.in)
			if got != tt.want {
				t.Errorf("hexEscape(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRawTeeSink_LazyOpen(t *testing.T) {
	logDir := t.TempDir()
	sink := newRawTeeSink(logDir)

	// File should not exist yet.
	if _, err := os.Stat(filepath.Join(logDir, "_raw.log")); err == nil {
		t.Fatal("_raw.log should not exist before first write")
	}

	sink.Write([]byte("first write\n"))

	data, err := os.ReadFile(filepath.Join(logDir, "_raw.log"))
	if err != nil {
		t.Fatalf("read _raw.log: %v", err)
	}
	if string(data) != "first write\n" {
		t.Errorf("got %q, want %q", string(data), "first write\n")
	}

	sink.Close()
}

func TestRawTeeSink_CloseIdempotent(t *testing.T) {
	sink := newRawTeeSink(t.TempDir())
	if err := sink.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Ring buffer tests
// ---------------------------------------------------------------------------

func TestRawLogRing_AppendAndSnapshot(t *testing.T) {
	ring := newRawLogRing(10)

	ring.append("line 1")
	ring.append("line 2")
	ring.append("line 3")

	snap := ring.snapshot()
	if len(snap) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(snap))
	}
	if snap[0] != "line 1" || snap[1] != "line 2" || snap[2] != "line 3" {
		t.Errorf("unexpected snapshot: %v", snap)
	}
}

func TestRawLogRing_MaxSize(t *testing.T) {
	ring := newRawLogRing(3)

	ring.append("a")
	ring.append("b")
	ring.append("c")
	ring.append("d") // should evict "a"

	snap := ring.snapshot()
	if len(snap) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(snap))
	}
	if snap[0] != "b" || snap[1] != "c" || snap[2] != "d" {
		t.Errorf("unexpected snapshot after eviction: %v", snap)
	}
}

func TestRawLogRing_SnapshotIsCopy(t *testing.T) {
	ring := newRawLogRing(10)
	ring.append("original")

	snap := ring.snapshot()
	snap[0] = "modified"

	snap2 := ring.snapshot()
	if snap2[0] != "original" {
		t.Error("snapshot should be a copy, not a reference")
	}
}

func TestRawLogRing_EmptySnapshot(t *testing.T) {
	ring := newRawLogRing(10)
	snap := ring.snapshot()
	if len(snap) != 0 {
		t.Errorf("expected empty snapshot, got %d lines", len(snap))
	}
}

func TestRawLogRing_ExactMaxSize(t *testing.T) {
	ring := newRawLogRing(3)

	ring.append("a")
	ring.append("b")
	ring.append("c")

	snap := ring.snapshot()
	if len(snap) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(snap))
	}
	if snap[0] != "a" || snap[1] != "b" || snap[2] != "c" {
		t.Errorf("unexpected snapshot: %v", snap)
	}
}

func TestRawTeeSink_RawLogLines(t *testing.T) {
	dir := t.TempDir()
	sink := newRawTeeSink(dir)
	defer sink.Close()

	// Write some data with newlines.
	sink.Write([]byte("line one\nline two\n"))

	lines := sink.RawLogLines()
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines in ring, got %d", len(lines))
	}
	if lines[0] != "line one" {
		t.Errorf("expected 'line one', got %q", lines[0])
	}
	if lines[1] != "line two" {
		t.Errorf("expected 'line two', got %q", lines[1])
	}
}

func TestRawTeeSink_RawLogLines_EmptyLinesFiltered(t *testing.T) {
	dir := t.TempDir()
	sink := newRawTeeSink(dir)
	defer sink.Close()

	// Write data with empty lines — should be filtered out.
	sink.Write([]byte("a\n\nb\n\nc\n"))

	lines := sink.RawLogLines()
	if len(lines) != 3 {
		t.Fatalf("expected 3 non-empty lines, got %d: %v", len(lines), lines)
	}
}

func TestRawTeeListener_RawLogLines(t *testing.T) {
	logDir := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	rules := []ProxyRule{{Path: "/api", Destination: "https://example.com"}}
	rtl := NewRawTeeListener(ln, rules, logDir)
	defer rtl.Close()

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})}
	go srv.Serve(rtl)
	defer srv.Close()

	resp, err := http.Get("http://" + ln.Addr().String() + "/api/test")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	time.Sleep(100 * time.Millisecond)

	lines := rtl.RawLogLines()
	if len(lines) == 0 {
		t.Error("expected non-empty raw log lines after proxy request")
	}

	// Verify the lines contain request/response markers.
	hasReq := false
	hasResp := false
	for _, line := range lines {
		if strings.HasPrefix(line, ">>>") {
			hasReq = true
		}
		if strings.HasPrefix(line, "<<<") {
			hasResp = true
		}
	}
	if !hasReq {
		t.Error("expected request line with >>> marker")
	}
	if !hasResp {
		t.Error("expected response line with <<< marker")
	}
}
