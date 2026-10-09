package mock

import (
	"encoding/json"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// specFromJSON parses a payload_contains spec exactly as parsePayloadContains
// would (json.Unmarshal into any).
func specFromJSON(t *testing.T, raw string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("unmarshal spec %q: %v", raw, err)
	}
	return v
}

func reqFromJSON(t *testing.T, raw string) any {
	t.Helper()
	v, ok := parseJSONBody([]byte(raw))
	if !ok {
		t.Fatalf("request body %q did not parse as JSON", raw)
	}
	return v
}

// ---------------------------------------------------------------------------
// Pure subset matching (MAT-003)
// ---------------------------------------------------------------------------

func TestSubsetMatch_ExtraKeysIgnored(t *testing.T) {
	spec := specFromJSON(t, `{"event":"user.created"}`)
	req := reqFromJSON(t, `{"event":"user.created","user_id":42,"extra":true}`)
	if !subsetMatch(spec, req) {
		t.Error("extra request keys must be ignored")
	}
	req = reqFromJSON(t, `{"event":"order.placed"}`)
	if subsetMatch(spec, req) {
		t.Error("different required value must not match")
	}
	req = reqFromJSON(t, `{"user_id":42}`)
	if subsetMatch(spec, req) {
		t.Error("missing required key must not match")
	}
}

func TestSubsetMatch_NestedObject(t *testing.T) {
	spec := specFromJSON(t, `{"user":{"id":42}}`)
	req := reqFromJSON(t, `{"user":{"id":42,"name":"Ada","roles":["admin"]}}`)
	if !subsetMatch(spec, req) {
		t.Error("nested subset — extra nested keys ignored")
	}
	req = reqFromJSON(t, `{"user":{"id":43}}`)
	if subsetMatch(spec, req) {
		t.Error("wrong nested value must not match")
	}
	req = reqFromJSON(t, `{"user":{"name":"Ada"}}`)
	if subsetMatch(spec, req) {
		t.Error("missing nested key must not match")
	}
}

func TestSubsetMatch_ArrayOrderIndependent(t *testing.T) {
	spec := specFromJSON(t, `{"items":[1,2]}`)
	req := reqFromJSON(t, `{"items":[2,1,3]}`)
	if !subsetMatch(spec, req) {
		t.Error("array subset must be order-independent")
	}
	req = reqFromJSON(t, `{"items":[1]}`)
	if subsetMatch(spec, req) {
		t.Error("spec element 2 absent — must not match")
	}
	req = reqFromJSON(t, `{"items":[1,1]}`)
	if subsetMatch(spec, req) {
		t.Error("each spec element needs a distinct request element — [1,1] lacks a 2")
	}
}

func TestSubsetMatch_ArrayBacktracking(t *testing.T) {
	// Greedy assignment of spec[0] to req[0] would strand spec[1]; the
	// matcher must backtrack (MAT-003 / test plan).
	spec := specFromJSON(t, `[{"a":1},{"b":2}]`)
	req := reqFromJSON(t, `[{"a":1,"b":2},{"a":1}]`)
	if !subsetMatch(spec, req) {
		t.Error("array subset must backtrack: spec [{\"a\":1},{\"b\":2}] vs [{\"a\":1,\"b\":2},{\"a\":1}]")
	}

	// Negative: only one element can satisfy both specs.
	spec = specFromJSON(t, `[{"a":1},{"a":2}]`)
	req = reqFromJSON(t, `[{"a":1,"a2":true}]`)
	if subsetMatch(spec, req) {
		t.Error("cannot reuse one request element for two distinct spec elements")
	}
}

func TestSubsetMatch_NumericCrossType(t *testing.T) {
	spec := specFromJSON(t, `{"amount":42}`)
	req := reqFromJSON(t, `{"amount":42.0}`)
	if !subsetMatch(spec, req) {
		t.Error("42 must equal 42.0 (JSON numbers compare via float64)")
	}
	req = reqFromJSON(t, `{"amount":42.5}`)
	if subsetMatch(spec, req) {
		t.Error("42.5 must not equal 42")
	}
}

func TestSubsetMatch_TopLevelTypes(t *testing.T) {
	// Object spec vs non-object request.
	if subsetMatch(specFromJSON(t, `{"a":1}`), specFromJSON(t, `[{"a":1}]`)) {
		t.Error("object spec must not match an array request")
	}
	// Array spec vs non-array request.
	if subsetMatch(specFromJSON(t, `[1]`), specFromJSON(t, `{"a":1}`)) {
		t.Error("array spec must not match an object request")
	}
	// Scalar specs.
	if !subsetMatch(specFromJSON(t, `42`), specFromJSON(t, `42.0`)) {
		t.Error("scalar numeric subset should match across representations")
	}
	if subsetMatch(specFromJSON(t, `42`), specFromJSON(t, `"42"`)) {
		t.Error("number spec must not match a string request")
	}
	if !subsetMatch(specFromJSON(t, `"hi"`), specFromJSON(t, `"hi"`)) {
		t.Error("string spec should match equal string request")
	}
	if !subsetMatch(specFromJSON(t, `true`), specFromJSON(t, `true`)) {
		t.Error("bool spec should match equal bool request")
	}
	// D3 — null spec matches only a literal null body.
	if !subsetMatch(specFromJSON(t, `null`), specFromJSON(t, `null`)) {
		t.Error("null spec should match a null request")
	}
	if subsetMatch(specFromJSON(t, `null`), specFromJSON(t, `0`)) {
		t.Error("null spec must not match a non-null request")
	}
}

