package mock

import (
	"testing"
)

func TestRegistry_AddAndGet(t *testing.T) {
	r := NewRegistry()
	ep := &Endpoint{ID: "ep1", Method: "GET", Path: "/hello"}
	if err := r.Add(ep); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := r.Get("ep1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Path != "/hello" {
		t.Fatalf("expected /hello, got %s", got.Path)
	}
}

func TestRegistry_AddDuplicate(t *testing.T) {
	r := NewRegistry()
	r.Add(&Endpoint{ID: "ep1"})
	err := r.Add(&Endpoint{ID: "ep1"})
	if err == nil {
		t.Fatal("expected error for duplicate")
	}
}

func TestRegistry_Update(t *testing.T) {
	r := NewRegistry()
	r.Add(&Endpoint{ID: "ep1", Path: "/old"})
	r.Update("ep1", &Endpoint{Path: "/new"})
	ep, _ := r.Get("ep1")
	if ep.Path != "/new" {
		t.Fatalf("expected /new, got %s", ep.Path)
	}
}

func TestRegistry_Remove(t *testing.T) {
	r := NewRegistry()
	r.Add(&Endpoint{ID: "ep1"})
	r.Remove("ep1")
	_, err := r.Get("ep1")
	if err == nil {
		t.Fatal("expected error after remove")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	r.Add(&Endpoint{ID: "ep1"})
	r.Add(&Endpoint{ID: "ep2"})
	eps := r.List()
	if len(eps) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(eps))
	}
}

func TestRegistry_MatchByMethod(t *testing.T) {
	r := NewRegistry()
	r.Add(&Endpoint{ID: "ep1", Method: "POST", Path: "/data", Response: Response{StatusCode: 201}})

	ep := r.Match("POST", "/data", nil, nil, nil)
	if ep == nil || ep.ID != "ep1" {
		t.Fatal("expected match")
	}

	ep = r.Match("GET", "/data", nil, nil, nil)
	if ep != nil {
		t.Fatal("expected no match for wrong method")
	}
}

func TestRegistry_MatchByHeaders(t *testing.T) {
	r := NewRegistry()
	r.Add(&Endpoint{ID: "ep1", Method: "POST", Path: "/webhook",
		Headers:  map[string]string{"X-Event": "user.created"},
		Response: Response{StatusCode: 200},
	})

	ep := r.Match("POST", "/webhook", map[string]string{"X-Event": "user.created"}, nil, nil)
	if ep == nil || ep.ID != "ep1" {
		t.Fatal("expected match with correct header")
	}

	ep = r.Match("POST", "/webhook", map[string]string{"X-Event": "order.placed"}, nil, nil)
	if ep != nil {
		t.Fatal("expected no match for wrong header value")
	}
}

func TestRegistry_MatchByPayload(t *testing.T) {
	r := NewRegistry()
	r.Add(&Endpoint{ID: "ep1", Method: "POST", Path: "/hook",
		Payload:  []byte(`{"event":"user.created"}`),
		Response: Response{StatusCode: 200},
	})

	ep := r.Match("POST", "/hook", nil, nil, []byte(`{"event":"user.created"}`))
	if ep == nil || ep.ID != "ep1" {
		t.Fatal("expected match with correct payload")
	}

	ep = r.Match("POST", "/hook", nil, nil, []byte(`{"event":"order.placed"}`))
	if ep != nil {
		t.Fatal("expected no match for wrong payload")
	}
}

func TestRegistry_MatchExactPath(t *testing.T) {
	r := NewRegistry()
	r.Add(&Endpoint{ID: "ep1", Path: "/api/v1/users", Response: Response{StatusCode: 200}})

	ep := r.Match("GET", "/api/v1/users", nil, nil, nil)
	if ep == nil {
		t.Fatal("expected match for exact path")
	}

	ep = r.Match("GET", "/api/v1/users/123", nil, nil, nil)
	if ep != nil {
		t.Fatal("expected no match for different path")
	}
}

