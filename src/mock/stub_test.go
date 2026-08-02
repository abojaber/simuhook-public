package mock

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseStubFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ep.yaml")
	os.WriteFile(path, []byte(`
id: my-ep
method: POST
path: /hook
headers:
  X-Key: val
payload: '{"a":1}'
response:
  status_code: 201
  body: '{"ok":true}'
  headers:
    X-Resp: hi
  delay: 500ms
`), 0644)

	ep, err := ParseStubFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep.ID != "my-ep" {
		t.Fatalf("expected my-ep, got %s", ep.ID)
	}
	if ep.Method != "POST" {
		t.Fatalf("expected POST, got %s", ep.Method)
	}
	if ep.Path != "/hook" {
		t.Fatalf("expected /hook, got %s", ep.Path)
	}
	if ep.Headers["X-Key"] != "val" {
		t.Fatalf("expected header X-Key=val")
	}
	if string(ep.Payload) != `{"a":1}` {
		t.Fatalf("expected payload, got %s", string(ep.Payload))
	}
	if ep.Response.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", ep.Response.StatusCode)
	}
	if string(ep.Response.Body) != `{"ok":true}` {
		t.Fatalf("expected body, got %s", string(ep.Response.Body))
	}
	if ep.Response.Headers["X-Resp"] != "hi" {
		t.Fatalf("expected response header X-Resp=hi")
	}
	if ep.Response.Delay != 500*time.Millisecond {
		t.Fatalf("expected 500ms delay, got %v", ep.Response.Delay)
	}
}

func TestParseStubFile_Minimal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "min.yaml")
	os.WriteFile(path, []byte(`
id: minimal
method: GET
path: /ping
response:
  status_code: 200
`), 0644)

	ep, err := ParseStubFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep.ID != "minimal" {
		t.Fatalf("expected minimal, got %s", ep.ID)
	}
	if ep.Response.Delay != 0 {
		t.Fatalf("expected no delay, got %v", ep.Response.Delay)
	}
}

func TestParseStubFile_MissingID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "noid.yaml")
	os.WriteFile(path, []byte(`method: GET
path: /x
response:
  status_code: 200
`), 0644)

	_, err := ParseStubFile(path)
	if err == nil {
		t.Fatal("expected error for missing id")
	}
}

func TestParseStubFile_InvalidDelay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baddelay.yaml")
	os.WriteFile(path, []byte(`
id: bad
method: GET
path: /x
response:
  status_code: 200
  delay: xyz
`), 0644)

	_, err := ParseStubFile(path)
	if err == nil {
		t.Fatal("expected error for invalid delay")
	}
}

func TestParseStubFile_NoMethod(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wild.yaml")
	os.WriteFile(path, []byte(`
id: catch-all
path: /any
response:
  status_code: 200
  body: ok
`), 0644)

	ep, err := ParseStubFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep.Method != "" {
		t.Fatalf("expected empty method, got %s", ep.Method)
	}
}

func TestParseStubFile_Echo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "echo.yaml")
	os.WriteFile(path, []byte(`
id: echo-test
method: POST
path: /echo
response:
  status_code: 200
  echo: true
`), 0644)

	ep, err := ParseStubFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ep.Response.Echo {
		t.Fatal("expected echo=true")
	}
	if string(ep.Response.Body) != "" {
		t.Fatalf("expected empty body when echo is set, got %q", string(ep.Response.Body))
	}
}

func TestParseStubFile_EchoDefaultsFalse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "normal.yaml")
	os.WriteFile(path, []byte(`
id: normal
method: GET
path: /ping
response:
  status_code: 200
  body: ok
`), 0644)

	ep, err := ParseStubFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep.Response.Echo {
		t.Fatal("expected echo=false by default")
	}
}

