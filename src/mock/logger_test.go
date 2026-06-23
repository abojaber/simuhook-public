package mock

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogger_CreatesDirAndFile(t *testing.T) {
	dir := t.TempDir()
	logger := NewLogger(dir)

	entry := LogEntry{
		Timestamp:      "2025-06-23T12:00:00Z",
		RequestID:      NewRequestID(),
		Webhook:        "test-ep",
		Method:         "POST",
		Path:           "/hook",
		ResponseStatus: 201,
		LatencyMs:      12.5,
		Matched:        true,
	}

	if err := logger.Log(entry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logger.Close()

	data, err := os.ReadFile(filepath.Join(dir, "test-ep.log"))
	if err != nil {
		t.Fatalf("expected log file: %v", err)
	}

	var decoded LogEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}

	if decoded.Webhook != "test-ep" {
		t.Fatalf("expected webhook test-ep, got %s", decoded.Webhook)
	}
	if decoded.ResponseStatus != 201 {
		t.Fatalf("expected status 201, got %d", decoded.ResponseStatus)
	}
	if decoded.Method != "POST" {
		t.Fatalf("expected method POST, got %s", decoded.Method)
	}
	if decoded.LatencyMs != 12.5 {
		t.Fatalf("expected latency 12.5, got %f", decoded.LatencyMs)
	}
	if !decoded.Matched {
		t.Fatal("expected matched=true")
	}
	if decoded.RequestID == "" {
		t.Fatal("expected non-empty request_id")
	}
	if decoded.Timestamp == "" {
		t.Fatal("expected non-empty timestamp")
	}
}

func TestLogger_AppendsEntries(t *testing.T) {
	dir := t.TempDir()
	logger := NewLogger(dir)

	logger.Log(LogEntry{RequestID: "1", Webhook: "ep", Method: "GET", ResponseStatus: 200})
	logger.Log(LogEntry{RequestID: "2", Webhook: "ep", Method: "POST", ResponseStatus: 201})
	logger.Close()

	data, _ := os.ReadFile(filepath.Join(dir, "ep.log"))
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 log lines, got %d", len(lines))
	}
}

func TestLogger_UnmatchedGoesToSeparateFile(t *testing.T) {
	dir := t.TempDir()
	logger := NewLogger(dir)

	logger.Log(LogEntry{RequestID: NewRequestID(), Webhook: "", Method: "GET", Path: "/unknown", ResponseStatus: 404, Matched: false})
	logger.Close()

	_, err := os.Stat(filepath.Join(dir, "_unmatched.log"))
	if err != nil {
		t.Fatalf("expected _unmatched.log to exist: %v", err)
	}
}

func TestLogger_SeparateFilesPerWebhook(t *testing.T) {
	dir := t.TempDir()
	logger := NewLogger(dir)

	logger.Log(LogEntry{RequestID: "1", Webhook: "alpha", Method: "GET", ResponseStatus: 200})
	logger.Log(LogEntry{RequestID: "2", Webhook: "beta", Method: "POST", ResponseStatus: 201})
	logger.Close()

	alphaData, _ := os.ReadFile(filepath.Join(dir, "alpha.log"))
	betaData, _ := os.ReadFile(filepath.Join(dir, "beta.log"))

	if len(alphaData) == 0 || len(betaData) == 0 {
		t.Fatal("both log files should have content")
	}
	if string(alphaData) == string(betaData) {
		t.Fatal("log files should differ")
	}
}

func TestLogger_AllRequiredFields(t *testing.T) {
	dir := t.TempDir()
	logger := NewLogger(dir)

	entry := LogEntry{
		Timestamp:      "2025-01-15T10:30:00.123Z",
		RequestID:      "abc-123",
		Webhook:        "orders",
		Method:         "POST",
		Path:           "/webhooks/orders",
		Headers:        map[string]string{"Content-Type": "application/json"},
		Body:           `{"order_id":42}`,
		Query:          map[string]string{"token": "abc"},
		ResponseStatus: 200,
		LatencyMs:      45.2,
		Matched:        true,
	}

	logger.Log(entry)
	logger.Close()

	data, _ := os.ReadFile(filepath.Join(dir, "orders.log"))
	var decoded LogEntry
	json.Unmarshal(data, &decoded)

	if decoded.Timestamp != "2025-01-15T10:30:00.123Z" {
		t.Fatalf("timestamp mismatch")
	}
	if decoded.RequestID != "abc-123" {
		t.Fatalf("request_id mismatch")
	}
	if decoded.Webhook != "orders" {
		t.Fatalf("webhook mismatch")
	}
	if decoded.Method != "POST" {
		t.Fatalf("method mismatch")
	}
	if decoded.Path != "/webhooks/orders" {
		t.Fatalf("path mismatch")
	}
	if decoded.Headers["Content-Type"] != "application/json" {
		t.Fatalf("headers mismatch")
	}
	if decoded.Body != `{"order_id":42}` {
		t.Fatalf("body mismatch")
	}
	if decoded.Query["token"] != "abc" {
		t.Fatalf("query mismatch")
	}
	if decoded.ResponseStatus != 200 {
		t.Fatalf("response_status mismatch")
	}
	if decoded.LatencyMs != 45.2 {
		t.Fatalf("latency_ms mismatch")
	}
	if !decoded.Matched {
		t.Fatalf("matched mismatch")
	}
}
