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