func TestRegistry_MatchEmptyMethod(t *testing.T) {
	r := NewRegistry()
	r.Add(&Endpoint{ID: "wild", Method: "", Path: "/catch", Response: Response{StatusCode: 200}})

	for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
		ep := r.Match(method, "/catch", nil, nil, nil)
		if ep == nil {
			t.Fatalf("expected match for %s when method is empty", method)
		}
	}
}

func TestRegistry_MatchEmptyPayloadAndHeaders(t *testing.T) {
	r := NewRegistry()
	r.Add(&Endpoint{ID: "ep1", Method: "POST", Path: "/hook", Response: Response{StatusCode: 200}})

	ep := r.Match("POST", "/hook",
		map[string]string{"X-Any": "value"},
		nil,
		[]byte(`{"anything":"goes"}`),
	)
	if ep == nil || ep.ID != "ep1" {
		t.Fatal("expected match when endpoint has no header/payload constraints")
	}
}

func TestRegistry_MatchByQuery(t *testing.T) {
	r := NewRegistry()
	r.Add(&Endpoint{ID: "ep1", Method: "GET", Path: "/ping",
		Query:    map[string]string{"ping_id": "11022022", "ping_child": "43443"},
		Response: Response{StatusCode: 200},
	})

	ep := r.Match("GET", "/ping", nil, map[string]string{"ping_id": "11022022", "ping_child": "43443"}, nil)
	if ep == nil || ep.ID != "ep1" {
		t.Fatal("expected match with correct query params")
	}

	ep = r.Match("GET", "/ping", nil, map[string]string{"ping_id": "11022022"}, nil)
	if ep != nil {
		t.Fatal("expected no match with missing query param")
	}

	ep = r.Match("GET", "/ping", nil, map[string]string{"ping_id": "wrong", "ping_child": "43443"}, nil)
	if ep != nil {
		t.Fatal("expected no match with wrong query param value")
	}
}

func TestExecuteBodyTemplate_RendersFields(t *testing.T) {
	data := &TemplateData{
		UUID:      "abc-def",
		Now:       "2026-01-01T00:00:00Z",
		Timestamp: 1765843200,
		RequestID: "req-123",
		Method:    "POST",
		Path:      "/test",
		Body:      `{"hello":"world"}`,
	}

	result := ExecuteBodyTemplate(`{"id":"{{.UUID}}","now":"{{.Now}}","ts":{{.Timestamp}},"method":"{{.Method}}"}`,
		data, nil, nil)

	expected := `{"id":"abc-def","now":"2026-01-01T00:00:00Z","ts":1765843200,"method":"POST"}`
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestExecuteBodyTemplate_NoTemplateMarkers(t *testing.T) {
	result := ExecuteBodyTemplate(`{"static":"value"}`, &TemplateData{UUID: "x"}, nil, nil)
	if result != `{"static":"value"}` {
		t.Fatalf("expected unchanged body, got %q", result)
	}
}

func TestExecuteBodyTemplate_WithHeaderFunc(t *testing.T) {
	data := &TemplateData{UUID: "abc"}
	headers := map[string]string{"X-Custom": "myvalue"}
	result := ExecuteBodyTemplate(`{"h":"{{Header "X-Custom"}}"}`, data, headers, nil)
	if result != `{"h":"myvalue"}` {
		t.Fatalf("expected header value in body, got %q", result)
	}
}

func TestExecuteBodyTemplate_RandomInt(t *testing.T) {
	data := &TemplateData{}
	for i := 0; i < 50; i++ {
		result := ExecuteBodyTemplate(`{{RandomInt 10 20}}`, data, nil, nil)
		if result < "10" || result > "19" {
			t.Fatalf("expected value between 10 and 19, got %s", result)
		}
	}
}

func TestExecuteBodyTemplate_RandomHex(t *testing.T) {
	data := &TemplateData{}
	result := ExecuteBodyTemplate(`{{RandomHex 16}}`, data, nil, nil)
	if len(result) != 16 {
		t.Fatalf("expected 16-char hex, got %q (len=%d)", result, len(result))
	}
	for _, c := range result {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Fatalf("non-hex character %c in %q", c, result)
		}
	}
}

