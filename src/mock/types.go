package mock

import "time"

type Endpoint struct {
	ID        string            `json:"id"`
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	Headers   map[string]string `json:"headers,omitempty"`
	Query     map[string]string `json:"query,omitempty"`
	Payload   []byte            `json:"payload,omitempty"`
	Response  Response          `json:"response"`
	Responses []Response        `json:"responses,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	rrIndex   uint64            // round-robin counter (not serialized)
}

type Response struct {
	StatusCode int               `json:"status_code"`
	Body       []byte            `json:"body,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Delay      time.Duration     `json:"delay,omitempty"`
	Echo       bool              `json:"echo,omitempty"`
	Template   bool              `json:"template,omitempty"`
	Weight     int               `json:"weight,omitempty"`
}
