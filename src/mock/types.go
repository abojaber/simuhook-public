package mock

import "time"

type Endpoint struct {
	ID       string            `json:"id"`
	Method   string            `json:"method"`
	Path     string            `json:"path"`
	Headers  map[string]string `json:"headers,omitempty"`
	Payload  []byte            `json:"payload,omitempty"`
	Response Response          `json:"response"`
	CreatedAt time.Time        `json:"created_at"`
}

type Response struct {
	StatusCode int               `json:"status_code"`
	Body       []byte            `json:"body,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Delay      time.Duration     `json:"delay,omitempty"`
}