func TestParseJSONBody_NonJSONAndEmpty(t *testing.T) {
	if _, ok := parseJSONBody([]byte("not-json")); ok {
		t.Error("non-JSON body must not parse")
	}
	if _, ok := parseJSONBody(nil); ok {
		t.Error("empty body must not parse")
	}
	if _, ok := parseJSONBody([]byte("")); ok {
		t.Error("empty string body must not parse")
	}
	if _, ok := parseJSONBody([]byte(`{"valid":true}`)); !ok {
		t.Error("valid JSON must parse")
	}
}

// ---------------------------------------------------------------------------
// Registry.Match integration for payload_contains (MAT-003)
// ---------------------------------------------------------------------------

func TestRegistry_Match_PayloadContains(t *testing.T) {
	r := NewRegistry()
	if err := r.Add(&Endpoint{
		ID:                 "filter",
		Method:             "POST",
		Path:               "/orders",
		PayloadContains:    specFromJSON(t, `{"event":"order.placed","amount":42}`),
		hasPayloadContains: true,
		Response:           Response{StatusCode: 200},
	}); err != nil {
		t.Fatalf("add: %v", err)
	}

	if ep := r.Match("POST", "/orders", nil, nil, []byte(`{"event":"order.placed","amount":42,"extra":true}`)); ep == nil || ep.ID != "filter" {
		t.Error("matching body (with extra fields) should match")
	}
	if ep := r.Match("POST", "/orders", nil, nil, []byte(`{"event":"order.placed"}`)); ep != nil {
		t.Error("missing required field should not match")
	}
	if ep := r.Match("POST", "/orders", nil, nil, []byte(`not-json`)); ep != nil {
		t.Error("non-JSON body must never match a payload_contains stub")
	}
	if ep := r.Match("POST", "/orders", nil, nil, nil); ep != nil {
		t.Error("nil body must never match a payload_contains stub")
	}
	if ep := r.Match("GET", "/orders", nil, nil, []byte(`{"event":"order.placed","amount":42}`)); ep != nil {
		t.Error("wrong method must not match")
	}
}

// ---------------------------------------------------------------------------
// Load-time validation (MAT-005)
// ---------------------------------------------------------------------------

func TestCompileEndpoint_PayloadExclusivity(t *testing.T) {
	err := compileEndpoint(&Endpoint{
		Payload:            []byte(`{"a":1}`),
		PayloadContains:    specFromJSON(t, `{"a":1}`),
		hasPayloadContains: true,
	})
	if err == nil {
		t.Fatal("expected error for payload + payload_contains")
	}
	if !strings.Contains(err.Error(), "conflicting payload matchers") {
		t.Errorf("error should name the conflict, got %v", err)
	}
	if !strings.Contains(err.Error(), "payload") || !strings.Contains(err.Error(), "payload_contains") {
		t.Errorf("error should name both fields, got %v", err)
	}
}

func TestParsePayloadContains_InvalidJSON(t *testing.T) {
	ep := &Endpoint{}
	err := parsePayloadContains(ep, "{oops")
	if err == nil {
		t.Fatal("expected error for invalid payload_contains JSON")
	}
	if !strings.Contains(err.Error(), "payload_contains") {
		t.Errorf("error should name payload_contains, got %v", err)
	}
	if ep.hasPayloadContains {
		t.Error("invalid spec must not set the matcher")
	}
}

func TestParsePayloadContains_EmptyUnsets(t *testing.T) {
	ep := &Endpoint{}
	if err := parsePayloadContains(ep, ""); err != nil {
		t.Fatalf("empty spec should not error: %v", err)
	}
	if ep.hasPayloadContains {
		t.Error("empty spec must leave the matcher unset")
	}
}

func TestParsePayloadContains_NullSpec(t *testing.T) {
	// D3 — an explicit JSON "null" spec is distinct from "unset".
	ep := &Endpoint{}
	if err := parsePayloadContains(ep, "null"); err != nil {
		t.Fatalf("null spec should parse: %v", err)
	}
	if !ep.hasPayloadContains {
		t.Error("explicit null spec must set the matcher")
	}
}
