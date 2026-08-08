package mock

import (
	"fmt"
	"testing"
)

// ---------------------------------------------------------------------------
// Specificity ordering (MAT-004)
// ---------------------------------------------------------------------------

func TestSpecificity_ExactBeatsGlobBeatsRegex(t *testing.T) {
	r := NewRegistry()
	mustAdd(t, r, &Endpoint{ID: "regex", PathRegex: `^/users/.*`})
	mustAdd(t, r, &Endpoint{ID: "glob", PathGlob: "/users/*"})
	mustAdd(t, r, &Endpoint{ID: "exact", Path: "/users/me"})

	ep := r.Match("GET", "/users/me", nil, nil, nil)
	if ep == nil || ep.ID != "exact" {
		t.Fatalf("exact path must win, got %v", epID(ep))
	}
}

func TestSpecificity_FewerWildcardGlobWins(t *testing.T) {
	r := NewRegistry()
	mustAdd(t, r, &Endpoint{ID: "catchall", PathGlob: "/users/**"})     // weight 2
	mustAdd(t, r, &Endpoint{ID: "narrow", PathGlob: "/users/*/orders"}) // weight 1

	ep := r.Match("GET", "/users/1/orders", nil, nil, nil)
	if ep == nil || ep.ID != "narrow" {
		t.Fatalf("fewer-wildcard glob must win, got %v", epID(ep))
	}
}

func TestSpecificity_GlobBeatsRegex(t *testing.T) {
	r := NewRegistry()
	mustAdd(t, r, &Endpoint{ID: "regex", PathRegex: `^/users/[a-z]+$`})
	mustAdd(t, r, &Endpoint{ID: "glob", PathGlob: "/users/*"})

	ep := r.Match("GET", "/users/ada", nil, nil, nil)
	if ep == nil || ep.ID != "glob" {
		t.Fatalf("glob must beat regex (class order), got %v", epID(ep))
	}
}

func TestSpecificity_LoadOrderTiebreakDeterministic(t *testing.T) {
	r := NewRegistry()
	// Identical specificity (same class, same wildcard count) — load order wins.
	mustAdd(t, r, &Endpoint{ID: "first", PathGlob: "/users/*"})
	mustAdd(t, r, &Endpoint{ID: "second", PathGlob: "/users/*"})

	want := "first"
	for i := 0; i < 100; i++ {
		ep := r.Match("GET", "/users/123", nil, nil, nil)
		if ep == nil || ep.ID != want {
			t.Fatalf("iteration %d: expected %s to win deterministically, got %v", i, want, epID(ep))
		}
	}
}

func TestSpecificity_SecondRegistrySameOrderDeterministic(t *testing.T) {
	// Rebuilding the same endpoints in the same order must produce the same
	// winner (guards against hidden map-order influence).
	for i := 0; i < 50; i++ {
		r := NewRegistry()
		mustAdd(t, r, &Endpoint{ID: "a", PathGlob: "/api/*"})
		mustAdd(t, r, &Endpoint{ID: "b", PathGlob: "/api/*"})
		ep := r.Match("GET", "/api/x", nil, nil, nil)
		if ep == nil || ep.ID != "a" {
			t.Fatalf("registry %d: expected a to win, got %v", i, epID(ep))
		}
	}
}

func TestSpecificity_SeqPreservedOnUpdate(t *testing.T) {
	// D8 — a hot-reload update keeps the original seq slot, so an updated
	// endpoint does not jump the load-order queue.
	r := NewRegistry()
	mustAdd(t, r, &Endpoint{ID: "a", PathGlob: "/users/*"})
	mustAdd(t, r, &Endpoint{ID: "b", PathGlob: "/users/*"})

	// Hot-reload "a" (the watcher's duplicate→Update path).
	if err := r.Update("a", &Endpoint{PathGlob: "/users/*"}); err != nil {
		t.Fatalf("update: %v", err)
	}

	ep := r.Match("GET", "/users/7", nil, nil, nil)
	if ep == nil || ep.ID != "a" {
		t.Fatalf("updated endpoint must keep its original seq slot, got %v", epID(ep))
	}
}

func TestSpecificity_NonCandidateExcluded(t *testing.T) {
	// An endpoint that fails ANY constraint is not a candidate — even if it
	// would otherwise be the most specific.
	r := NewRegistry()
	mustAdd(t, r, &Endpoint{ID: "exact", Method: "POST", Path: "/users/me"})
	mustAdd(t, r, &Endpoint{ID: "glob", PathGlob: "/users/*"})

	// GET /users/me: the exact POST stub is NOT a candidate (method mismatch),
	// so the glob stub wins.
	ep := r.Match("GET", "/users/me", nil, nil, nil)
	if ep == nil || ep.ID != "glob" {
		t.Fatalf("constraint-failing exact stub must be excluded, got %v", epID(ep))
	}

	// POST /users/me: now the exact stub is a candidate and wins.
	ep = r.Match("POST", "/users/me", nil, nil, nil)
	if ep == nil || ep.ID != "exact" {
		t.Fatalf("exact stub must win when constraints pass, got %v", epID(ep))
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustAdd(t *testing.T, r *Registry, ep *Endpoint) {
	t.Helper()
	if err := r.Add(ep); err != nil {
		t.Fatalf("add %s: %v", ep.ID, err)
	}
}

func epID(ep *Endpoint) string {
	if ep == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%q", ep.ID)
}
