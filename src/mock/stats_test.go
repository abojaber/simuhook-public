package mock

import (
	"math"
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
	if snap.TotalRequests != 2000 {
		t.Errorf("TotalRequests = %d, want 2000", snap.TotalRequests)
	}
	if snap.Latency.Count != 2000 {
		t.Errorf("Latency.Count = %d, want 2000", snap.Latency.Count)
	}
}

// testLogEntryWithLatency creates a log entry with the given latency.
func testLogEntryWithLatency(webhook, path string, lat float64) LogEntry {
	e := testLogEntry(webhook, path)
	e.LatencyMs = lat
	return e
}

// TestLatency_ExactAccumulators verifies that Count/Sum/Min/Max are exact
// for a known set of latency values.
func TestLatency_ExactAccumulators(t *testing.T) {
	aggs := []float64{5.0, 2.0, 10.0, 7.0}
	s := NewStatsTracker(10)
	for _, v := range aggs {
		s.RecordRequest(testLogEntryWithLatency("ep", "/ep", v))
	}
	snap := s.Snapshot()

	wantCount := uint64(4)
	wantSum := 24.0
	wantMin := 2.0
	wantMax := 10.0

	if snap.TotalRequests != wantCount {
		t.Errorf("TotalRequests = %d, want %d", snap.TotalRequests, wantCount)
	}
	if snap.Latency.Count != wantCount {
		t.Errorf("Latency.Count = %d, want %d", snap.Latency.Count, wantCount)
	}
	if snap.Latency.Sum != wantSum {
		t.Errorf("Latency.Sum = %f, want %f", snap.Latency.Sum, wantSum)
	}
	if snap.Latency.Min != wantMin {
		t.Errorf("Latency.Min = %f, want %f", snap.Latency.Min, wantMin)
	}
	if snap.Latency.Max != wantMax {
		t.Errorf("Latency.Max = %f, want %f", snap.Latency.Max, wantMax)
	}
}

// TestLatency_AggregatesBeyondRingBuffer is the headline acceptance criterion:
// latency aggregates cover ALL requests even when the ring buffer discards logs.
func TestLatency_AggregatesBeyondRingBuffer(t *testing.T) {
	const maxLogs = 5
	const totalRequests = 200
	s := NewStatsTracker(maxLogs)
	for i := 0; i < totalRequests; i++ {
		// Vary latencies so we exercise the histogram meaningfully.
		s.RecordRequest(testLogEntryWithLatency("ep", "/ep", float64(i%50)+1.0))
	}
	snap := s.Snapshot()

	if snap.TotalLogs != maxLogs {
		t.Errorf("TotalLogs = %d, want %d (ring-capped)", snap.TotalLogs, maxLogs)
	}
	if snap.TotalRequests != totalRequests {
		t.Errorf("TotalRequests = %d, want %d (lifetime total)", snap.TotalRequests, totalRequests)
	}
	if snap.Latency.Count != totalRequests {
		t.Errorf("Latency.Count = %d, want %d", snap.Latency.Count, totalRequests)
	}
	if len(snap.RecentLogs) != maxLogs {
		t.Errorf("len(RecentLogs) = %d, want %d", len(snap.RecentLogs), maxLogs)
	}
}

// TestLatency_ZeroRecords verifies that zero records produce a zero-value
// LatencyStats with no NaN or Inf on any field.
func TestLatency_ZeroRecords(t *testing.T) {
	s := NewStatsTracker(10)
	snap := s.Snapshot()

	if snap.TotalRequests != 0 {
		t.Errorf("TotalRequests = %d, want 0", snap.TotalRequests)
	}
	if snap.Latency.Count != 0 {
		t.Errorf("Latency.Count = %d, want 0", snap.Latency.Count)
	}
	if snap.Latency.Sum != 0 {
		t.Errorf("Latency.Sum = %f, want 0", snap.Latency.Sum)
	}
	if snap.Latency.Min != 0 {
		t.Errorf("Latency.Min = %f, want 0", snap.Latency.Min)
	}
	if snap.Latency.Max != 0 {
		t.Errorf("Latency.Max = %f, want 0", snap.Latency.Max)
	}
	if snap.Latency.P95 != 0 {
		t.Errorf("Latency.P95 = %f, want 0", snap.Latency.P95)
	}
	avg := snap.Latency.Avg()
	if avg != 0 {
		t.Errorf("Latency.Avg() = %f, want 0", avg)
	}
	if math.IsNaN(snap.Latency.Sum) {
		t.Error("Latency.Sum is NaN")
	}
	if math.IsNaN(snap.Latency.P95) {
		t.Error("Latency.P95 is NaN")
	}
	if math.IsInf(snap.Latency.Sum, 0) {
		t.Error("Latency.Sum is Inf")
	}
}

