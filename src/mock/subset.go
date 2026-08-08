package mock

import "encoding/json"

// ---------------------------------------------------------------------------
// payload_contains subset matching (MAT-003)
//
// A stub with payload_contains matches a request body when the body is valid
// JSON and the spec is a *subset* of it:
//   - spec object  → every spec key must be present and subset-match the
//     request value; extra request keys are ignored.
//   - spec array   → each spec element must subset-match a DISTINCT request
//     element (order-independent, backtracking — not greedy).
//   - scalars      → deep equality with numeric cross-type equality
//     (JSON numbers all parse to float64, so 42 == 42.0).
//   - null spec    → matches only a literal null body (D3).
//   - anything else (mismatched types) → false.
//
// Out of scope: JSONPath expressions and regex on body text.
// ---------------------------------------------------------------------------

// parseJSONBody parses a raw request body as JSON into Go any values
// (map[string]any, []any, float64, string, bool, nil). Empty or invalid JSON
// returns ok=false — a non-JSON request body never matches a
// payload_contains stub.
func parseJSONBody(raw []byte) (any, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, false
	}
	return v, true
}

// subsetMatch reports whether spec is a subset of req under the MAT-003
// semantics described above.
func subsetMatch(spec, req any) bool {
	switch s := spec.(type) {
	case map[string]any:
		r, ok := req.(map[string]any)
		if !ok {
			return false
		}
		for k, sv := range s {
			rv, ok := r[k]
			if !ok || !subsetMatch(sv, rv) {
				return false
			}
		}
		return true
	case []any:
		r, ok := req.([]any)
		if !ok {
			return false
		}
		return subsetMatchArray(s, r)
	case float64:
		rf, ok := numToFloat64(req)
		return ok && s == rf
	case string:
		r, ok := req.(string)
		return ok && s == r
	case bool:
		r, ok := req.(bool)
		return ok && s == r
	case nil:
		return req == nil
	default:
		return false
	}
}

// numToFloat64 accepts any JSON numeric representation (float64 in practice,
// plus int/int64 defensively) so numeric equality is cross-type.
func numToFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

// subsetMatchArray reports whether every spec element subset-matches a
// distinct request element, order-independent.
func subsetMatchArray(spec, req []any) bool {
	used := make([]bool, len(req))
	return subsetArrayRec(spec, req, used)
}

// subsetArrayRec backtracks over request elements so the greedy first
// assignment cannot block a later spec element. Example: spec
// [{"a":1},{"b":2}] vs req [{"a":1,"b":2},{"a":1}] — assigning spec[0] to
// req[0] would strand spec[1], so the algorithm unmarks and tries req[1].
func subsetArrayRec(spec, req []any, used []bool) bool {
	if len(spec) == 0 {
		return true
	}
	for i, rv := range req {
		if used[i] {
			continue
		}
		if subsetMatch(spec[0], rv) {
			used[i] = true
			if subsetArrayRec(spec[1:], req, used) {
				return true
			}
			used[i] = false
		}
	}
	return false
}
