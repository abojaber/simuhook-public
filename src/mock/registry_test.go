package mock

import (
	"fmt"
	"sync"
	"testing"
)

// TestRegistry_AddConcurrentWithMatch verifies that concurrent Add and Match
// operations are safe under -race (Registry uses an RWMutex).
func TestRegistry_AddConcurrentWithMatch(t *testing.T) {
	r := NewRegistry()

	// Seed one endpoint so Match has something to do during the test.
	r.Add(&Endpoint{ID: "seed", Method: "GET", Path: "/seed", Response: Response{StatusCode: 200}})

	var wg sync.WaitGroup

	// Writers: add many endpoints concurrently.
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				id := fmt.Sprintf("ep-%d-%d", w, i)
				ep := &Endpoint{ID: id, Method: "GET", Path: "/" + id, Response: Response{StatusCode: 200}}
				_ = r.Add(ep) // duplicates may collide, that's fine
			}
		}(w)
	}

	// Readers: match continuously.
	for rIdx := 0; rIdx < 4; rIdx++ {
		wg.Add(1)
		go func(rIdx int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				_ = r.Match("GET", "/seed", nil, nil, nil)
				_ = r.Match("GET", fmt.Sprintf("/ep-%d-%d", rIdx%4, i%200), nil, nil, nil)
			}
		}(rIdx)
	}

	wg.Wait()

	// Verify writes took effect.
	if _, err := r.Get("ep-0-0"); err != nil {
		t.Fatalf("expected ep-0-0 to exist after concurrent adds: %v", err)
	}
	if _, err := r.Get("ep-3-199"); err != nil {
		t.Fatalf("expected ep-3-199 to exist after concurrent adds: %v", err)
	}
}

// TestRegistry_UpdateRemoveConcurrentWithMatch verifies concurrent
// Update/Remove alongside Match is safe under -race.
func TestRegistry_UpdateRemoveConcurrentWithMatch(t *testing.T) {
	r := NewRegistry()

	// Seed a set of endpoints that writers will update/remove.
	for i := 0; i < 50; i++ {
		id := fmt.Sprintf("ep-%d", i)
		r.Add(&Endpoint{ID: id, Method: "GET", Path: "/" + id, Response: Response{StatusCode: 200}})
	}

	var wg sync.WaitGroup

	// Writers: update and remove concurrently.
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				id := fmt.Sprintf("ep-%d", i%50)
				if i%2 == 0 {
					_ = r.Update(id, &Endpoint{Method: "POST", Path: "/updated"})
				} else {
					_ = r.Remove(id)
					// Re-add so later iterations still have something to hit.
					_ = r.Add(&Endpoint{ID: id, Method: "GET", Path: "/" + id, Response: Response{StatusCode: 200}})
				}
			}
		}(w)
	}

	// Readers: match continuously.
	for rIdx := 0; rIdx < 4; rIdx++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				_ = r.Match("GET", fmt.Sprintf("/ep-%d", i%50), nil, nil, nil)
				_ = r.Match("POST", "/updated", nil, nil, nil)
			}
		}()
	}

	wg.Wait()
}

// TestRegistry_ListConcurrentWithMatch verifies List is safe alongside Match
// under concurrent access.
func TestRegistry_ListConcurrentWithMatch(t *testing.T) {
	r := NewRegistry()
	for i := 0; i < 20; i++ {
		id := fmt.Sprintf("ep-%d", i)
		r.Add(&Endpoint{ID: id, Method: "GET", Path: "/" + id, Response: Response{StatusCode: 200}})
	}

	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				_ = r.List()
				_ = r.Match("GET", "/ep-5", nil, nil, nil)
			}
		}()
	}
	wg.Wait()

	if len(r.List()) != 20 {
		t.Fatalf("expected 20 endpoints, got %d", len(r.List()))
	}
}
