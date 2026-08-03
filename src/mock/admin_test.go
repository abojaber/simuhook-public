package mock

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newAdminFixture creates a handler and registry with the given endpoints
// pre-seeded.
func newAdminFixture(t *testing.T, eps ...*Endpoint) (http.Handler, *Registry) {
	t.Helper()
	r := NewRegistry()
	for _, ep := range eps {
		if err := r.Add(ep); err != nil {
			t.Fatalf("seed endpoint %s: %v", ep.ID, err)
		}
	}
	return NewAdminHandler(r), r
}

// doAdmin issues an HTTP request against the admin handler and returns the
// recorder. body may be empty ("").
func doAdmin(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rdr := strings.NewReader(body)
	req := httptest.NewRequest(method, path, rdr)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// decodeEndpoint decodes a single Endpoint from the response recorder body.
// NOTE: Payload and Response.Body are []byte, which encode as base64 in JSON.
// Use "body":"b2s=" for body "ok".
func decodeEndpoint(t *testing.T, rec *httptest.ResponseRecorder) Endpoint {
	t.Helper()
	var ep Endpoint
	if err := json.Unmarshal(rec.Body.Bytes(), &ep); err != nil {
		t.Fatalf("decode endpoint: %v (body=%q)", err, rec.Body.String())
	}
	return ep
}

// ---------------------------------------------------------------------------
// Test 1: Nil registry panics
// ---------------------------------------------------------------------------

func TestNewAdminHandler_NilRegistryPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic, got none")
		}
	}()
	NewAdminHandler(nil)
}

// ---------------------------------------------------------------------------
// Test 2: GET list empty
// ---------------------------------------------------------------------------

func TestAdminHandler_GetList_Empty(t *testing.T) {
	h, _ := newAdminFixture(t)
	rec := doAdmin(t, h, http.MethodGet, AdminEndpointsPath, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}
	// F3+F4: empty list encodes as "[]\n" (never null)
	if got := rec.Body.String(); got != "[]\n" {
		t.Fatalf("expected body %q, got %q", "[]\\n", got)
	}
}

// ---------------------------------------------------------------------------
// Test 3: GET list populated
// ---------------------------------------------------------------------------

func TestAdminHandler_GetList_Populated(t *testing.T) {
	a := &Endpoint{ID: "a", Method: "GET", Path: "/a"}
	b := &Endpoint{ID: "b", Method: "POST", Path: "/b", Response: Response{StatusCode: 201}}
	h, _ := newAdminFixture(t, a, b)

	rec := doAdmin(t, h, http.MethodGet, AdminEndpointsPath, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}

	var eps []Endpoint
	if err := json.Unmarshal(rec.Body.Bytes(), &eps); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(eps) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(eps))
	}

	ids := map[string]bool{}
	for _, ep := range eps {
		ids[ep.ID] = true
	}
	if !ids["a"] || !ids["b"] {
		t.Fatalf("expected ids {a,b}, got %v", ids)
	}
}

// ---------------------------------------------------------------------------
// Test 4: POST created
// ---------------------------------------------------------------------------

func TestAdminHandler_Post_Created(t *testing.T) {
	h, r := newAdminFixture(t)

	body := `{"id":"new","method":"POST","path":"/new","response":{"status_code":201,"body":"b2s="}}`
	rec := doAdmin(t, h, http.MethodPost, AdminEndpointsPath, body)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	// Q1: POST 201 sets no Content-Type (recorder reports "")
	if ct := rec.Header().Get("Content-Type"); ct != "" {
		t.Fatalf("expected Content-Type \"\" (no sniff in recorder), got %q", ct)
	}
	// F3: json.Encoder appends "\n"
	rb := rec.Body.Bytes()
	if len(rb) == 0 || rb[len(rb)-1] != '\n' {
		t.Fatalf("expected body ending with newline, got %q", rec.Body.String())
	}

	ep := decodeEndpoint(t, rec)
	if ep.ID != "new" {
		t.Fatalf("expected id=new, got %q", ep.ID)
	}
	if ep.Method != "POST" {
		t.Fatalf("expected method=POST, got %q", ep.Method)
	}
	if ep.Path != "/new" {
		t.Fatalf("expected path=/new, got %q", ep.Path)
	}
	if ep.Response.StatusCode != 201 {
		t.Fatalf("expected response.status_code=201, got %d", ep.Response.StatusCode)
	}
	if string(ep.Response.Body) != "ok" {
		t.Fatalf("expected response.body=ok, got %q", string(ep.Response.Body))
	}

	// Registry contains it
	got, err := r.Get("new")
	if err != nil {
		t.Fatalf("Get(new): %v", err)
	}
	if got.ID != "new" {
		t.Fatalf("registry id mismatch: %q", got.ID)
	}
	if len(r.List()) != 1 {
		t.Fatalf("expected 1 endpoint in registry, got %d", len(r.List()))
	}
}

