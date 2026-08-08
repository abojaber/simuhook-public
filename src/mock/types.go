package mock

import (
	"regexp"
	"time"
)

type Endpoint struct {
	ID        string            `json:"id"`
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	PathGlob  string            `json:"path_glob,omitempty"`
	PathRegex string            `json:"path_regex,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Query     map[string]string `json:"query,omitempty"`
	Payload   []byte            `json:"payload,omitempty"`
	// PayloadContains is a JSON subset spec parsed once at load (MAT-003).
	// At most one of Payload / PayloadContains may be set — parse error
	// otherwise (enforced by compileEndpoint).
	PayloadContains    any            `json:"payload_contains,omitempty"`
	Response           Response       `json:"response"`
	Responses          []Response     `json:"responses,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	rrIndex            uint64         // round-robin counter (not serialized)
	pathRe             *regexp.Regexp // compiled PathRegex (MAT-002), set at load
	pathGlobSegs       []string       // pre-split PathGlob segments (MAT-001), set at load
	globWeight         int            // D1: `**`=2, `*`=1, `?`=0 (fewer wins), set at load
	hasPayloadContains bool           // distinguishes "null spec" from "unset" (D3)
	seq                uint64         // load-order slot (MAT-004/D8), assigned in Add, preserved on Update
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
