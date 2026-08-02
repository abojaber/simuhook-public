package mock

import (
	"sync"
	"testing"
	"time"
)

func testLogEntry(webhook, path string) LogEntry {
	return LogEntry{
		Timestamp:      time.Now().UTC().Format(time.RFC3339Nano),
		RequestID:      NewRequestID(),
		Webhook:        webhook,
		Method:         "GET",
		Path:           path,
		Headers:        map[string]string{},
		Query:          map[string]string{},
		ResponseStatus: 200,
		LatencyMs:      1.0,
		Matched:        true,
	}
}

func TestNewStatsTracker(t *testing.T) {
	s := NewStatsTracker(50)
	if s == nil {
		t.Fatal("expected non-nil tracker")
	}
	if s.maxLogs != 50 {
		t.Errorf("maxLogs = %d, want 50", s.maxLogs)
	}
	if len(s.recentLogs) != 50 {
		t.Errorf("recentLogs len = %d, want 50", len(s.recentLogs))
	}
	if s.nextIndex != 0 {
		t.Errorf("nextIndex = %d, want 0", s.nextIndex)
	}
	if s.full {
		t.Error("full = true, want false")
	}
	if s.startTime.IsZero() {
		t.Error("startTime is zero, want set")
	}
	if len(s.hitCounts) != 0 {
		t.Errorf("hitCounts len = %d, want 0", len(s.hitCounts))
	}
}

func TestRecordRequest_IncrementsHitCount(t *testing.T) {
	s := NewStatsTracker(10)
	s.RecordRequest(testLogEntry("health", "/health"))
	s.RecordRequest(testLogEntry("health", "/health"))
	s.RecordRequest(testLogEntry("orders", "/webhooks/orders"))

	if s.hitCounts["health"] != 2 {
		t.Errorf("hitCounts[health] = %d, want 2", s.hitCounts["health"])
	}
	if s.hitCounts["orders"] != 1 {
		t.Errorf("hitCounts[orders] = %d, want 1", s.hitCounts["orders"])
	}
}

func TestRecordRequest_EmptyWebhookDoesNotIncrement(t *testing.T) {
	s := NewStatsTracker(10)
	s.RecordRequest(testLogEntry("", "/unknown"))

	if len(s.hitCounts) != 0 {
		t.Errorf("hitCounts len = %d, want 0 (empty webhook must not increment)", len(s.hitCounts))
	}
	if s.nextIndex != 1 {
		t.Errorf("nextIndex = %d, want 1 (entry still recorded in ring)", s.nextIndex)
	}
}

func TestRecordRequest_RingBufferWrap(t *testing.T) {
	const max = 3
	s := NewStatsTracker(max)

	// Record one more than capacity so the oldest entry is overwritten.
	for i := 0; i < max+1; i++ {
		s.RecordRequest(testLogEntry("ep", "/ep"))
	}

	if !s.full {
		t.Error("full = false, want true after wrap")
	}
	if s.nextIndex != 1 {
		t.Errorf("nextIndex = %d, want 1 (position after wrap)", s.nextIndex)
	}

	snap := s.Snapshot()
	if len(snap.RecentLogs) != max {
		t.Errorf("recent logs len = %d, want %d", len(snap.RecentLogs), max)
	}
	if snap.TotalLogs != uint64(max) {
		t.Errorf("TotalLogs = %d, want %d", snap.TotalLogs, max)
	}
	// The oldest (first-recorded) entry must have been overwritten.
	if snap.RecentLogs[0].RequestID == s.recentLogs[0].RequestID {
		t.Error("oldest entry not overwritten; expected wrap to drop entry 0")
	}
}

