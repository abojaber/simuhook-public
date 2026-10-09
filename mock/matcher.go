package mock

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ---------------------------------------------------------------------------
// Path matcher dispatch (MAT-001/MAT-002)
//
// pathClass orders the three path matcher kinds by specificity:
// exact 0 < glob 1 < regex 2 < none 3. Only one may be set per endpoint —
// compileEndpoint enforces that at load.
// ---------------------------------------------------------------------------

func pathClass(ep *Endpoint) int {
	switch {
	case ep.Path != "":
		return 0
	case ep.PathGlob != "":
		return 1
	case ep.PathRegex != "":
		return 2
	default:
		return 3
	}
}

// matchPath is the per-endpoint dispatch. Matching operates on the raw
// request path as received (no URL decoding — byte-identical to the legacy
// exact match). An endpoint with no path matcher configured never matches
// (D4 — no "match any path" change).
func (ep *Endpoint) matchPath(got string) bool {
	switch {
	case ep.Path != "":
		return ep.Path == got
	case ep.PathGlob != "":
		return matchGlobSegs(ep.pathGlobSegs, splitPathSegs(got))
	case ep.PathRegex != "":
		return ep.pathRe != nil && ep.pathRe.MatchString(got)
	default:
		return false
	}
}

// DisplayPath returns the effective path pattern for display:
// path → path_glob → path_regex (MAT-005). Used by the dashboard endpoints
// panel and the startup banner so pattern-based stubs are not blank.
func (ep *Endpoint) DisplayPath() string {
	switch {
	case ep.Path != "":
		return ep.Path
	case ep.PathGlob != "":
		return ep.PathGlob
	case ep.PathRegex != "":
		return ep.PathRegex
	default:
		return ""
	}
}

// ---------------------------------------------------------------------------
// compileEndpoint — compile-at-load validation (MAT-005)
//
// Enforces path exclusivity (at most one of path / path_glob / path_regex),
// payload exclusivity (at most one of payload / payload_contains), pre-splits
// glob segments and computes the wildcard weight (D1), and compiles the regex
// once. Called from ParseStubFile (YAML) and Registry.Add/Update (the
// programmatic safety net). Compiled fields are set before the endpoint is
// published under the registry write lock, so readers never observe a
// partially compiled endpoint.
// ---------------------------------------------------------------------------

func compileEndpoint(ep *Endpoint) error {
	if n := pathMatcherCount(ep); n > 1 {
		return fmt.Errorf("conflicting path matchers: %s (at most one of path/path_glob/path_regex)",
			pathMatcherFields(ep))
	}
	if len(ep.Payload) > 0 && ep.hasPayloadContains {
		return fmt.Errorf("conflicting payload matchers: payload, payload_contains (at most one of payload/payload_contains)")
	}

	ep.pathRe = nil
	ep.pathGlobSegs = nil
	ep.globWeight = 0

	if ep.PathGlob != "" {
		if err := compileGlobPath(ep); err != nil {
			return err
		}
	}
	if ep.PathRegex != "" {
		re, err := regexp.Compile(ep.PathRegex)
		if err != nil {
			return fmt.Errorf("invalid path_regex %q: %w", ep.PathRegex, err)
		}
		ep.pathRe = re
	}
	return nil
}

func pathMatcherCount(ep *Endpoint) int {
	n := 0
	for _, set := range []bool{ep.Path != "", ep.PathGlob != "", ep.PathRegex != ""} {
		if set {
			n++
		}
	}
	return n
}

func pathMatcherFields(ep *Endpoint) string {
	var fields []string
	if ep.Path != "" {
		fields = append(fields, "path")
	}
	if ep.PathGlob != "" {
		fields = append(fields, "path_glob")
	}
	if ep.PathRegex != "" {
		fields = append(fields, "path_regex")
	}
	return strings.Join(fields, ", ")
}

// compileGlobPath pre-splits a path_glob into segments and computes its
// wildcard weight. In-segment '*' is rejected (D2): only whole-segment '*' /
// '**' and in-segment '?' are supported.
func compileGlobPath(ep *Endpoint) error {
	segs := splitPathSegs(ep.PathGlob)
	for _, s := range segs {
		if s == "*" || s == "**" {
			continue
		}
		if strings.Contains(s, "*") {
			return fmt.Errorf("invalid path_glob %q: in-segment '*' is not supported (only whole-segment * and **)", ep.PathGlob)
		}
	}
	ep.pathGlobSegs = segs
	ep.globWeight = globWildcardWeight(segs)
	return nil
}

// globWildcardWeight computes D1 wildcard weight: '**'=2, '*'=1, '?'=0.
// Fewer wins, so /users/*/orders (weight 1) beats /users/** (weight 2).
func globWildcardWeight(segs []string) int {
	w := 0
	for _, s := range segs {
		switch s {
		case "**":
			w += 2
		case "*":
			w += 1
		}
	}
	return w
}

// ---------------------------------------------------------------------------
// Glob matching (MAT-001)
//
// splitPathSegs normalizes a path into segments: leading/trailing slashes are
// trimmed (D5 — trailing-slash normalization for glob only), the root "/" and
// the empty path both produce nil, and interior empty segments are preserved
// so "/a//b" differs from "/a/b".
// ---------------------------------------------------------------------------

func splitPathSegs(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

// matchGlobSegs recursively matches a glob pattern against a request path,
// both as segment slices. '**' matches zero or more segments, '*' matches
// exactly one segment, and any other segment is a rune-wise literal with '?'
// matching exactly one rune (matchGlobSegment).
func matchGlobSegs(pattern, got []string) bool {
	if len(pattern) == 0 {
		return len(got) == 0
	}
	if pattern[0] == "**" {
		// Try zero consumed segments, then one, two, ...
		for i := 0; i <= len(got); i++ {
			if matchGlobSegs(pattern[1:], got[i:]) {
				return true
			}
		}
		return false
	}
	if len(got) == 0 {
		return false
	}
	if !matchGlobSegment(pattern[0], got[0]) {
		return false
	}
	return matchGlobSegs(pattern[1:], got[1:])
}

// matchGlobSegment matches a single glob segment against a single request
// segment. A whole-segment '*' matches anything (the caller guarantees a
// segment exists); otherwise the match is rune-wise with '?' matching exactly
// one rune and literals matched exactly.
func matchGlobSegment(pattern, seg string) bool {
	if pattern == "*" {
		return true
	}
	pr, gr := []rune(pattern), []rune(seg)
	if len(pr) != len(gr) {
		return false
	}
	for i := range pr {
		if pr[i] != '?' && pr[i] != gr[i] {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// payload_contains (MAT-003)
//
// parsePayloadContains parses the YAML string into the exported
// PayloadContains value. Empty spec leaves the matcher unset (D3's
// hasPayloadContains flag disambiguates an explicit JSON "null" spec from an
// unset one).
// ---------------------------------------------------------------------------

func parsePayloadContains(ep *Endpoint, raw string) error {
	if raw == "" {
		ep.PayloadContains = nil
		ep.hasPayloadContains = false
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return fmt.Errorf("invalid payload_contains JSON: %w", err)
	}
	ep.PayloadContains = v
	ep.hasPayloadContains = true
	return nil
}
