package mock

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
)

type Registry struct {
	mu        sync.RWMutex
	endpoints map[string]*Endpoint
}

func NewRegistry() *Registry {
	return &Registry{
		endpoints: make(map[string]*Endpoint),
	}
}

func (r *Registry) Add(ep *Endpoint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.endpoints[ep.ID]; exists {
		return fmt.Errorf("endpoint %s already exists", ep.ID)
	}
	r.endpoints[ep.ID] = ep
	return nil
}

func (r *Registry) Update(id string, ep *Endpoint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.endpoints[id]; !exists {
		return fmt.Errorf("endpoint %s not found", id)
	}
	ep.ID = id
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

func (r *Registry) Match(method, path string, headers, query map[string]string, body []byte) *Endpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, ep := range r.endpoints {
		if !matchMethod(ep.Method, method) {
			continue
		}
		if !matchPath(ep.Path, path) {
			continue
		}
		if !matchHeaders(ep.Headers, headers) {
			continue
		}
		if !matchQuery(ep.Query, query) {
			continue
		}
		if !matchPayload(ep.Payload, body) {
			continue
		}
		return ep
	}
	return nil
}

func matchMethod(want, got string) bool {
	return want == "" || want == got
}

func matchPath(want, got string) bool {
	return want == got
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
func PickResponse(ep *Endpoint) *Response {
	if len(ep.Responses) == 0 {
		return &ep.Response
	}

	if hasWeights(ep.Responses) {
		return pickWeighted(ep.Responses)
	}

	// Round-robin
	idx := atomic.AddUint64(&ep.rrIndex, 1) - 1
	return &ep.Responses[idx%uint64(len(ep.Responses))]
}

func hasWeights(responses []Response) bool {
	for _, r := range responses {
		if r.Weight > 0 {
			return true
		}
	}
	return false
}

func pickWeighted(responses []Response) *Response {
	total := 0
	for _, r := range responses {
		total += r.Weight
	}
	if total <= 0 {
		return &responses[rand.Intn(len(responses))]
	}
	n := rand.Intn(total)
	for i, r := range responses {
		n -= r.Weight
		if n < 0 {
			return &responses[i]
		}
	}
	return &responses[len(responses)-1]
}
