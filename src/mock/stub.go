package mock

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type stubFile struct {
	ID        string            `yaml:"id"`
	Method    string            `yaml:"method"`
	Path      string            `yaml:"path"`
	Headers   map[string]string `yaml:"headers,omitempty"`
	Query     map[string]string `yaml:"query,omitempty"`
	Payload   string            `yaml:"payload,omitempty"`
	Response  stubResponse      `yaml:"response"`
	Responses []stubResponse    `yaml:"responses,omitempty"`
}

type stubResponse struct {
	StatusCode int               `yaml:"status_code"`
	Body       string            `yaml:"body,omitempty"`
	Headers    map[string]string `yaml:"headers,omitempty"`
	Delay      string            `yaml:"delay,omitempty"`
	Echo       bool              `yaml:"echo,omitempty"`
	Template   bool              `yaml:"template,omitempty"`
	Weight     int               `yaml:"weight,omitempty"`
}

func parseStubResponse(s stubResponse) (Response, error) {
	r := Response{
		StatusCode: s.StatusCode,
		Body:       []byte(s.Body),
		Headers:    s.Headers,
		Echo:       s.Echo,
		Template:   s.Template,
		Weight:     s.Weight,
	}
	if s.Delay != "" {
		d, err := time.ParseDuration(s.Delay)
		if err != nil {
			return r, fmt.Errorf("invalid delay %q: %w", s.Delay, err)
		}
		r.Delay = d
	}
	return r, nil
}

func ParseStubFile(path string) (*Endpoint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read stub file %s: %w", path, err)
	}

	var s stubFile
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse stub file %s: %w", path, err)
	}

	if s.ID == "" {
		return nil, fmt.Errorf("stub file %s missing id", path)
	}

	singleResp, err := parseStubResponse(s.Response)
	if err != nil {
		return nil, fmt.Errorf("stub %s response: %w", s.ID, err)
	}

	ep := &Endpoint{
		ID:        s.ID,
		Method:    s.Method,
		Path:      s.Path,
		Headers:   s.Headers,
		Query:     s.Query,
		Payload:   []byte(s.Payload),
		CreatedAt: time.Now(),
		Response:  singleResp,
	}

	// Parse multiple response variants if present
	for _, sr := range s.Responses {
		r, err := parseStubResponse(sr)
		if err != nil {
			return nil, fmt.Errorf("stub %s response variant: %w", s.ID, err)
		}
		ep.Responses = append(ep.Responses, r)
	}

	return ep, nil
}
