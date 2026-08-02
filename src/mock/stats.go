package mock

import (
	"math"
	"sync"
	"time"
)

// StatsTracker tracks request statistics and keeps a ring buffer of recent logs.
type StatsTracker struct {
	mu          sync.RWMutex
	startTime   time.Time
	hitCounts   map[string]uint64
	recentLogs  []LogEntry
	maxLogs     int
	nextIndex   int
	full        bool
	total       uint64
	unmatched   uint64
	overall     latencyAgg
	perEndpoint map[string]*latencyAgg
	notify      chan struct{}
}

// NewStatsTracker creates a StatsTracker with the given ring buffer size.
func NewStatsTracker(maxLogs int) *StatsTracker {
	return &StatsTracker{
		startTime:   time.Now(),
		hitCounts:   make(map[string]uint64),
		recentLogs:  make([]LogEntry, maxLogs),
		maxLogs:     maxLogs,
		perEndpoint: make(map[string]*latencyAgg),
		notify:      make(chan struct{}, 1),
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

	// latency tracking
	s.total++
	if entry.Webhook == "" {
		s.unmatched++
		s.overall.observe(entry.LatencyMs)
	} else {
		s.overall.observe(entry.LatencyMs)
		if s.perEndpoint[entry.Webhook] == nil {
			s.perEndpoint[entry.Webhook] = &latencyAgg{}
		}
		s.perEndpoint[entry.Webhook].observe(entry.LatencyMs)
	}

	// non-blocking coalescing notify
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

// StatsSnapshot is a point-in-time copy of the tracker state.
type StatsSnapshot struct {
	Uptime          time.Duration
	HitCounts       map[string]uint64
	RecentLogs      []LogEntry
	TotalLogs       uint64
	TotalRequests   uint64
	Latency         LatencyStats
	EndpointLatency map[string]LatencyStats
	UnmatchedCount  uint64
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

	epLat := make(map[string]LatencyStats, len(s.perEndpoint))
	for k, v := range s.perEndpoint {
		epLat[k] = v.stats()
	}

	return StatsSnapshot{
		Uptime:          time.Since(s.startTime),
		HitCounts:       counts,
		RecentLogs:      logs,
		TotalLogs:       total,
		TotalRequests:   s.total,
		Latency:         s.overall.stats(),
		EndpointLatency: epLat,
		UnmatchedCount:  s.unmatched,
	}
}

// Uptime returns the time since the tracker was created.
func (s *StatsTracker) Uptime() time.Duration {
	return time.Since(s.startTime)
}

// Notify returns a channel that receives a value after each recorded request.
// Sends are non-blocking and coalescing: if a value is already pending it is
// dropped, so a slow or absent consumer never blocks RecordRequest. Because
// consumers are expected to re-read Snapshot(), a dropped notification loses
// no information.
func (s *StatsTracker) Notify() <-chan struct{} {
	return s.notify
}

// ---------------------------------------------------------------------------
// Latency aggregation
// ---------------------------------------------------------------------------

// LatencyStats summarises response latency in milliseconds.
// The zero value is valid and means "no requests recorded".
type LatencyStats struct {
	Count uint64  `json:"count"`
	Sum   float64 `json:"sum_ms"`
	Min   float64 `json:"min_ms"`
	Max   float64 `json:"max_ms"`
	P95   float64 `json:"p95_ms"` // approximate; exact to ~19%
}

// Avg returns the mean latency, or 0 when no requests were recorded.
func (l LatencyStats) Avg() float64 {
	if l.Count == 0 {
		return 0
	}
	return l.Sum / float64(l.Count)
}

const (
	histMinMs      = 0.01
	histSubBuckets = 4
	histOctaves    = 23
	histBuckets    = histSubBuckets * histOctaves // 92
)

// subBucketStep is 2^(1/histSubBuckets) − 1, pre-computed for efficiency.
var subBucketStep = math.Pow(2, 1.0/histSubBuckets) - 1.0

// latencyAgg accumulates latency observations. NOT safe for concurrent use;
// callers must hold the owning StatsTracker's lock.
type latencyAgg struct {
	count   uint64
	sum     float64
	min     float64
	max     float64
	buckets [histBuckets]uint64
}

// observe records a single latency value. Non-finite (NaN/Inf) and negative
// values are silently ignored.
func (a *latencyAgg) observe(ms float64) {
	if ms <= 0 || math.IsNaN(ms) || math.IsInf(ms, 0) {
		return
	}
	a.count++
	a.sum += ms
	if a.count == 1 {
		a.min = ms
		a.max = ms
	} else {
		a.min = math.Min(a.min, ms)
		a.max = math.Max(a.max, ms)
	}
	idx := bucketIndex(ms)
	a.buckets[idx]++
}

// bucketIndex returns the histogram bucket index for a latency in milliseconds.
// Values below histMinMs are clamped to bucket 0; values above the top boundary
// are clamped to the last bucket.
func bucketIndex(ms float64) int {
	if ms < histMinMs {
		return 0
	}
	octave := int(math.Floor(math.Log2(ms / histMinMs)))
	if octave < 0 {
		octave = 0
	}
	if octave >= histOctaves {
		return histBuckets - 1
	}

	// fraction ∈ [1, 2)
	octaveLow := histMinMs * math.Pow(2, float64(octave))
	fraction := ms / octaveLow // ≥ 1
	sub := int(math.Floor((fraction - 1) / subBucketStep))
	if sub >= histSubBuckets {
		sub = histSubBuckets - 1
	}

	idx := histSubBuckets*octave + sub
	if idx >= histBuckets {
		return histBuckets - 1
	}
	return idx
}

// stats returns the aggregated LatencyStats. Returns a zero-value LatencyStats
// when no observations have been recorded.
func (a *latencyAgg) stats() LatencyStats {
	if a.count == 0 {
		return LatencyStats{}
	}
	p95 := a.computeP95()
	// Clamp P95 to [Min, Max] to guard against histogram approximation drift.
	if p95 < a.min {
		p95 = a.min
	}
	if p95 > a.max {
		p95 = a.max
	}
	return LatencyStats{
		Count: a.count,
		Sum:   a.sum,
		Min:   a.min,
		Max:   a.max,
		P95:   p95,
	}
}

// computeP95 estimates the 95th percentile using linear interpolation inside
// the histogram bucket that contains the rank. Returns 0 when count == 0
// (caller must guard against this).
func (a *latencyAgg) computeP95() float64 {
	rank := 0.95 * float64(a.count)
	var cum uint64
	var prevCum uint64
	for i := 0; i < histBuckets; i++ {
		prevCum = cum
		cum += a.buckets[i]
		if float64(cum) >= rank {
			countInBucket := a.buckets[i]
			lower := bucketLowerBound(i)
			upper := bucketUpperBound(i)
			if countInBucket == 0 || cum == prevCum {
				return lower
			}
			return lower + (upper-lower)*(rank-float64(prevCum))/float64(countInBucket)
		}
	}
	// Should not reach here if count > 0.
	return a.max
}

// bucketLowerBound returns the inclusive lower boundary of histogram bucket i.
func bucketLowerBound(i int) float64 {
	return histMinMs * math.Pow(2, float64(i)/histSubBuckets)
}

// bucketUpperBound returns the exclusive upper boundary of histogram bucket i.
func bucketUpperBound(i int) float64 {
	return histMinMs * math.Pow(2, float64(i+1)/histSubBuckets)
}
