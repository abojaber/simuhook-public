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
		s.writeResponse(w, resp)
		return
	}

	if ep.Response.Delay > 0 {
		time.Sleep(ep.Response.Delay)
	}

	s.writeResponse(w, ep.Response)
}

func (s *Server) writeResponse(w http.ResponseWriter, resp Response) {
	for k, v := range resp.Headers {
		w.Header().Set(k, v)
	}
	if resp.StatusCode == 0 {
		resp.StatusCode = http.StatusOK
	}
	w.WriteHeader(resp.StatusCode)
	if len(resp.Body) > 0 {
		w.Write(resp.Body)
	}
}

func (s *Server) MarshalEndpoints() ([]byte, error) {
	eps := s.registry.List()
	return json.Marshal(eps)
}
