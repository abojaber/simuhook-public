package mock

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestServer_MarshalEndpoints verifies MarshalEndpoints returns valid JSON
// representing all registered endpoints.
func TestServer_MarshalEndpoints(t *testing.T) {
	r := NewRegistry()
	r.Add(&Endpoint{ID: "ep1", Method: "GET", Path: "/one", Response: Response{StatusCode: 200}})
	r.Add(&Endpoint{ID: "ep2", Method: "POST", Path: "/two", Response: Response{StatusCode: 201}})

	s := &Server{registry: r}

	data, err := s.MarshalEndpoints()
	if err != nil {
		t.Fatalf("MarshalEndpoints returned error: %v", err)
	}

	var decoded []Endpoint
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("MarshalEndpoints output is not valid JSON: %v\n%s", err, data)
	}
	if len(decoded) != 2 {
		t.Fatalf("expected 2 endpoints in JSON, got %d", len(decoded))
	}

	ids := map[string]bool{}
	for _, ep := range decoded {
		ids[ep.ID] = true
	}
	if !ids["ep1"] || !ids["ep2"] {
		t.Fatalf("expected ep1 and ep2 in JSON, got %v", ids)
	}
}

// TestServer_WriteResponse_NilHeaders verifies writeResponse does not panic
// when the response headers map is nil.
func TestServer_WriteResponse_NilHeaders(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()

	resp := Response{
		StatusCode: 204,
		Body:       []byte(`no-content`),
		Headers:    nil,
	}

	s.writeResponse(rec, resp, resp.Body, nil)

	if rec.Code != 204 {
		t.Fatalf("expected status 204, got %d", rec.Code)
	}
	if got := rec.Body.String(); got != "no-content" {
		t.Fatalf("expected body %q, got %q", "no-content", got)
	}
}

// TestServer_Handler_ZeroWeightVariant verifies the handler does not crash
// when a responses[] variant has zero weight — it falls back to round-robin.
func TestServer_Handler_ZeroWeightVariant(t *testing.T) {
	r := NewRegistry()
	r.Add(&Endpoint{
		ID:     "zero-weight",
		Method: "GET",
		Path:   "/zero",
		Responses: []Response{
			{StatusCode: 200, Body: []byte(`ok-a`), Weight: 0},
			{StatusCode: 500, Body: []byte(`err-b`), Weight: 0},
		},
	})

	s := &Server{registry: r}

	statuses := map[int]int{}
	for i := 0; i < 4; i++ {
		req := httptest.NewRequest(http.MethodGet, "/zero", nil)
		rec := httptest.NewRecorder()
		s.handler(rec, req)
		statuses[rec.Code]++
	}

	// Zero-weight variants mean hasWeights() is false → round-robin alternates.
	if statuses[200] != 2 || statuses[500] != 2 {
		t.Fatalf("expected round-robin 2x200 / 2x500, got %v", statuses)
	}
}

// TestServer_Handler_ZeroWeightVariant_BodyRoundRobin verifies bodies also
// alternate under round-robin when all weights are zero.
func TestServer_Handler_ZeroWeightVariant_BodyRoundRobin(t *testing.T) {
	r := NewRegistry()
	r.Add(&Endpoint{
		ID:     "zero-weight-bodies",
		Method: "GET",
		Path:   "/zero-bodies",
		Responses: []Response{
			{StatusCode: 200, Body: []byte(`a`), Weight: 0},
			{StatusCode: 200, Body: []byte(`b`), Weight: 0},
		},
	})

	s := &Server{registry: r}

	var bodies []string
	for i := 0; i < 4; i++ {
		req := httptest.NewRequest(http.MethodGet, "/zero-bodies", nil)
		rec := httptest.NewRecorder()
		s.handler(rec, req)
		b, _ := io.ReadAll(rec.Result().Body)
		bodies = append(bodies, string(b))
	}

	expected := []string{"a", "b", "a", "b"}
	for i, got := range bodies {
		if got != expected[i] {
			t.Fatalf("expected body %q at step %d, got %q", expected[i], i, got)
		}
	}
}

// TestServer_Handler_MarshalIncludesCreatedAt verifies CreatedAt is serialized
// so downstream consumers can parse endpoint metadata.
func TestServer_MarshalEndpoints_IncludesCreatedAt(t *testing.T) {
	r := NewRegistry()
	ep := &Endpoint{ID: "with-ts", Method: "GET", Path: "/ts"}
	r.Add(ep)

	s := &Server{registry: r}
	data, err := s.MarshalEndpoints()
	if err != nil {
		t.Fatalf("MarshalEndpoints returned error: %v", err)
	}
	if !bytes.Contains(data, []byte(`"created_at"`)) {
		t.Fatalf("expected created_at field in JSON, got %s", data)
	}
}
