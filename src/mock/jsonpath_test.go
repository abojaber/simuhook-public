package mock

import (
	"encoding/json"
	"strings"
	"testing"
)

// The story's AC fixture body (TPL-002):
//   {"user":{"name":"Ada","roles":["admin","dev"]},"amount":42,"ok":true,"tags":null}

func mustParse(t *testing.T, raw string) any {
	t.Helper()
	v, ok := parseJSONBody([]byte(raw))
	if !ok {
		t.Fatalf("fixture %q did not parse", raw)
	}
	return v
}

func render(t *testing.T, root any, path string) string {
	t.Helper()
	toks, err := tokenizeJSONPath(path)
	if err != nil {
		return ""
	}
	out, err := evalJSONPath(root, toks)
	if err != nil {
		return ""
	}
	return out
}

// --- tokenize grammar ---------------------------------------------------

func TestTokenizeJSONPath_Valid(t *testing.T) {
	cases := []struct {
		path string
		want []string
	}{
		{"$", []string{}},
		{"$.user.name", []string{"user", "name"}},
		{"$.a.b", []string{"a", "b"}},
		{"$.a[0]", []string{"a", "0"}},
		{"$[0]", []string{"0"}},
		{"$.a[0].b", []string{"a", "0", "b"}},
		{"$.items[1].tags[0]", []string{"items", "1", "tags", "0"}},
		{"$.0", []string{"0"}},          // numeric KEY via dot form
		{"$._a-b", []string{"_a-b"}},    // underscore + hyphen allowed
		{"$[12]", []string{"12"}},       // multi-digit index
		{"$[0][1]", []string{"0", "1"}}, // nested root-array index
	}
	for _, c := range cases {
		got, err := tokenizeJSONPath(c.path)
		if err != nil {
			t.Errorf("tokenizeJSONPath(%q) unexpected error: %v", c.path, err)
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("tokenizeJSONPath(%q) = %v, want %v", c.path, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("tokenizeJSONPath(%q) = %v, want %v", c.path, got, c.want)
				break
			}
		}
	}
}

func TestTokenizeJSONPath_Invalid(t *testing.T) {
	bad := []string{
		"",            // empty path
		"a",           // no $ prefix
		"user.name",   // no $ prefix
		".user",       // leading dot
		"$user",       // after $ only . or [ allowed
		"$.",          // trailing/empty key
		"$.a[]",       // empty brackets
		"$.a[-1]",     // negative index
		"$.a[1",       // missing ]
		"$.a[x]",      // non-digit index
		"$.a b",       // space (D15, zero trimming)
		"$.a b.c",     // space anywhere
		"$..a",        // recursive descent (out of scope)
		"$.a[*]",      // wildcard (out of scope)
		"$.a[?(@.b)]", // filter (out of scope)
		"$.a.",        // trailing dot
		"$.[]",        // dot then empty bracket
		"$ .a",        // space after $
	}
	for _, p := range bad {
		if _, err := tokenizeJSONPath(p); err == nil {
			t.Errorf("tokenizeJSONPath(%q) expected error, got nil", p)
		}
	}
}

// --- eval semantics -----------------------------------------------------

func TestEvalJSONPath_NestedKey(t *testing.T) {
	root := mustParse(t, `{"user":{"name":"Ada","roles":["admin","dev"]},"amount":42,"ok":true,"tags":null}`)
	if got := render(t, root, "$.user.name"); got != "Ada" {
		t.Errorf(`$.user.name = %q, want "Ada"`, got)
	}
}

func TestEvalJSONPath_ArrayIndex(t *testing.T) {
	root := mustParse(t, `{"user":{"name":"Ada","roles":["admin","dev"]},"amount":42,"ok":true,"tags":null}`)
	if got := render(t, root, "$.user.roles[1]"); got != "dev" {
		t.Errorf(`$.user.roles[1] = %q, want "dev"`, got)
	}
}

func TestEvalJSONPath_NumberUnquoted(t *testing.T) {
	root := mustParse(t, `{"user":{"name":"Ada","roles":["admin","dev"]},"amount":42,"ok":true,"tags":null}`)
	if got := render(t, root, "$.amount"); got != "42" {
		t.Errorf(`$.amount = %q, want "42"`, got)
	}
}

func TestEvalJSONPath_BoolUnquoted(t *testing.T) {
	root := mustParse(t, `{"user":{"name":"Ada","roles":["admin","dev"]},"amount":42,"ok":true,"tags":null}`)
	if got := render(t, root, "$.ok"); got != "true" {
		t.Errorf(`$.ok = %q, want "true"`, got)
	}
}

func TestEvalJSONPath_NullValue(t *testing.T) {
	root := mustParse(t, `{"user":{"name":"Ada","roles":["admin","dev"]},"amount":42,"ok":true,"tags":null}`)
	if got := render(t, root, "$.tags"); got != "null" {
		t.Errorf(`$.tags = %q, want "null"`, got)
	}
}

func TestEvalJSONPath_ObjectArrayMarshal(t *testing.T) {
	root := mustParse(t, `{"user":{"name":"Ada","roles":["admin","dev"]},"amount":42,"ok":true,"tags":null}`)
	got := render(t, root, "$.user.roles")
	want := `["admin","dev"]`
	if got != want {
		t.Errorf(`$.user.roles = %q, want %q`, got, want)
	}
	if !json.Valid([]byte(got)) {
		t.Errorf(`$.user.roles output %q is not valid JSON`, got)
	}

	// object → canonical re-marshal (D13): sorted keys, compacted
	gotObj := render(t, root, "$.user")
	if !json.Valid([]byte(gotObj)) {
		t.Errorf(`$.user output %q is not valid JSON`, gotObj)
	}
	if !strings.Contains(gotObj, `"name":"Ada"`) {
		t.Errorf(`$.user = %q, missing name key`, gotObj)
	}
}

