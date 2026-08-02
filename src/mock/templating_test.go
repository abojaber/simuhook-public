package mock

import "testing"

// TestRandomInt_MaxLessThanMin verifies randomInt returns min when the range
// is degenerate (max <= min), avoiding any division/panic issues.
func TestRandomInt_MaxLessThanMin(t *testing.T) {
	cases := []struct {
		min, max, want int
	}{
		{5, 5, 5},   // max == min
		{10, 5, 10}, // max < min
		{0, 0, 0},   // both zero
		{-3, -7, -3},
	}
	for _, c := range cases {
		for i := 0; i < 20; i++ {
			got := randomInt(c.min, c.max)
			if got != c.want {
				t.Fatalf("randomInt(%d, %d) = %d, want %d", c.min, c.max, got, c.want)
			}
		}
	}
}

// TestRandomInt_NormalRange verifies randomInt stays within [min, max) when
// the range is valid.
func TestRandomInt_NormalRange(t *testing.T) {
	for i := 0; i < 200; i++ {
		got := randomInt(10, 20)
		if got < 10 || got >= 20 {
			t.Fatalf("randomInt(10, 20) = %d, out of range", got)
		}
	}
}

// TestExecuteBodyTemplate_InvalidSyntax verifies that malformed template
// syntax (e.g. "{{.") falls back to the original string without error.
func TestExecuteBodyTemplate_InvalidSyntax(t *testing.T) {
	input := `{"bad": "{{.}`
	data := &TemplateData{UUID: "abc"}
	result := ExecuteBodyTemplate(input, data, nil, nil)
	if result != input {
		t.Fatalf("expected fallback to original %q, got %q", input, result)
	}
}

// TestExecuteBodyTemplate_UnknownField verifies a parse-valid but unresolved
// field falls back to the original string (Execute returns error → fallback).
func TestExecuteBodyTemplate_UnknownField(t *testing.T) {
	input := `{"x":"{{.NoSuchField}}"}`
	data := &TemplateData{}
	result := ExecuteBodyTemplate(input, data, nil, nil)
	if result != input {
		t.Fatalf("expected fallback to original %q, got %q", input, result)
	}
}

// TestExecuteBodyTemplate_BodyWithTemplateLikeChars verifies that the .Body
// field value is rendered literally, even when it contains template markers.
func TestExecuteBodyTemplate_BodyWithTemplateLikeChars(t *testing.T) {
	bodyContent := `{{.ShouldNotRender}}`
	data := &TemplateData{Body: bodyContent}
	result := ExecuteBodyTemplate(`{"body":"{{.Body}}"}`, data, nil, nil)
	expected := `{"body":"{{.ShouldNotRender}}"}`
	if result != expected {
		t.Fatalf("expected literal body %q, got %q", expected, result)
	}
}

// TestBuildTemplateData_NilBody verifies .Body is an empty string when the
// request body is nil or empty, without panicking.
func TestBuildTemplateData_NilBody(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		data := BuildTemplateData("req-1", "POST", "/hook", nil)
		if data.Body != "" {
			t.Fatalf("expected empty .Body for nil input, got %q", data.Body)
		}
		if data.RequestID != "req-1" || data.Method != "POST" || data.Path != "/hook" {
			t.Fatalf("unexpected metadata: %+v", data)
		}
	})

	t.Run("empty", func(t *testing.T) {
		data := BuildTemplateData("req-2", "GET", "/ping", []byte{})
		if data.Body != "" {
			t.Fatalf("expected empty .Body for empty input, got %q", data.Body)
		}
	})
}

// TestBuildTemplateData_NonEmptyBody verifies .Body carries the raw request body.
func TestBuildTemplateData_NonEmptyBody(t *testing.T) {
	data := BuildTemplateData("req-3", "POST", "/hook", []byte(`{"event":"user.created"}`))
	if data.Body != `{"event":"user.created"}` {
		t.Fatalf("expected raw body, got %q", data.Body)
	}
	if data.UUID == "" {
		t.Error("expected a UUID to be generated")
	}
	if data.Now == "" || data.Timestamp == 0 {
		t.Error("expected timestamp fields to be populated")
	}
}
