package mock

import (
	"encoding/json"
	"io"
	"net/http"
	"time"
)

type Server struct {
	registry *Registry
	server   *http.Server
	addr     string
}

func NewServer(addr string, registry *Registry) *Server {
	return &Server{
		registry: registry,
		addr:     addr,
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handler)
	s.server = &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}
	return s.server.ListenAndServe()
}

func (s *Server) Stop() error {
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

func (s *Server) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()

	headers := make(map[string]string)
	for k := range r.Header {
		headers[k] = r.Header.Get(k)
	}
	query := make(map[string]string)
	for k := range r.URL.Query() {
		query[k] = r.URL.Query().Get(k)
	}

	ep := s.registry.Match(r.Method, r.URL.Path, headers, query, body)
	if ep == nil {
		resp := Response{
			StatusCode: http.StatusNotFound,
			Body:       []byte(`{"error":"no matching endpoint"}`),
		}
		s.writeResponse(w, resp, resp.Body, nil)
		return
	}

	if ep.Response.Delay > 0 {
		time.Sleep(ep.Response.Delay)
	}

	// Build template data if needed
	requestID := NewRequestID()
	var tmplData *TemplateData
	if ep.Response.Template {
		tmplData = BuildTemplateData(requestID, r.Method, r.URL.Path, body)
	}

	// Determine response body: echo > template > configured
	respBody := ep.Response.Body
	if ep.Response.Echo {
		respBody = body
	} else if ep.Response.Template && len(ep.Response.Body) > 0 {
		rendered := ExecuteBodyTemplate(string(ep.Response.Body), tmplData, headers, query)
		respBody = []byte(rendered)
	}

	s.writeResponse(w, ep.Response, respBody, tmplData)
}

func (s *Server) writeResponse(w http.ResponseWriter, resp Response, bodyOverride []byte, tmplData *TemplateData) {
	// Set response headers (with optional template execution)
	for k, v := range resp.Headers {
		if resp.Template && tmplData != nil {
			v = ExecuteHeaderTemplate(v, tmplData, nil, nil)
		}
		w.Header().Set(k, v)
	}
	if resp.StatusCode == 0 {
		resp.StatusCode = http.StatusOK
	}
	w.WriteHeader(resp.StatusCode)
	if len(bodyOverride) > 0 {
		w.Write(bodyOverride)
	}
}

func (s *Server) MarshalEndpoints() ([]byte, error) {
	eps := s.registry.List()
	return json.Marshal(eps)
}