func TestSnapshot_BeforeWrap(t *testing.T) {
	s := NewStatsTracker(5)
	e1 := testLogEntry("a", "/a")
	e2 := testLogEntry("b", "/b")
	s.RecordRequest(e1)
	s.RecordRequest(e2)

	snap := s.Snapshot()

	if len(snap.RecentLogs) != 2 {
		t.Fatalf("recent logs len = %d, want 2", len(snap.RecentLogs))
	}
	// Insertion order preserved.
	if snap.RecentLogs[0].RequestID != e1.RequestID {
		t.Error("RecentLogs[0] is not the first recorded entry")
	}
	if snap.RecentLogs[1].RequestID != e2.RequestID {
		t.Error("RecentLogs[1] is not the second recorded entry")
	}
	if snap.TotalLogs != 2 {
		t.Errorf("TotalLogs = %d, want 2", snap.TotalLogs)
	}
	if snap.HitCounts["a"] != 1 || snap.HitCounts["b"] != 1 {
		t.Errorf("hit counts wrong: %v", snap.HitCounts)
	}
}

func TestSnapshot_AfterWrap(t *testing.T) {
	const max = 3
	s := NewStatsTracker(max)
	// 5 entries with max 3 → entries 2,3,4 survive, oldest first.
	for i := 0; i < 5; i++ {
		s.RecordRequest(testLogEntry("ep", "/ep"))
	}

	snap := s.Snapshot()

	if len(snap.RecentLogs) != max {
		t.Fatalf("recent logs len = %d, want %d", len(snap.RecentLogs), max)
	}
	// Chronological order: last three recorded, oldest first.
	for i := 0; i < max; i++ {
		want := s.recentLogs[(s.nextIndex+i)%max].RequestID
		if snap.RecentLogs[i].RequestID != want {
			t.Errorf("RecentLogs[%d] mismatch: got %s, want %s", i, snap.RecentLogs[i].RequestID, want)
		}
	}
	if snap.TotalLogs != uint64(max) {
		t.Errorf("TotalLogs = %d, want %d", snap.TotalLogs, max)
	}
}

func TestSnapshot_EmptyBuffer(t *testing.T) {
	s := NewStatsTracker(10)
	snap := s.Snapshot()

	if len(snap.RecentLogs) != 0 {
		t.Errorf("recent logs len = %d, want 0", len(snap.RecentLogs))
	}
	if snap.TotalLogs != 0 {
		t.Errorf("TotalLogs = %d, want 0", snap.TotalLogs)
	}
	if len(snap.HitCounts) != 0 {
		t.Errorf("hitCounts len = %d, want 0", len(snap.HitCounts))
	}
}

func TestSnapshot_IsCopyNotAlias(t *testing.T) {
	s := NewStatsTracker(10)
	s.RecordRequest(testLogEntry("a", "/a"))

	snap := s.Snapshot()
	// Mutating the snapshot must not affect the tracker.
	snap.RecentLogs[0].Path = "mutated"
	snap.HitCounts["a"] = 999

	after := s.Snapshot()
	if after.RecentLogs[0].Path != "/a" {
		t.Errorf("tracker log mutated via snapshot: path = %s", after.RecentLogs[0].Path)
	}
	if after.HitCounts["a"] != 1 {
		t.Errorf("tracker hit count mutated via snapshot: = %d, want 1", after.HitCounts["a"])
	}
}

func TestUptime(t *testing.T) {
	s := NewStatsTracker(10)
	// Give the clock a moment to advance.
	time.Sleep(2 * time.Millisecond)
	uptime := s.Uptime()
	if uptime <= 0 {
		t.Errorf("Uptime() = %v, want > 0", uptime)
	}
}

func TestConcurrentRecordAndSnapshot(t *testing.T) {
	s := NewStatsTracker(50)

	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				s.RecordRequest(testLogEntry("ep", "/ep"))
				if i%50 == 0 {
					_ = s.Snapshot()
				}
			}
		}(g)
	}
	wg.Wait()

	snap := s.Snapshot()
	if snap.HitCounts["ep"] != 2000 {
		t.Errorf("HitCounts[ep] = %d, want 2000", snap.HitCounts["ep"])
	}
	if snap.TotalLogs != 50 {
		t.Errorf("TotalLogs = %d, want 50 (capped at ring size)", snap.TotalLogs)
	}
	if len(snap.RecentLogs) != 50 {
		t.Errorf("recent logs len = %d, want 50", len(snap.RecentLogs))
	}
}
