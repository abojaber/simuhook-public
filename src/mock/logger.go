package mock

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// LogEntry represents a single logged request, written as JSON Lines.
type LogEntry struct {
	Timestamp       string            `json:"timestamp"`
	RequestID       string            `json:"request_id"`
	Webhook         string            `json:"webhook"`
	Method          string            `json:"method"`
	Path            string            `json:"path"`
	Headers         map[string]string `json:"headers"`
	Body            string            `json:"body"`
	Query           map[string]string `json:"query"`
	ResponseStatus  int               `json:"response_status"`
	LatencyMs       float64           `json:"latency_ms"`
	Matched         bool              `json:"matched"`
	ResponseHeaders map[string]string `json:"response_headers"`
	ResponseBody    string            `json:"response_body"`
}

// Logger writes JSON log entries to per-webhook files under a log directory.
type Logger struct {
	dir   string
	mu    sync.Mutex
	files map[string]*os.File
}

// NewLogger creates a Logger that writes to the given directory.
// The directory is created on first write if it does not exist.
func NewLogger(dir string) *Logger {
	return &Logger{
		dir:   dir,
		files: make(map[string]*os.File),
	}
}

// Log writes a single log entry to <dir>/<webhook>.log.
// For unmatched requests (webhook == ""), the file is named _unmatched.log.
func (l *Logger) Log(entry LogEntry) error {
	if entry.ResponseHeaders == nil {
		entry.ResponseHeaders = map[string]string{}
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal log entry: %w", err)
	}
	data = append(data, '\n')

	name := entry.Webhook
	if name == "" {
		name = "_unmatched"
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := os.MkdirAll(l.dir, 0755); err != nil {
		return fmt.Errorf("create log dir %s: %w", l.dir, err)
	}

	f, ok := l.files[name]
	if !ok {
		path := filepath.Join(l.dir, name+".log")
		f, err = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("open log file %s: %w", path, err)
		}
		l.files[name] = f
	}

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write log entry: %w", err)
	}
	return nil
}

// Close flushes and closes all open log files.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	var firstErr error
	for name, f := range l.files {
		if err := f.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close log %s: %w", name, err)
		}
		delete(l.files, name)
	}
	return firstErr
}

// NewRequestID generates a UUID v4 string for correlating log entries.
func NewRequestID() string {
	return newUUID()
}
