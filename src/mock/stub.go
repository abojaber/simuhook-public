package mock

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type stubFile struct {
	ID       string            `yaml:"id"`
	Method   string            `yaml:"method"`
	Path     string            `yaml:"path"`
	Headers  map[string]string `yaml:"headers,omitempty"`
	Query    map[string]string `yaml:"query,omitempty"`
	Payload  string            `yaml:"payload,omitempty"`
	Response stubResponse      `yaml:"response"`
}

type stubResponse struct {
	StatusCode int               `yaml:"status_code"`
	Body       string            `yaml:"body,omitempty"`
	Headers    map[string]string `yaml:"headers,omitempty"`
	Delay      string            `yaml:"delay,omitempty"`
	Echo       bool              `yaml:"echo,omitempty"`
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

	ep := &Endpoint{
		ID:        s.ID,
		Method:    s.Method,
		Path:      s.Path,
		Headers:   s.Headers,
		Query:     s.Query,
		Payload:   []byte(s.Payload),
		CreatedAt: time.Now(),
		Response: Response{
			StatusCode: s.Response.StatusCode,
			Body:       []byte(s.Response.Body),
			Headers:    s.Response.Headers,
			Echo:       s.Response.Echo,
		},
	}

	if s.Response.Delay != "" {
		d, err := time.ParseDuration(s.Response.Delay)
		if err != nil {
			return nil, fmt.Errorf("stub %s invalid delay %q: %w", s.ID, s.Response.Delay, err)
		}
		ep.Response.Delay = d
	}

	return ep, nil
}
