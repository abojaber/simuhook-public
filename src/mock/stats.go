package mock

import (
	"sync"
	"time"
)

// StatsTracker tracks request statistics and keeps a ring buffer of recent logs.
type StatsTracker struct {
	mu         sync.RWMutex
	startTime  time.Time
	hitCounts  map[string]uint64
	recentLogs []LogEntry
	maxLogs    int
	nextIndex  int
	full       bool
}

// NewStatsTracker creates a StatsTracker with the given ring buffer size.
func NewStatsTracker(maxLogs int) *StatsTracker {
	return &StatsTracker{
		startTime:  time.Now(),
		hitCounts:  make(map[string]uint64),
		recentLogs: make([]LogEntry, maxLogs),
		maxLogs:    maxLogs,
	}
}

// RecordRequest records a log entry and updates hit counts.
func (s *StatsTracker) RecordRequest(entry LogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry.Webhook != "" {
		s.hitCounts[entry.Webhook]++
	}

	s.recentLogs[s.nextIndex] = entry
	s.nextIndex++
	if s.nextIndex >= s.maxLogs {
		s.nextIndex = 0
		s.full = true
	}
}

// StatsSnapshot is a point-in-time copy of the tracker state.
type StatsSnapshot struct {
	Uptime     time.Duration
	HitCounts  map[string]uint64
	RecentLogs []LogEntry
	TotalLogs  uint64
}

// Snapshot returns a thread-safe copy of the current stats.
func (s *StatsTracker) Snapshot() StatsSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	counts := make(map[string]uint64, len(s.hitCounts))
	for k, v := range s.hitCounts {
		counts[k] = v
	}

	var logs []LogEntry
	if s.full {
		logs = make([]LogEntry, s.maxLogs)
		for i := 0; i < s.maxLogs; i++ {
			logs[i] = s.recentLogs[(s.nextIndex+i)%s.maxLogs]
		}
	} else {
		logs = make([]LogEntry, s.nextIndex)
		copy(logs, s.recentLogs[:s.nextIndex])
	}

	total := uint64(len(s.recentLogs))
	if !s.full {
		total = uint64(s.nextIndex)
	}

	return StatsSnapshot{
		Uptime:     time.Since(s.startTime),
		HitCounts:  counts,
		RecentLogs: logs,
		TotalLogs:  total,
	}
}

// Uptime returns the time since the tracker was created.
func (s *StatsTracker) Uptime() time.Duration {
	return time.Since(s.startTime)
}