// TestLatency_SingleRecord verifies Min == Max == P95 for a single observation.
func TestLatency_SingleRecord(t *testing.T) {
	const v = 42.5
	s := NewStatsTracker(10)
	s.RecordRequest(testLogEntryWithLatency("ep", "/ep", v))
	snap := s.Snapshot()

	if snap.Latency.Count != 1 {
		t.Errorf("Count = %d, want 1", snap.Latency.Count)
	}
	if snap.Latency.Min != v {
		t.Errorf("Min = %f, want %f", snap.Latency.Min, v)
	}
	if snap.Latency.Max != v {
		t.Errorf("Max = %f, want %f", snap.Latency.Max, v)
	}
	if snap.Latency.P95 != v {
		t.Errorf("P95 = %f, want %f", snap.Latency.P95, v)
	}
}

// TestLatency_P95KnownDistribution verifies that P95 falls within the expected
// range for a uniform distribution of 1..100 ms.
func TestLatency_P95KnownDistribution(t *testing.T) {
	s := NewStatsTracker(200)
	for i := 1.0; i <= 100.0; i++ {
		s.RecordRequest(testLogEntryWithLatency("ep", "/ep", i))
	}
	snap := s.Snapshot()

	// Sanity: Min and Max must be correct.
	if snap.Latency.Min != 1.0 {
		t.Errorf("Min = %f, want 1.0", snap.Latency.Min)
	}
	if snap.Latency.Max != 100.0 {
		t.Errorf("Max = %f, want 100.0", snap.Latency.Max)
	}
	// P95 must be clamped between Min and Max.
	if snap.Latency.P95 < snap.Latency.Min {
		t.Errorf("P95 = %f < Min = %f", snap.Latency.P95, snap.Latency.Min)
	}
	if snap.Latency.P95 > snap.Latency.Max {
		t.Errorf("P95 = %f > Max = %f", snap.Latency.P95, snap.Latency.Max)
	}
	// P95 should be roughly around 95 ± 20 (allowing ~19% bucket tolerance).
	if snap.Latency.P95 < 70 || snap.Latency.P95 > 110 {
		t.Errorf("P95 = %f, expected roughly 70–110 for uniform 1–100", snap.Latency.P95)
	}
}

// TestLatency_PerEndpointIsolation verifies that per-endpoint latency stats
// are independent and do not leak between endpoints.
func TestLatency_PerEndpointIsolation(t *testing.T) {
	s := NewStatsTracker(50)

	// ep1: 3 requests, slow
	for i := 0; i < 3; i++ {
		s.RecordRequest(testLogEntryWithLatency("ep1", "/ep1", 100.0))
	}
	// ep2: 2 requests, fast
	for i := 0; i < 2; i++ {
		s.RecordRequest(testLogEntryWithLatency("ep2", "/ep2", 10.0))
	}

	snap := s.Snapshot()

	// Overall covers both.
	if snap.Latency.Count != 5 {
		t.Errorf("overall Count = %d, want 5", snap.Latency.Count)
	}
	// Averages: ep1 ≈ 100, ep2 ≈ 10.
	ep1 := snap.EndpointLatency["ep1"]
	ep2 := snap.EndpointLatency["ep2"]

	if ep1.Count != 3 {
		t.Errorf("ep1.Count = %d, want 3", ep1.Count)
	}
	if ep2.Count != 2 {
		t.Errorf("ep2.Count = %d, want 2", ep2.Count)
	}
	if ep1.Min != 100.0 || ep1.Max != 100.0 {
		t.Errorf("ep1 Min/Max = %f/%f, want 100/100", ep1.Min, ep1.Max)
	}
	if ep2.Min != 10.0 || ep2.Max != 10.0 {
		t.Errorf("ep2 Min/Max = %f/%f, want 10/10", ep2.Min, ep2.Max)
	}
	// Make sure a non-existent endpoint isn't present.
	if _, ok := snap.EndpointLatency["nonexistent"]; ok {
		t.Error("EndpointLatency contains unexpected key 'nonexistent'")
	}
}

