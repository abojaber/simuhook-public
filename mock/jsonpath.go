package mock

// ---------------------------------------------------------------------------
// JSONPath extraction (TPL-002)
//
// A minimal, strict JSONPath subset for template functions ({{JSONPath "$..."}}):
//   - $                     → the whole parsed body
//   - $.key                 → object key lookup (keys: [A-Za-z0-9_-]+)
//   - $.key.sub             → nested lookup
//   - $[0]                  → array index (non-negative integers only)
//   - $.items[0].name       → mixed nesting
//
// Deliberately rejected at tokenize time (out of scope, story §Notes):
// filters ($.x[?(@.a)]), wildcards ($.x[*]), recursive descent ($..x),
// negative indices, whitespace in keys/paths. Errors and missing paths
// render as "" (never a panic, never a template fallback).
//
// Parsing the body happens exactly once per request in BuildTemplateData
// (TemplateData.parsedBody) — the template func only traverses. Numbers
// arrive as float64 (json.Unmarshal, no UseNumber), rendered with
// FormatFloat('g', -1, 64) — 42 → "42", 42.5 → "42.5", 1e21 → "1e+21" (D14).
// Objects/arrays render via canonical re-marshal (D13): sorted map keys,
// compacted whitespace — never byte-exact to the request.
// ---------------------------------------------------------------------------

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// tokenizeJSONPath splits a template-facing path like "$.user.name[0]" into
// its traversal tokens ["user","name","0"]. Strict grammar — any deviation
// (empty path, missing $, leading dot, empty key, empty/non-digit index,
// space, filters/wildcards/recursive descent) is an error.
func tokenizeJSONPath(path string) ([]string, error) {
	if path == "" {
		return nil, fmt.Errorf("jsonpath: empty path")
	}
	if path[0] != '$' {
		return nil, fmt.Errorf("jsonpath: path must start with '$': %q", path)
	}
	var toks []string
	rest := path[1:]
	for len(rest) > 0 {
		switch rest[0] {
		case '.':
			rest = rest[1:]
			if rest == "" || rest[0] == '.' || rest[0] == '[' {
				return nil, fmt.Errorf("jsonpath: empty key in %q", path)
			}
			i := 0
			for i < len(rest) && isKeyChar(rest[i]) {
				i++
			}
			if i == 0 {
				return nil, fmt.Errorf("jsonpath: bad key in %q", path)
			}
			toks = append(toks, rest[:i])
			rest = rest[i:]
		case '[':
			rest = rest[1:]
			i := 0
			for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
				i++
			}
			if i == 0 || i >= len(rest) || rest[i] != ']' {
				return nil, fmt.Errorf("jsonpath: bad index in %q", path)
			}
			toks = append(toks, rest[:i])
			rest = rest[i+1:]
		default:
			return nil, fmt.Errorf("jsonpath: unexpected %q in %q", rest[0], path)
		}
	}
	return toks, nil
}

// isKeyChar reports whether b is valid inside a dotted key: [A-Za-z0-9_-].
// No spaces, no dots, no unicode (D15 strict grammar).
func isKeyChar(b byte) bool {
	return b >= 'a' && b <= 'z' ||
		b >= 'A' && b <= 'Z' ||
		b >= '0' && b <= '9' ||
		b == '_' || b == '-'
}

// evalJSONPath walks root along tokens. An index token is disambiguated from
// a key by the container type: map[string]any → key lookup; []any → integer
// index. Returns the rendered value, or "" for missing keys, out-of-bounds
// indices, type mismatches, and traversal into null (JSONPath null
// semantics). Never panics.
func evalJSONPath(root any, tokens []string) (string, error) {
	cur := root
	for _, tok := range tokens {
		switch tv := cur.(type) {
		case map[string]any:
			v, ok := tv[tok]
			if !ok {
				return "", nil
			}
			cur = v
		case []any:
			n, err := strconv.Atoi(tok)
			if err != nil || n < 0 || n >= len(tv) {
				return "", nil
			}
			cur = tv[n]
		case nil:
			return "", nil
		default:
			return "", nil
		}
	}
	return renderJSONValue(cur), nil
}

// renderJSONValue renders a parsed JSON value to template text:
//   - string             → raw, unquoted
//   - float64            → FormatFloat('g', -1, 64) (D14)
//   - bool               → "true" / "false"
//   - nil                → "null"
//   - map[string]any / []any → canonical json.Marshal (D13)
func renderJSONValue(v any) string {
	switch tv := v.(type) {
	case string:
		return tv
	case float64:
		return strconv.FormatFloat(tv, 'g', -1, 64)
	case bool:
		if tv {
			return "true"
		}
		return "false"
	case nil:
		return "null"
	default:
		b, err := json.Marshal(tv)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

// makeJSONPathFunc returns the template function bound to this request's
// parsed body. Never panics: tokenize/eval errors and a nil body all yield
// "" (story ACs: non-JSON body, missing path, malformed path → "").
func makeJSONPathFunc(data *TemplateData) func(string) string {
	return func(path string) string {
		if data.parsedBody == nil {
			return ""
		}
		toks, err := tokenizeJSONPath(path)
		if err != nil {
			return ""
		}
		out, err := evalJSONPath(data.parsedBody, toks)
		if err != nil {
			return ""
		}
		return out
	}
}
