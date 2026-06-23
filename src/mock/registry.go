package mock

import (
	"fmt"
	"sync"
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