// TestLatency_UnmatchedAccounting verifies that unmatched requests are counted
// in the overall Latency/UnmatchedCount but excluded from EndpointLatency.
func TestLatency_UnmatchedAccounting(t *testing.T) {
	s := NewStatsTracker(50)

	// 3 matched, 2 unmatched.
	s.RecordRequest(testLogEntryWithLatency("ep", "/ep", 5.0))
	s.RecordRequest(testLogEntryWithLatency("", "/unknown", 1.0))
	s.RecordRequest(testLogEntryWithLatency("ep", "/ep", 10.0))
	s.RecordRequest(testLogEntryWithLatency("", "/other", 2.0))
	s.RecordRequest(testLogEntryWithLatency("ep", "/ep", 7.0))

	snap := s.Snapshot()

	if snap.TotalRequests != 5 {
		t.Errorf("TotalRequests = %d, want 5", snap.TotalRequests)
	}
	if snap.UnmatchedCount != 2 {
		t.Errorf("UnmatchedCount = %d, want 2", snap.UnmatchedCount)
	}
	if snap.Latency.Count != 5 {
		t.Errorf("Latency.Count = %d, want 5 (includes unmatched)", snap.Latency.Count)
	}
	// Unmatched must not appear in EndpointLatency.
	if _, ok := snap.EndpointLatency[""]; ok {
		t.Error("EndpointLatency contains empty-key entry for unmatched")
	}
	if snap.EndpointLatency["ep"].Count != 3 {
		t.Errorf("EndpointLatency[ep].Count = %d, want 3", snap.EndpointLatency["ep"].Count)
	}
	if len(snap.EndpointLatency) != 1 {
		t.Errorf("len(EndpointLatency) = %d, want 1 (only ep)", len(snap.EndpointLatency))
	}
}

// TestLatency_IgnoresNonFinite verifies that NaN, +Inf, -Inf, and negative
// latencies do not corrupt the accumulators.
func TestLatency_IgnoresNonFinite(t *testing.T) {
	s := NewStatsTracker(20)

	// Good values.
	s.RecordRequest(testLogEntryWithLatency("ep", "/ep", 10.0))
	s.RecordRequest(testLogEntryWithLatency("ep", "/ep", 20.0))

	// Bad values.
	s.RecordRequest(testLogEntryWithLatency("ep", "/ep", math.NaN()))
	s.RecordRequest(testLogEntryWithLatency("ep", "/ep", math.Inf(1)))
	s.RecordRequest(testLogEntryWithLatency("ep", "/ep", math.Inf(-1)))
	s.RecordRequest(testLogEntryWithLatency("ep", "/ep", -5.0))
	s.RecordRequest(testLogEntryWithLatency("ep", "/ep", 0.0))

	snap := s.Snapshot()

	if snap.Latency.Count != 2 {
		t.Errorf("Latency.Count = %d, want 2 (bad values ignored)", snap.Latency.Count)
	}
	if snap.Latency.Sum != 30.0 {
		t.Errorf("Latency.Sum = %f, want 30.0", snap.Latency.Sum)
	}
	if snap.Latency.Min != 10.0 {
		t.Errorf("Latency.Min = %f, want 10.0", snap.Latency.Min)
	}
	if snap.Latency.Max != 20.0 {
		t.Errorf("Latency.Max = %f, want 20.0", snap.Latency.Max)
	}
}

// TestNotify_CoalescesAndNeverBlocks verifies that the notification channel
// never blocks RecordRequest and coalesces multiple notifications.
func TestNotify_CoalescesAndNeverBlocks(t *testing.T) {
	s := NewStatsTracker(10)

	// Record 100 requests with no consumer. Must not block.
	const n = 100
	for i := 0; i < n; i++ {
		s.RecordRequest(testLogEntryWithLatency("ep", "/ep", 1.0))
	}

	// Drain exactly one notification (coalesced from the 100 records).
	select {
	case <-s.Notify():
		// ok — at least one notification was pending.
	default:
		t.Error("expected a pending notification on channel")
	}

	// No more pending (the rest were coalesced/dropped).
	select {
	case <-s.Notify():
		t.Error("unexpected second notification — coalescing failed")
	default:
		// ok
	}
}