func TestEvalJSONPath_Root(t *testing.T) {
	root := mustParse(t, `{"user":{"name":"Ada","roles":["admin","dev"]},"amount":42,"ok":true,"tags":null}`)
	got := render(t, root, "$")
	if !json.Valid([]byte(got)) {
		t.Errorf(`$ output %q is not valid JSON`, got)
	}

	// scalar roots
	if got := render(t, mustParse(t, `42`), "$"); got != "42" {
		t.Errorf(`$ on scalar 42 = %q, want "42"`, got)
	}
	if got := render(t, mustParse(t, `"hello"`), "$"); got != "hello" {
		t.Errorf(`$ on string root = %q, want "hello"`, got)
	}
}

func TestEvalJSONPath_MissingAndOOB(t *testing.T) {
	root := mustParse(t, `{"user":{"name":"Ada","roles":["admin","dev"]},"amount":42,"ok":true,"tags":null}`)
	cases := []string{
		"$.user.missing",    // missing key
		"$.user.roles[5]",   // out of bounds
		"$.user.roles[-1]",  // negative (tokenize error anyway → "")
		"$.name.deep",       // scalar reached before path exhausted
		"$.roles.name",      // string-as-index (on scalar/string)
		"$.user.name.extra", // path past a string
		"$.tags.deep",       // traversal INTO null → ""
	}
	for _, p := range cases {
		if got := render(t, root, p); got != "" {
			t.Errorf(`%s = %q, want ""`, p, got)
		}
	}
}

func TestEvalJSONPath_DeepNesting(t *testing.T) {
	root := mustParse(t, `{"items":[{"name":"a","tags":["x","y"]},{"name":"b","tags":["z"]}]}`)
	if got := render(t, root, "$.items[1].tags[0]"); got != "z" {
		t.Errorf(`$.items[1].tags[0] = %q, want "z"`, got)
	}
}

func TestEvalJSONPath_FloatPrecision(t *testing.T) {
	if got := render(t, mustParse(t, `42.5`), "$"); got != "42.5" {
		t.Errorf(`42.5 = %q, want "42.5"`, got)
	}
}

func TestEvalJSONPath_LargeNumberExponent(t *testing.T) {
	// D14: FormatFloat(v, 'g', -1, 64) — 1e21 → "1e+21"
	root := mustParse(t, `{"n":1e21}`)
	if got := render(t, root, "$.n"); got != "1e+21" {
		t.Errorf(`$.n (1e21) = %q, want "1e+21"`, got)
	}
}

func TestEvalJSONPath_NonJSONBody(t *testing.T) {
	// D12 is enforced at the CLOSURE level (makeJSONPathFunc: parsedBody ==
	// nil → "" even for $). The raw eval func on a nil root follows plain
	// JSON semantics: $ renders "null"; any path into nil → "".
	var nilRoot any = nil
	if got := render(t, nilRoot, "$"); got != "null" {
		t.Errorf(`$ on nil root = %q, want "null" (raw eval JSON semantics)`, got)
	}
	if got := render(t, nilRoot, "$.x"); got != "" {
		t.Errorf(`$.x on nil root = %q, want ""`, got)
	}

	// A body that PARSES to JSON null DOES render "null" (valid JSON value)
	nullRoot := mustParse(t, `null`)
	if got := render(t, nullRoot, "$"); got != "null" {
		t.Errorf(`$ on JSON null = %q, want "null"`, got)
	}
}

// --- the closure (template-facing) -------------------------------------

func TestJSONPathClosure_ReadsCachedParsedBody(t *testing.T) {
	data := BuildTemplateData("req-1", "POST", "/hook", []byte(`{"user":{"name":"Ada"}}`))
	if data.parsedBody == nil {
		t.Fatal("expected parsedBody non-nil for valid JSON body")
	}
	fn := makeJSONPathFunc(data)
	if got := fn("$.user.name"); got != "Ada" {
		t.Errorf(`closure $.user.name = %q, want "Ada"`, got)
	}
	// second call hits the same cached parse
	if got := fn("$.user.name"); got != "Ada" {
		t.Errorf(`closure second call = %q, want "Ada"`, got)
	}
}

func TestJSONPathClosure_InvalidPathEmpty(t *testing.T) {
	data := BuildTemplateData("req-1", "POST", "/hook", []byte(`{"a":1}`))
	fn := makeJSONPathFunc(data)
	for _, p := range []string{"user", "$..a", "$.a[", "$.a b"} {
		if got := fn(p); got != "" {
			t.Errorf(`closure(%q) = %q, want ""`, p, got)
		}
	}
}

func TestJSONPathClosure_NilParsedBody(t *testing.T) {
	data := &TemplateData{} // parsedBody nil explicitly
	fn := makeJSONPathFunc(data)
	if got := fn("$"); got != "" {
		t.Errorf(`closure($) with nil parsedBody = %q, want ""`, got)
	}
	if got := fn("$.x"); got != "" {
		t.Errorf(`closure($.x) with nil parsedBody = %q, want ""`, got)
	}
}

// --- root array direct index -------------------------------------------

func TestEvalJSONPath_RootArrayIndex(t *testing.T) {
	root := mustParse(t, `["a","b","c"]`)
	if got := render(t, root, "$[1]"); got != "b" {
		t.Errorf(`$[1] = %q, want "b"`, got)
	}
	if got := render(t, root, "$[9]"); got != "" {
		t.Errorf(`$[9] = %q, want "" (OOB)`, got)
	}
}