func TestExecuteHeaderTemplate_RendersField(t *testing.T) {
	data := &TemplateData{RequestID: "req-999"}
	result := ExecuteHeaderTemplate("{{.RequestID}}", data, nil, nil)
	if result != "req-999" {
		t.Fatalf("expected req-999, got %q", result)
	}
}

func TestExecuteHeaderTemplate_NoMarkers(t *testing.T) {
	result := ExecuteHeaderTemplate("static-value", &TemplateData{RequestID: "x"}, nil, nil)
	if result != "static-value" {
		t.Fatalf("expected unchanged, got %q", result)
	}
}

func TestPickResponse_FallsBackToSingular(t *testing.T) {
	ep := &Endpoint{
		Response: Response{StatusCode: 200, Body: []byte(`singular`)},
	}
	for i := 0; i < 10; i++ {
		r, idx := PickResponse(ep)
		if r.StatusCode != 200 || string(r.Body) != "singular" {
			t.Fatalf("expected singular response")
		}
		if idx != -1 {
			t.Fatalf("expected variant index -1 for singular response, got %d", idx)
		}
	}
}

func TestPickResponse_RoundRobin(t *testing.T) {
	ep := &Endpoint{
		Responses: []Response{
			{StatusCode: 200, Body: []byte(`first`)},
			{StatusCode: 201, Body: []byte(`second`)},
			{StatusCode: 202, Body: []byte(`third`)},
		},
	}

	expected := []int{200, 201, 202, 200, 201, 202}
	expectedIdx := []int{0, 1, 2, 0, 1, 2}
	for i, want := range expected {
		r, idx := PickResponse(ep)
		if r.StatusCode != want {
			t.Fatalf("round-robin step %d: expected %d, got %d", i, want, r.StatusCode)
		}
		if idx != expectedIdx[i] {
			t.Fatalf("round-robin step %d: expected variant index %d, got %d", i, expectedIdx[i], idx)
		}
	}
}

func TestPickResponse_WeightedRandom(t *testing.T) {
	ep := &Endpoint{
		Responses: []Response{
			{StatusCode: 200, Weight: 90},
			{StatusCode: 500, Weight: 10},
		},
	}

	counts := map[int]int{}
	for i := 0; i < 1000; i++ {
		r, idx := PickResponse(ep)
		counts[r.StatusCode]++
		if idx < 0 || idx > 1 {
			t.Fatalf("expected variant index 0 or 1, got %d", idx)
		}
	}

	if counts[200] < 800 {
		t.Fatalf("expected ~900 success, got %d", counts[200])
	}
	if counts[500] < 1 {
		t.Fatal("expected at least one 500")
	}
}

func TestPickResponse_EqualWeight(t *testing.T) {
	ep := &Endpoint{
		Responses: []Response{
			{StatusCode: 200, Weight: 1},
			{StatusCode: 404, Weight: 1},
		},
	}

	counts := map[int]int{}
	for i := 0; i < 1000; i++ {
		r, _ := PickResponse(ep)
		counts[r.StatusCode]++
	}

	if counts[200] < 300 || counts[404] < 300 {
		t.Fatalf("expected both statuses to appear, got %v", counts)
	}
}

func TestPickResponse_NoWeightsUsesRoundRobin(t *testing.T) {
	ep := &Endpoint{
		Responses: []Response{
			{StatusCode: 200, Body: []byte(`a`)},
			{StatusCode: 200, Body: []byte(`b`)},
		},
	}

	bodies := make([]string, 4)
	indices := make([]int, 4)
	for i := 0; i < 4; i++ {
		r, idx := PickResponse(ep)
		bodies[i] = string(r.Body)
		indices[i] = idx
	}

	expected := []string{"a", "b", "a", "b"}
	expectedIdx := []int{0, 1, 0, 1}
	for i, b := range bodies {
		if b != expected[i] {
			t.Fatalf("expected %q at step %d, got %q", expected[i], i, b)
		}
		if indices[i] != expectedIdx[i] {
			t.Fatalf("expected variant index %d at step %d, got %d", expectedIdx[i], i, indices[i])
		}
	}
}
