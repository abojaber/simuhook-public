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
	PathGlob  string            `yaml:"path_glob,omitempty"`
	PathRegex string            `yaml:"path_regex,omitempty"`
	Headers   map[string]string `yaml:"headers,omitempty"`
	Query     map[string]string `yaml:"query,omitempty"`
	Payload   string            `yaml:"payload,omitempty"`
	// PayloadContains is the raw JSON text of the payload_contains spec.
	// It must be quoted in YAML; parsed at load into Endpoint.PayloadContains.
	PayloadContains string         `yaml:"payload_contains,omitempty"`
	Response        stubResponse   `yaml:"response"`
	Responses       []stubResponse `yaml:"responses,omitempty"`
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
		PathGlob:  s.PathGlob,
		PathRegex: s.PathRegex,
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

	// Parse the payload_contains spec (MAT-003) and compile path matchers
	// (MAT-005) as the LAST steps so every validation error is wrapped with
	// the stub ID and the watcher logs skip/parse-error consistently with
	// the existing invalid-delay handling.
	if err := parsePayloadContains(ep, s.PayloadContains); err != nil {
		return nil, fmt.Errorf("stub %s: %w", s.ID, err)
	}
	if err := compileEndpoint(ep); err != nil {
		return nil, fmt.Errorf("stub %s: %w", s.ID, err)
	}

	return ep, nil
}
