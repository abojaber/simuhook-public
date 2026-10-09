package mock

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// globMatch is the pure glob matcher over two path strings.
func globMatch(pattern, path string) bool {
	return matchGlobSegs(splitPathSegs(pattern), splitPathSegs(path))
}

// mustCompile compiles an endpoint, failing the test on error.
func mustCompile(t *testing.T, ep *Endpoint) *Endpoint {
	t.Helper()
	if err := compileEndpoint(ep); err != nil {
		t.Fatalf("compileEndpoint: %v", err)
	}
	return ep
}

// ---------------------------------------------------------------------------
// splitPathSegs edge cases (MAT-001 / D5)
// ---------------------------------------------------------------------------

func TestSplitPathSegs_EdgeCases(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"root", "/", nil},
		{"empty", "", nil},
		{"single segment", "/users", []string{"users"}},
		{"trailing slash trimmed", "/users/", []string{"users"}},
		{"leading slash trimmed", "/users/123", []string{"users", "123"}},
		{"interior empty preserved", "/a//b", []string{"a", "", "b"}},
		{"all trailing slashes trimmed", "/a/b//", []string{"a", "b"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := splitPathSegs(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("splitPathSegs(%q) = %#v, want %#v", tc.in, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("splitPathSegs(%q) = %#v, want %#v", tc.in, got, tc.want)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Glob matcher (MAT-001)
// ---------------------------------------------------------------------------

func TestMatchGlob_SingleStar(t *testing.T) {
	if !globMatch("/users/*", "/users/123") {
		t.Error("/users/* should match /users/123")
	}
	if globMatch("/users/*", "/users/123/orders") {
		t.Error("/users/* must NOT match /users/123/orders (extra segment)")
	}
	if globMatch("/users/*", "/users") {
		t.Error("/users/* must NOT match /users (the * requires exactly one segment)")
	}
	if !globMatch("/users/*", "/users/ada") {
		t.Error("/users/* should match /users/ada")
	}
}

func TestMatchGlob_DoubleStar(t *testing.T) {
	if !globMatch("/users/**", "/users/123") {
		t.Error("/users/** should match /users/123")
	}
	if !globMatch("/users/**", "/users/123/orders") {
		t.Error("/users/** should match /users/123/orders")
	}
	if !globMatch("/users/**", "/users") {
		t.Error("/users/** should match /users (** may match zero segments)")
	}
}

func TestMatchGlob_DoubleStarMiddle(t *testing.T) {
	// ** in the middle spans zero or more segments.
	for _, p := range []string{"/a/b", "/a/x/b", "/a/x/y/b"} {
		if !globMatch("/a/**/b", p) {
			t.Errorf("/a/**/b should match %s", p)
		}
	}
	if globMatch("/a/**/b", "/a/b/c") {
		t.Error("/a/**/b must NOT match /a/b/c (trailing segment unmatched)")
	}
}

func TestMatchGlob_QuestionMark(t *testing.T) {
	if !globMatch("/files/report-?.pdf", "/files/report-1.pdf") {
		t.Error("? should match exactly one character")
	}
	if globMatch("/files/report-?.pdf", "/files/report-12.pdf") {
		t.Error("? must NOT match two characters")
	}
	if globMatch("/files/report-?.pdf", "/files/report-a1.pdf") {
		t.Error("? must NOT match a segment of a different length")
	}
}

func TestMatchGlob_NoWildcardEqualsExact(t *testing.T) {
	if !globMatch("/a/b/c", "/a/b/c") {
		t.Error("no-wildcard glob should match exactly")
	}
	if globMatch("/a/b/c", "/a/b") {
		t.Error("no-wildcard glob must not match a shorter path")
	}
	if globMatch("/a/b/c", "/a/b/c/d") {
		t.Error("no-wildcard glob must not match a longer path")
	}
}

func TestMatchGlob_RootAndEmpty(t *testing.T) {
	if !globMatch("/", "/") {
		t.Error("/ should match /")
	}
	if globMatch("/", "/users") {
		t.Error("/ must NOT match /users")
	}
}

func TestMatchGlob_TrailingSlash(t *testing.T) {
	// D5 — trailing-slash normalization applies to glob only.
	if !globMatch("/users/", "/users") {
		t.Error("/users/ should match /users (trailing slash normalized)")
	}
	if !globMatch("/users/", "/users/") {
		t.Error("/users/ should match /users/")
	}
}

func TestMatchGlob_EmptySegments(t *testing.T) {
	// Interior empty segments are preserved and significant.
	if !globMatch("/a//b", "/a//b") {
		t.Error("/a//b should match /a//b")
	}
	if globMatch("/a//b", "/a/b") {
		t.Error("/a//b must NOT match /a/b (interior empty segment is significant)")
	}
}

func TestMatchGlob_Determinism(t *testing.T) {
	// Repeated evaluation of the same pattern/path is stable.
	for i := 0; i < 100; i++ {
		if !globMatch("/users/**/orders", "/users/1/2/orders") {
			t.Fatalf("iteration %d: expected stable match", i)
		}
		if globMatch("/users/**/orders", "/users/1/2/other") {
			t.Fatalf("iteration %d: expected stable non-match", i)
		}
	}
}

// ---------------------------------------------------------------------------
// Glob wildcard weight (D1)
// ---------------------------------------------------------------------------

func TestGlobWildcardWeight(t *testing.T) {
	cases := []struct {
		pattern string
		want    int
	}{
		{"/users/*/orders", 1},
		{"/users/**", 2},
		{"/users/??", 0},
		{"/a/b/c", 0},
		{"/files/**/*", 3},
	}
	for _, tc := range cases {
		if got := globWildcardWeight(splitPathSegs(tc.pattern)); got != tc.want {
			t.Errorf("globWildcardWeight(%q) = %d, want %d", tc.pattern, got, tc.want)
		}
	}
}

func TestCompileEndpoint_InSegmentStar(t *testing.T) {
	// D2 — in-segment '*' fails loudly at load.
	err := compileEndpoint(&Endpoint{PathGlob: "/files/*.pdf"})
	if err == nil {
		t.Fatal("expected parse error for in-segment '*'")
	}
	if !strings.Contains(err.Error(), "path_glob") {
		t.Errorf("error should name path_glob, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Regex matching (MAT-002)
// ---------------------------------------------------------------------------

func TestMatchRegex_Anchored(t *testing.T) {
	ep := mustCompile(t, &Endpoint{PathRegex: `^/orders/[0-9]{5}$`})
	if !ep.matchPath("/orders/12345") {
		t.Error("^/orders/[0-9]{5}$ should match /orders/12345")
	}
	if ep.matchPath("/orders/1234") {
		t.Error("four digits must not match")
	}
	if ep.matchPath("/orders/123456") {
		t.Error("six digits must not match")
	}
	if ep.matchPath("/webhooks/orders/12345") {
		t.Error("prefix before the anchor must not match")
	}
}

func TestMatchRegex_UnanchoredNoImplicitAnchoring(t *testing.T) {
	// D6 — anchoring is the user's job; a bare pattern matches anywhere.
	ep := mustCompile(t, &Endpoint{PathRegex: `orders`})
	if !ep.matchPath("/webhooks/orders/extra") {
		t.Error("unanchored regex should match as a substring (no implicit anchoring)")
	}
	if ep.matchPath("/webhooks/orders") {
		// Still a match (substring); this branch guards the positive case above.
		t.Log("substring match confirmed")
	}
}

func TestCompileEndpoint_InvalidRegex(t *testing.T) {
	err := compileEndpoint(&Endpoint{PathRegex: `([unclosed`})
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
	if !strings.Contains(err.Error(), "invalid path_regex") {
		t.Errorf("error should name invalid path_regex, got %v", err)
	}
	if !strings.Contains(err.Error(), "([unclosed") {
		t.Errorf("error should name the offending regex, got %v", err)
	}
}

func TestCompileEndpoint_PathExclusivity(t *testing.T) {
	combos := []struct {
		name string
		ep   *Endpoint
	}{
		{"path+glob", &Endpoint{Path: "/a", PathGlob: "/a/*"}},
		{"path+regex", &Endpoint{Path: "/a", PathRegex: "^/a$"}},
		{"glob+regex", &Endpoint{PathGlob: "/a/*", PathRegex: "^/a/"}},
		{"all three", &Endpoint{Path: "/a", PathGlob: "/a/*", PathRegex: "^/a/"}},
	}
	for _, tc := range combos {
		t.Run(tc.name, func(t *testing.T) {
			err := compileEndpoint(tc.ep)
			if err == nil {
				t.Fatal("expected exclusivity error")
			}
			if !strings.Contains(err.Error(), "conflicting path matchers") {
				t.Errorf("error should name the conflicting matchers, got %v", err)
			}
			if !strings.Contains(err.Error(), "path_glob") && !strings.Contains(err.Error(), "path_regex") {
				t.Errorf("error should name the conflicting fields, got %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Registry safety net (MAT-005 AC6): Add/Update compile errors leave the
// registry unchanged.
// ---------------------------------------------------------------------------

func TestRegistry_AddInvalidRegex_RegistryUnchanged(t *testing.T) {
	r := NewRegistry()
	if err := r.Add(&Endpoint{ID: "good", Path: "/a"}); err != nil {
		t.Fatalf("seed add: %v", err)
	}
	err := r.Add(&Endpoint{ID: "bad", PathRegex: `([unclosed`})
	if err == nil {
		t.Fatal("expected error for invalid regex Add")
	}
	if !strings.Contains(err.Error(), "bad") {
		t.Errorf("error should name the endpoint, got %v", err)
	}
	if _, err := r.Get("bad"); err == nil {
		t.Fatal("invalid endpoint must NOT be stored")
	}
	if r.Match("GET", "/a", nil, nil, nil) == nil {
		t.Fatal("existing endpoint must still match after a failed Add")
	}
}

func TestRegistry_UpdateInvalidRegex_RegistryUnchanged(t *testing.T) {
	r := NewRegistry()
	if err := r.Add(&Endpoint{ID: "ep", Path: "/old"}); err != nil {
		t.Fatalf("seed add: %v", err)
	}
	err := r.Update("ep", &Endpoint{PathRegex: `([unclosed`})
	if err == nil {
		t.Fatal("expected error for invalid regex Update")
	}
	if !strings.Contains(err.Error(), "endpoint ep") {
		t.Errorf("Update error must name the endpoint ID, got %q", err.Error())
	}
	ep, err := r.Get("ep")
	if err != nil {
		t.Fatalf("endpoint must still exist after failed Update: %v", err)
	}
	if ep.Path != "/old" {
		t.Errorf("endpoint must keep its previous configuration, got path %q", ep.Path)
	}
}

// ---------------------------------------------------------------------------
// DisplayPath (MAT-005)
// ---------------------------------------------------------------------------

func TestDisplayPath_Precedence(t *testing.T) {
	cases := []struct {
		name string
		ep   *Endpoint
		want string
	}{
		{"exact path", &Endpoint{Path: "/users/me"}, "/users/me"},
		{"glob only", &Endpoint{PathGlob: "/users/**"}, "/users/**"},
		{"regex only", &Endpoint{PathRegex: `^/orders/[0-9]{5}$`}, `^/orders/[0-9]{5}$`},
		{"none", &Endpoint{}, ""},
		{"path beats glob", &Endpoint{Path: "/a", PathGlob: "/a/*"}, "/a"},
		{"glob beats regex", &Endpoint{PathGlob: "/a/*", PathRegex: "^/a/"}, "/a/*"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.ep.DisplayPath(); got != tc.want {
				t.Errorf("DisplayPath() = %q, want %q", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// matchPath dispatch through compiled endpoints (MAT-001/MAT-002)
// ---------------------------------------------------------------------------

func TestEndpointMatchPath_Dispatch(t *testing.T) {
	exact := mustCompile(t, &Endpoint{Path: "/users/me"})
	glob := mustCompile(t, &Endpoint{PathGlob: "/users/**"})
	re := mustCompile(t, &Endpoint{PathRegex: `^/users/[0-9]+$`})
	none := mustCompile(t, &Endpoint{}) // D4 — no matcher, never matches

	if !exact.matchPath("/users/me") || exact.matchPath("/users/123") {
		t.Error("exact matcher must match only the exact path")
	}
	if !glob.matchPath("/users/123") || !glob.matchPath("/users/123/orders") {
		t.Error("glob matcher should match multi-segment paths")
	}
	if !re.matchPath("/users/42") || re.matchPath("/users/ada") {
		t.Error("regex matcher should match the pattern only")
	}
	if none.matchPath("/anything") {
		t.Error("endpoint with no path matcher must never match (D4)")
	}
}
