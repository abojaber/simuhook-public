package mock

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
)

// ErrDuplicate is returned by Registry.Add when an endpoint ID already exists.
// The watcher branches on errors.Is(err, ErrDuplicate) to distinguish a
// hot-reload update from a real load error (MAT-005).
var ErrDuplicate = errors.New("endpoint already exists")

type Registry struct {
	mu        sync.RWMutex
	endpoints map[string]*Endpoint
	nextSeq   uint64 // monotonic load-order counter (MAT-004/D8)
}

func NewRegistry() *Registry {
	return &Registry{
		endpoints: make(map[string]*Endpoint),
	}
}

// Add stores a new endpoint. The endpoint is compiled (path glob pre-split,
// regex compile, exclusivity checks) BEFORE it is stored — a compile error
// returns with nothing stored and seq not bumped. seq is assigned here in
// load order and preserved by Update (D8).
func (r *Registry) Add(ep *Endpoint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.endpoints[ep.ID]; exists {
		return fmt.Errorf("endpoint %s: %w", ep.ID, ErrDuplicate)
	}
	if err := compileEndpoint(ep); err != nil {
		return fmt.Errorf("endpoint %s: %w", ep.ID, err)
	}
	ep.seq = r.nextSeq
	r.nextSeq++
	r.endpoints[ep.ID] = ep
	return nil
}

// Update replaces an existing endpoint under the same ID. The replacement is
// compiled first — on error the registry is left unchanged. The original
// endpoint's seq slot is preserved so a hot-reload (watcher duplicate→Update)
// does not change load-order tie-breaking (D8).
func (r *Registry) Update(id string, ep *Endpoint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	old, exists := r.endpoints[id]
	if !exists {
		return fmt.Errorf("endpoint %s not found", id)
	}
	ep.ID = id
	if err := compileEndpoint(ep); err != nil {
		return fmt.Errorf("endpoint %s: %w", id, err)
	}
	ep.seq = old.seq
	r.endpoints[id] = ep
	return nil
}

func (r *Registry) Remove(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.endpoints[id]; !exists {
		return fmt.Errorf("endpoint %s not found", id)
	}
	delete(r.endpoints, id)
	return nil
}

func (r *Registry) Get(id string) (*Endpoint, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ep, ok := r.endpoints[id]
	if !ok {
		return nil, fmt.Errorf("endpoint %s not found", id)
	}
	return ep, nil
}

func (r *Registry) List() []*Endpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()
	eps := make([]*Endpoint, 0, len(r.endpoints))
	for _, ep := range r.endpoints {
		eps = append(eps, ep)
	}
	return eps
}

// Match returns the most specific endpoint matching the request, or nil.
// All constraints (method, path, headers, query, payload/payload_contains)
// are AND-ed before specificity comparison — an endpoint that fails any
// constraint is not a candidate (MAT-004). The request body is parsed as
// JSON at most once per Match call, lazily, only when a payload_contains
// candidate exists (MAT-003).
func (r *Registry) Match(method, path string, headers, query map[string]string, body []byte) *Endpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var (
		parsed   any
		parsedOK bool
		parsedDo bool
		best     *Endpoint
	)
	for _, ep := range r.endpoints {
		if !matchMethod(ep.Method, method) {
			continue
		}
		if !ep.matchPath(path) {
			continue
		}
		if !matchHeaders(ep.Headers, headers) {
			continue
		}
		if !matchQuery(ep.Query, query) {
			continue
		}
		if ep.hasPayloadContains {
			if !parsedDo {
				parsedDo = true
				parsed, parsedOK = parseJSONBody(body)
			}
			if !parsedOK || !subsetMatch(ep.PayloadContains, parsed) {
				continue
			}
		} else if !matchPayload(ep.Payload, body) {
			continue
		}
		if best == nil || better(ep, best) {
			best = ep
		}
	}
	return best
}

// better reports whether endpoint a outranks endpoint b under the MAT-004
// specificity order: path class (exact 0 < glob 1 < regex 2) → glob wildcard
// weight (fewer wins, D1) → load order (seq, D8). The comparison is a total
// order, so the winner never depends on map iteration order.
func better(a, b *Endpoint) bool {
	ca, cb := pathClass(a), pathClass(b)
	if ca != cb {
		return ca < cb
	}
	// Both glob-class: fewer wildcards wins (D1). Regex-class endpoints have
	// no comparable weight, so they fall straight to load order.
	if ca == 1 && a.globWeight != b.globWeight {
		return a.globWeight < b.globWeight
	}
	return a.seq < b.seq
}

func matchMethod(want, got string) bool {
	return want == "" || want == got
}

func matchHeaders(want, got map[string]string) bool {
	if len(want) == 0 {
		return true
	}
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}

func matchQuery(want, got map[string]string) bool {
	if len(want) == 0 {
		return true
	}
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}

func matchPayload(want, got []byte) bool {
	if len(want) == 0 {
		return true
	}
	return string(want) == string(got)
}

// PickResponse selects a response variant for the given endpoint.
// If the endpoint has multiple responses, it uses weighted random selection
// (if weights are present) or round-robin (if no weights).
// Falls back to the singular Response if no variants are configured.
// The second return value is the 0-based index into ep.Responses that was
// selected, or -1 when the singular ep.Response was used (no variants
// configured) — see LogEntry.ResponseVariant.
func PickResponse(ep *Endpoint) (*Response, int) {
	if len(ep.Responses) == 0 {
		return &ep.Response, -1
	}

	if hasWeights(ep.Responses) {
		return pickWeighted(ep.Responses)
	}

	// Round-robin
	idx := atomic.AddUint64(&ep.rrIndex, 1) - 1
	i := int(idx % uint64(len(ep.Responses)))
	return &ep.Responses[i], i
}

func hasWeights(responses []Response) bool {
	for _, r := range responses {
		if r.Weight > 0 {
			return true
		}
	}
	return false
}

func pickWeighted(responses []Response) (*Response, int) {
	total := 0
	for _, r := range responses {
		total += r.Weight
	}
	if total <= 0 {
		i := rand.Intn(len(responses))
		return &responses[i], i
	}
	n := rand.Intn(total)
	for i, r := range responses {
		n -= r.Weight
		if n < 0 {
			return &responses[i], i
		}
	}
	i := len(responses) - 1
	return &responses[i], i
}