// ---------------------------------------------------------------------------
// Test 5: POST duplicate → 409
// ---------------------------------------------------------------------------

func TestAdminHandler_Post_Duplicate409(t *testing.T) {
	dup := &Endpoint{ID: "dup", Method: "GET", Path: "/orig"}
	h, r := newAdminFixture(t, dup)

	body := `{"id":"dup","path":"/new"}`
	rec := doAdmin(t, h, http.MethodPost, AdminEndpointsPath, body)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
	// F2: http.Error → text/plain + nosniff
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Fatalf("expected Content-Type text/plain; charset=utf-8, got %q", ct)
	}
	if nosniff := rec.Header().Get("X-Content-Type-Options"); nosniff != "nosniff" {
		t.Fatalf("expected X-Content-Type-Options nosniff, got %q", nosniff)
	}
	expected := "endpoint dup already exists\n"
	if rec.Body.String() != expected {
		t.Fatalf("expected body %q, got %q", expected, rec.Body.String())
	}

	// Registry unchanged
	got, err := r.Get("dup")
	if err != nil {
		t.Fatalf("Get(dup): %v", err)
	}
	if got.Path != "/orig" {
		t.Fatalf("expected Path=/orig (unchanged), got %q", got.Path)
	}
}

// ---------------------------------------------------------------------------
// Test 6: POST invalid JSON → 400
// ---------------------------------------------------------------------------

