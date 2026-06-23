package mock

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

func TestServer_HandlerMatch(t *testing.T) {
	r := NewRegistry()
	r.Add(&Endpoint{
		ID:       "hello",
		Method:   "GET",
		Path:     "/hello",
		Response: Response{StatusCode: 200, Body: []byte(`{"msg":"hi"}`), Headers: map[string]string{"X-Custom": "yes"}},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		headers := make(map[string]string)
		for k := range req.Header {
			headers[k] = req.Header.Get(k)
		}
		ep := r.Match(req.Method, req.URL.Path, headers, nil, body)
		if ep == nil {
			w.WriteHeader(404)
			return
		}
		for k, v := range ep.Response.Headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(ep.Response.StatusCode)
		w.Write(ep.Response.Body)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if resp.Header.Get("X-Custom") != "yes" {
		t.Fatalf("expected X-Custom header")
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"msg":"hi"}` {
		t.Fatalf("expected body, got %s", string(body))
	}
}

func TestServer_HandlerNoMatch(t *testing.T) {
	r := NewRegistry()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ep := r.Match(req.Method, req.URL.Path, nil, nil, nil)
		if ep == nil {
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"no matching endpoint"}`))
			return
		}
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
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

func TestServer_Delay(t *testing.T) {
	r := NewRegistry()
	r.Add(&Endpoint{
		ID:       "slow",
		Method:   "GET",
		Path:     "/slow",
		Response: Response{StatusCode: 200, Body: []byte(`done`), Delay: 100 * time.Millisecond},
	})

	start := time.Now()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ep := r.Match(req.Method, req.URL.Path, nil, nil, nil)
		time.Sleep(ep.Response.Delay)
		w.WriteHeader(ep.Response.StatusCode)
		w.Write(ep.Response.Body)
	}))
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/slow")
	resp.Body.Close()
	d := time.Since(start)
	if d < 100*time.Millisecond {
		t.Fatalf("expected at least 100ms delay, got %v", d)
	}
}

func TestServer_DefaultStatusCode(t *testing.T) {
	r := NewRegistry()
	r.Add(&Endpoint{ID: "def", Method: "GET", Path: "/def", Response: Response{Body: []byte(`ok`)}})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ep := r.Match(req.Method, req.URL.Path, nil, nil, nil)
		code := ep.Response.StatusCode
		if code == 0 {
			code = 200
		}
		w.WriteHeader(code)
		w.Write(ep.Response.Body)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/def")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 default, got %d", resp.StatusCode)
	}
}

func TestServer_MultipleEndpointsRequest(t *testing.T) {
	r := NewRegistry()
	r.Add(&Endpoint{ID: "create", Method: "POST", Path: "/users", Response: Response{StatusCode: 201, Body: []byte(`created`)}})
	r.Add(&Endpoint{ID: "list", Method: "GET", Path: "/users", Response: Response{StatusCode: 200, Body: []byte(`list`)}})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		headers := make(map[string]string)
		for k := range req.Header {
			headers[k] = req.Header.Get(k)
		}
		ep := r.Match(req.Method, req.URL.Path, headers, nil, body)
		w.WriteHeader(ep.Response.StatusCode)
		w.Write(ep.Response.Body)
	}))
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/users")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "list" {
		t.Fatalf("expected 'list', got %s", string(body))
	}

	resp2, _ := http.Post(srv.URL+"/users", "application/json", bytes.NewReader([]byte(`{}`)))
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if string(body2) != "created" {
		t.Fatalf("expected 'created', got %s", string(body2))
	}
}

func TestServer_EchoResponseBody(t *testing.T) {
	r := NewRegistry()
	r.Add(&Endpoint{
		ID:     "echo-ep",
		Method: "POST",
		Path:   "/echo",
		Response: Response{
			StatusCode: 200,
			Body:       []byte(`should be ignored`),
			Echo:       true,
		},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		headers := make(map[string]string)
		for k := range req.Header {
			headers[k] = req.Header.Get(k)
		}
		ep := r.Match(req.Method, req.URL.Path, headers, nil, body)
		w.WriteHeader(ep.Response.StatusCode)
		if ep.Response.Echo {
			w.Write(body)
		} else {
			w.Write(ep.Response.Body)
		}
	}))
	defer srv.Close()

	payload := `{"hello":"world"}`
	resp, err := http.Post(srv.URL+"/echo", "application/json", bytes.NewReader([]byte(payload)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if string(respBody) != payload {
		t.Fatalf("expected echoed body %q, got %q", payload, string(respBody))
	}
}

func TestServer_EchoDisabledUsesConfiguredBody(t *testing.T) {
	r := NewRegistry()
	r.Add(&Endpoint{
		ID:     "normal-ep",
		Method: "POST",
		Path:   "/normal",
		Response: Response{
			StatusCode: 200,
			Body:       []byte(`configured`),
			Echo:       false,
		},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		ep := r.Match(req.Method, req.URL.Path, nil, nil, body)
		if ep.Response.Echo {
			w.Write(body)
		} else {
			w.Write(ep.Response.Body)
		}
	}))
	defer srv.Close()

	resp, _ := http.Post(srv.URL+"/normal", "application/json", bytes.NewReader([]byte(`request body`)))
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if string(respBody) != "configured" {
		t.Fatalf("expected configured body, got %q", string(respBody))
	}
}