func TestParseStubFile_Template(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tmpl.yaml")
	os.WriteFile(path, []byte(`
id: tmpl-test
method: POST
path: /tmpl
response:
  status_code: 200
  template: true
  body: '{"id":"{{.UUID}}","time":"{{.Now}}"}'
  headers:
    X-ID: "{{.RequestID}}"
`), 0644)

	ep, err := ParseStubFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ep.Response.Template {
		t.Fatal("expected template=true")
	}
	if string(ep.Response.Body) != `{"id":"{{.UUID}}","time":"{{.Now}}"}` {
		t.Fatalf("expected raw template body, got %q", string(ep.Response.Body))
	}
	if ep.Response.Headers["X-ID"] != "{{.RequestID}}" {
		t.Fatalf("expected raw template header, got %q", ep.Response.Headers["X-ID"])
	}
}

func TestParseStubFile_TemplateDefaultsFalse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plain.yaml")
	os.WriteFile(path, []byte(`
id: plain
method: GET
path: /plain
response:
  status_code: 200
  body: static
`), 0644)

	ep, err := ParseStubFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep.Response.Template {
		t.Fatal("expected template=false by default")
	}
}

func TestParseStubFile_MultipleResponses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "varied.yaml")
	os.WriteFile(path, []byte(`
id: varied
method: GET
path: /status
responses:
  - status_code: 200
    body: ok
    weight: 80
  - status_code: 500
    body: fail
    weight: 20
    delay: 1s
`), 0644)

	ep, err := ParseStubFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ep.Responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(ep.Responses))
	}
	if ep.Responses[0].StatusCode != 200 {
		t.Fatalf("expected 200, got %d", ep.Responses[0].StatusCode)
	}
	if ep.Responses[0].Weight != 80 {
		t.Fatalf("expected weight 80, got %d", ep.Responses[0].Weight)
	}
	if string(ep.Responses[0].Body) != "ok" {
		t.Fatalf("expected body ok, got %q", string(ep.Responses[0].Body))
	}
	if ep.Responses[1].StatusCode != 500 {
		t.Fatalf("expected 500, got %d", ep.Responses[1].StatusCode)
	}
	if ep.Responses[1].Weight != 20 {
		t.Fatalf("expected weight 20, got %d", ep.Responses[1].Weight)
	}
	if ep.Responses[1].Delay != 1_000_000_000 {
		t.Fatalf("expected 1s delay, got %v", ep.Responses[1].Delay)
	}
}

func TestParseStubFile_ResponsesWithEchoAndTemplate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "advanced.yaml")
	os.WriteFile(path, []byte(`
id: advanced
method: POST
path: /adv
responses:
  - status_code: 200
    echo: true
  - status_code: 200
    template: true
    body: '{"id":"{{.UUID}}"}'
`), 0644)

	ep, err := ParseStubFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ep.Responses[0].Echo {
		t.Fatal("expected echo=true on first variant")
	}
	if !ep.Responses[1].Template {
		t.Fatal("expected template=true on second variant")
	}
	if string(ep.Responses[1].Body) != `{"id":"{{.UUID}}"}` {
		t.Fatalf("expected template body, got %q", string(ep.Responses[1].Body))
	}
}

func TestParseStubFile_WithYmlExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "endpoint.yml")
	os.WriteFile(path, []byte(`
id: yml-ep
method: GET
path: /yml
response:
  status_code: 200
  body: ok
`), 0644)

	ep, err := ParseStubFile(path)
	if err != nil {
		t.Fatalf("unexpected error for .yml file: %v", err)
	}
	if ep.ID != "yml-ep" {
		t.Fatalf("expected yml-ep, got %s", ep.ID)
	}
}

func TestParseStubFile_IgnoresUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unknown.yaml")
	os.WriteFile(path, []byte(`
id: unknown-ep
method: POST
path: /unknown
unexpected_top_level: whatever
headers:
  X-Key: val
unknown_nested:
  foo: bar
response:
  status_code: 200
  body: ok
  custom_future_field:
    a: 1
`), 0644)

	ep, err := ParseStubFile(path)
	if err != nil {
		t.Fatalf("unknown fields should be ignored, got error: %v", err)
	}
	if ep.ID != "unknown-ep" {
		t.Fatalf("expected unknown-ep, got %s", ep.ID)
	}
	if ep.Headers["X-Key"] != "val" {
		t.Fatalf("expected known header to still parse, got %v", ep.Headers)
	}
	if ep.Response.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", ep.Response.StatusCode)
	}
}