func TestAdminHandler_Post_InvalidJSON400(t *testing.T) {
	h, r := newAdminFixture(t)

	tests := []struct {
		name, body, expectedBody string
	}{
		{"truncated", "{", "unexpected EOF\n"},
		{"empty", "", "EOF\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := doAdmin(t, h, http.MethodPost, AdminEndpointsPath, tt.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
				t.Fatalf("expected Content-Type text/plain; charset=utf-8, got %q", ct)
			}
			if nosniff := rec.Header().Get("X-Content-Type-Options"); nosniff != "nosniff" {
				t.Fatalf("expected X-Content-Type-Options nosniff, got %q", nosniff)
			}
			if rec.Body.String() != tt.expectedBody {
				t.Fatalf("expected body %q, got %q", tt.expectedBody, rec.Body.String())
			}
		})
	}

	// wrongType: {"id":1} → prefix "json: cannot unmarshal", suffix "\n"
	t.Run("wrongType", func(t *testing.T) {
		rec := doAdmin(t, h, http.MethodPost, AdminEndpointsPath, `{"id":1}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
			t.Fatalf("expected Content-Type text/plain; charset=utf-8, got %q", ct)
		}
		body := rec.Body.String()
		if !strings.HasPrefix(body, "json: cannot unmarshal") {
			t.Fatalf("expected prefix 'json: cannot unmarshal', got %q", body)
		}
		if !strings.HasSuffix(body, "\n") {
			t.Fatalf("expected suffix '\\n', got %q", body)
		}
	})

	// Registry unchanged
	if len(r.List()) != 0 {
		t.Fatalf("expected empty registry, got %d endpoints", len(r.List()))
	}
}

// ---------------------------------------------------------------------------
// Test 7: GET one → 200
// ---------------------------------------------------------------------------

func TestAdminHandler_GetOne_200(t *testing.T) {
	a := &Endpoint{ID: "a", Method: "GET", Path: "/a"}
	h, _ := newAdminFixture(t, a)

	rec := doAdmin(t, h, http.MethodGet, AdminEndpointsPath+"/a", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}
	rb := rec.Body.Bytes()
	if len(rb) == 0 || rb[len(rb)-1] != '\n' {
		t.Fatalf("expected body ending with newline, got %q", rec.Body.String())
	}
	ep := decodeEndpoint(t, rec)
	if ep.ID != "a" {
		t.Fatalf("expected id=a, got %q", ep.ID)
	}
}

// ---------------------------------------------------------------------------
// Test 8: GET unknown → 404
// ---------------------------------------------------------------------------

func TestAdminHandler_GetOne_Unknown404(t *testing.T) {
	h, _ := newAdminFixture(t)

	rec := doAdmin(t, h, http.MethodGet, AdminEndpointsPath+"/nope", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	expected := "endpoint nope not found\n"
	if rec.Body.String() != expected {
		t.Fatalf("expected body %q, got %q", expected, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Fatalf("expected Content-Type text/plain; charset=utf-8, got %q", ct)
	}
}

// ---------------------------------------------------------------------------
// Test 9: PUT update → 200
// ---------------------------------------------------------------------------

func TestAdminHandler_Put_Update200(t *testing.T) {
	a := &Endpoint{ID: "a", Method: "GET", Path: "/a"}
	h, r := newAdminFixture(t, a)

	body := `{"id":"IGNORED","path":"/a2","response":{"status_code":204}}`
	rec := doAdmin(t, h, http.MethodPut, AdminEndpointsPath+"/a", body)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}

	ep := decodeEndpoint(t, rec)
	// Registry.Update sets ep.ID = id — response reports the path id, not the body id
	if ep.ID != "a" {
		t.Fatalf("expected id=a (path id wins), got %q", ep.ID)
	}
	if ep.Path != "/a2" {
		t.Fatalf("expected path=/a2, got %q", ep.Path)
	}

	// Registry has updated value
	got, err := r.Get("a")
	if err != nil {
		t.Fatalf("Get(a): %v", err)
	}
	if got.Path != "/a2" {
		t.Fatalf("expected registry Path=/a2, got %q", got.Path)
	}

	// IGNORED must not exist
	if _, err := r.Get("IGNORED"); err == nil {
		t.Fatal("expected Get(IGNORED) to fail")
	}
	if len(r.List()) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(r.List()))
	}
}

// ---------------------------------------------------------------------------
// Test 10: PUT invalid JSON → 400 (decode before existence check)
// ---------------------------------------------------------------------------

func TestAdminHandler_Put_InvalidJSON400(t *testing.T) {
	a := &Endpoint{ID: "a", Method: "GET", Path: "/a"}
	h, r := newAdminFixture(t, a)

	subtests := []struct {
		name, id, body string
	}{
		{"existingID", "a", "{"},
		{"unknownID", "nope", "{"},
	}
	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			rec := doAdmin(t, h, http.MethodPut, AdminEndpointsPath+"/"+st.id, st.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", rec.Code)
			}
			expected := "unexpected EOF\n"
			if rec.Body.String() != expected {
				t.Fatalf("expected body %q, got %q", expected, rec.Body.String())
			}
		})
	}

	// Seeded endpoint unchanged
	got, err := r.Get("a")
	if err != nil {
		t.Fatalf("Get(a): %v", err)
	}
	if got.Path != "/a" {
		t.Fatalf("expected Path=/a unchanged, got %q", got.Path)
	}
}

// ---------------------------------------------------------------------------
// Test 11: PUT unknown id → 404
// ---------------------------------------------------------------------------

func TestAdminHandler_Put_Unknown404(t *testing.T) {
	a := &Endpoint{ID: "a", Method: "GET", Path: "/a"}
	h, r := newAdminFixture(t, a)

	rec := doAdmin(t, h, http.MethodPut, AdminEndpointsPath+"/nope", `{"path":"/x"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	expected := "endpoint nope not found\n"
	if rec.Body.String() != expected {
		t.Fatalf("expected body %q, got %q", expected, rec.Body.String())
	}

	if len(r.List()) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(r.List()))
	}
	if _, err := r.Get("nope"); err == nil {
		t.Fatal("expected Get(nope) to fail")
	}
}

// ---------------------------------------------------------------------------
// Test 12: DELETE → 204
// ---------------------------------------------------------------------------

func TestAdminHandler_Delete204(t *testing.T) {
	a := &Endpoint{ID: "a", Method: "GET", Path: "/a"}
	h, r := newAdminFixture(t, a)

	rec := doAdmin(t, h, http.MethodDelete, AdminEndpointsPath+"/a", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}
	// F5: DELETE 204 sets no Content-Type (recorder reports "")
	if ct := rec.Header().Get("Content-Type"); ct != "" {
		t.Fatalf("expected Content-Type \"\", got %q", ct)
	}

	if _, err := r.Get("a"); err == nil {
		t.Fatal("expected Get(a) to fail after delete")
	}
	if len(r.List()) != 0 {
		t.Fatalf("expected 0 endpoints, got %d", len(r.List()))
	}
}

// ---------------------------------------------------------------------------
// Test 13: DELETE unknown → 404
// ---------------------------------------------------------------------------

func TestAdminHandler_Delete_Unknown404(t *testing.T) {
	h, _ := newAdminFixture(t)

	rec := doAdmin(t, h, http.MethodDelete, AdminEndpointsPath+"/nope", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	expected := "endpoint nope not found\n"
	if rec.Body.String() != expected {
		t.Fatalf("expected body %q, got %q", expected, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Test 14: Missing ID → 400 (id check precedes method switch)
// ---------------------------------------------------------------------------

func TestAdminHandler_MissingID400(t *testing.T) {
	h, _ := newAdminFixture(t)

	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			rec := doAdmin(t, h, method, AdminEndpointsPath+"/", "")
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("%s: expected 400, got %d", method, rec.Code)
			}
			expected := "missing endpoint id\n"
			if rec.Body.String() != expected {
				t.Fatalf("%s: expected body %q, got %q", method, expected, rec.Body.String())
			}
			if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
				t.Fatalf("%s: expected Content-Type text/plain; charset=utf-8, got %q", method, ct)
			}
			if nosniff := rec.Header().Get("X-Content-Type-Options"); nosniff != "nosniff" {
				t.Fatalf("%s: expected X-Content-Type-Options nosniff, got %q", method, nosniff)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test 15: Method not allowed → 405
// ---------------------------------------------------------------------------

func TestAdminHandler_MethodNotAllowed405(t *testing.T) {
	a := &Endpoint{ID: "a", Method: "GET", Path: "/a"}
	h, _ := newAdminFixture(t, a)

	// Collection: PUT, DELETE, PATCH, HEAD → 405
	collectionRoutes := []struct{ method string }{
		{http.MethodPut},
		{http.MethodDelete},
		{http.MethodPatch},
		{http.MethodHead},
	}
	for _, st := range collectionRoutes {
		t.Run("collection "+st.method, func(t *testing.T) {
			rec := doAdmin(t, h, st.method, AdminEndpointsPath, "")
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("expected 405, got %d", rec.Code)
			}
			expected := "method not allowed\n"
			if rec.Body.String() != expected {
				t.Fatalf("expected body %q, got %q", expected, rec.Body.String())
			}
			if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
				t.Fatalf("expected Content-Type text/plain; charset=utf-8, got %q", ct)
			}
			if nosniff := rec.Header().Get("X-Content-Type-Options"); nosniff != "nosniff" {
				t.Fatalf("expected X-Content-Type-Options nosniff, got %q", nosniff)
			}
		})
	}

	// Item: POST, PATCH on /a → 405
	itemRoutes := []struct{ method string }{
		{http.MethodPost},
		{http.MethodPatch},
	}
	for _, st := range itemRoutes {
		t.Run("item "+st.method, func(t *testing.T) {
			rec := doAdmin(t, h, st.method, AdminEndpointsPath+"/a", "")
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("expected 405, got %d", rec.Code)
			}
			expected := "method not allowed\n"
			if rec.Body.String() != expected {
				t.Fatalf("expected body %q, got %q", expected, rec.Body.String())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test 16: Nested ID (subtree semantics)
// ---------------------------------------------------------------------------

func TestAdminHandler_NestedID(t *testing.T) {
	h, r := newAdminFixture(t)

	// a/b does not exist → 404
	rec := doAdmin(t, h, http.MethodGet, AdminEndpointsPath+"/a/b", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	expected := "endpoint a/b not found\n"
	if rec.Body.String() != expected {
		t.Fatalf("expected body %q, got %q", expected, rec.Body.String())
	}

	// Seed with id "a/b"
	ab := &Endpoint{ID: "a/b", Method: "POST", Path: "/a/b"}
	if err := r.Add(ab); err != nil {
		t.Fatalf("Add(a/b): %v", err)
	}

	rec2 := doAdmin(t, h, http.MethodGet, AdminEndpointsPath+"/a/b", "")
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec2.Code)
	}
	ep := decodeEndpoint(t, rec2)
	if ep.ID != "a/b" {
		t.Fatalf("expected id=a/b, got %q", ep.ID)
	}
}

// ---------------------------------------------------------------------------
// Test 17: Mux precedence — admin requests are NOT logged
// ---------------------------------------------------------------------------

func TestAdminHandler_MuxPrecedence_AdminNotLogged(t *testing.T) {
	r := NewRegistry()

	// Seed a hook endpoint for the mock surface
	hook := &Endpoint{ID: "hook", Method: "GET", Path: "/hook", Response: Response{StatusCode: 200, Body: []byte(`ok`)}}
	if err := r.Add(hook); err != nil {
		t.Fatalf("Add(hook): %v", err)
	}

	// Recording observer: count how many LogEntries are produced
	var mu sync.Mutex
	var observerCount int
	rec := func(e LogEntry) {
		mu.Lock()
		defer mu.Unlock()
		observerCount++
	}

	// Build mux as main.go does
	admin := NewAdminHandler(r)
	mux := http.NewServeMux()
	mux.Handle(AdminEndpointsPath, admin)     // collection
	mux.Handle(AdminEndpointsPath+"/", admin) // item: /{id}
	mux.Handle("/", NewHandler(HandlerConfig{
		Registry:  r,
		Observers: []func(LogEntry){rec},
	}))

	// Issue admin requests via mux
	doMux := func(method, path, body string) *httptest.ResponseRecorder {
		rdr := strings.NewReader(body)
		req := httptest.NewRequest(method, path, rdr)
		rec2 := httptest.NewRecorder()
		mux.ServeHTTP(rec2, req)
		return rec2
	}

	// GET /_admin/endpoints
	rec1 := doMux(http.MethodGet, AdminEndpointsPath, "")
	if rec1.Code != http.StatusOK {
		t.Fatalf("GET /_admin/endpoints: expected 200, got %d", rec1.Code)
	}

	// GET /_admin/endpoints/a (registers a fixture inline)
	a := &Endpoint{ID: "a", Method: "POST", Path: "/a"}
	if err := r.Add(a); err != nil {
		t.Fatalf("Add(a): %v", err)
	}
	rec2 := doMux(http.MethodGet, AdminEndpointsPath+"/a", "")
	if rec2.Code != http.StatusOK {
		t.Fatalf("GET /_admin/endpoints/a: expected 200, got %d", rec2.Code)
	}

	// POST /_admin/endpoints
	rec3 := doMux(http.MethodPost, AdminEndpointsPath, `{"id":"b","method":"DELETE","path":"/b"}`)
	if rec3.Code != http.StatusCreated {
		t.Fatalf("POST /_admin/endpoints: expected 201, got %d", rec3.Code)
	}

	// GET /hook (non-admin, should fire observer)
	rec4 := doMux(http.MethodGet, "/hook", "")
	if rec4.Code != http.StatusOK {
		t.Fatalf("GET /hook: expected 200, got %d", rec4.Code)
	}

	// Assert: admin requests produced ZERO observer entries; /hook produced exactly one
	mu.Lock()
	count := observerCount
	mu.Unlock()
	if count != 1 {
		t.Fatalf("expected 1 observer entry (from /hook only), got %d", count)
	}
}
