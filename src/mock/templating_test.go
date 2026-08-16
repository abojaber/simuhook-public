package mock

import (
	"regexp"
	"strings"
	"testing"
)

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

// --- TPL-001 / TPL-002 additions (phase C) ---

// wantFakerFuncs is the exact curated FuncMap key set (TPL-001 §0,
// 27 entries). TestFuncMapKeys_ExactCuratedSet asserts no leakage
// (RandomString/Number/Int NOT present).
var wantFakerFuncs = []string{
	"Name", "FirstName", "LastName", "Email", "Phone",
	"City", "Country", "State", "Zip", "Street",
	"Company", "JobTitle",
	"IPv4Address", "IPv6Address", "DomainName", "MACAddress", "UserAgent",
	"CreditCardNumber", "Price", "ProductName", "ULID",
	"HexColor", "Word", "Sentence", "Paragraph", "Bool", "UUID",
}

func TestFuncMapKeys_ExactCuratedSet(t *testing.T) {
	if len(fakerFuncMap) != len(wantFakerFuncs) {
		t.Fatalf("fakerFuncMap has %d entries, want %d", len(fakerFuncMap), len(wantFakerFuncs))
	}
	for _, name := range wantFakerFuncs {
		if _, ok := fakerFuncMap[name]; !ok {
			t.Errorf("fakerFuncMap missing %q", name)
		}
	}
	// anti-leak: the explicitly NOT-added funcs
	for _, name := range []string{"RandomString", "Number", "Int"} {
		if _, ok := fakerFuncMap[name]; ok {
			t.Errorf("fakerFuncMap must NOT contain %q", name)
		}
	}
}

func TestFakerFuncs_PresentAndPlausible(t *testing.T) {
	data := BuildTemplateData("req-f", "POST", "/hook", []byte(`{}`))
	// render each func standalone through ExecuteBodyTemplate
	for _, name := range wantFakerFuncs {
		tmpl := "{{" + name + "}}"
		got := ExecuteBodyTemplate(tmpl, data, nil, nil)
		if got == tmpl {
			t.Errorf("func %s returned its own template (fallback) — not rendered", name)
			continue
		}
		if got == "" && name != "Bool" {
			t.Errorf("func %s returned empty string", name)
		}
	}
	// shape checks for a few structured ones
	email := ExecuteBodyTemplate("{{Email}}", data, nil, nil)
	if !strings.Contains(email, "@") {
		t.Errorf("Email = %q, not an email", email)
	}
	phone := ExecuteBodyTemplate("{{Phone}}", data, nil, nil)
	if phone == "" || !strings.ContainsAny(phone, "0123456789") {
		t.Errorf("Phone = %q, not a phone", phone)
	}
	ip := ExecuteBodyTemplate("{{IPv4Address}}", data, nil, nil)
	if !strings.Contains(ip, ".") {
		t.Errorf("IPv4Address = %q, not an IPv4", ip)
	}
	hex := ExecuteBodyTemplate("{{HexColor}}", data, nil, nil)
	if !strings.HasPrefix(hex, "#") {
		t.Errorf("HexColor = %q, not a hex color", hex)
	}
	ulid := ExecuteBodyTemplate("{{ULID}}", data, nil, nil)
	if len(ulid) != 20 {
		t.Errorf("ULID (via gofakeit.ID) = %q, want 20 chars", ulid)
	}
	price := ExecuteBodyTemplate("{{Price}}", data, nil, nil)
	if !regexpMustMatch(`^\d+\.\d{2}$`, price) {
		t.Errorf("Price = %q, want decimal with 2 places", price)
	}
	boolVal := ExecuteBodyTemplate("{{Bool}}", data, nil, nil)
	if boolVal != "true" && boolVal != "false" {
		t.Errorf("Bool = %q, want true/false", boolVal)
	}
}

func regexpMustMatch(pat, s string) bool {
	re := regexp.MustCompile(pat)
	return re.MatchString(s)
}

func TestTemplate_ComposeFakerWithFieldsAndFuncs(t *testing.T) {
	data := BuildTemplateData("req-c", "POST", "/hook", nil)
	data.Body = "dfd"
	headers := map[string]string{"X-Test": "hv"}
	query := map[string]string{"q": "qv"}
	got := ExecuteBodyTemplate(`{{.RequestID}}|{{Email}}|{{Query "q"}}`, data, headers, query)
	if got == "dfd|" {
		t.Fatalf("template not rendered: %q", got)
	}
	if !strings.HasPrefix(got, "req-c|") {
		t.Errorf("got %q, want prefix req-c|", got)
	}
	if !strings.Contains(got, "@") || !strings.Contains(got, "|qv") {
		t.Errorf("got %q, want email + query value", got)
	}
}

func TestTemplate_FakerWithIf(t *testing.T) {
	data := BuildTemplateData("req-if", "POST", "/hook", nil)
	got := ExecuteBodyTemplate(`{{if .RequestID}}{{Name}}{{end}}`, data, nil, nil)
	if got == "" {
		t.Error("expected a name when .RequestID is set")
	}
	// false branch: .Body is empty → {{Zip}} renders in the else branch
	got2 := ExecuteBodyTemplate(`{{if .Body}}{{Name}}{{else}}{{Zip}}{{end}}`, data, nil, nil)
	if got2 == "" {
		t.Error("expected a zip in the else branch")
	}
}

func TestUUIDFuncFieldIndependence(t *testing.T) {
	data := BuildTemplateData("req-u", "POST", "/hook", nil)
	got := ExecuteBodyTemplate(`{{.UUID}}|{{UUID}}`, data, nil, nil)
	parts := strings.Split(got, "|")
	if len(parts) != 2 {
		t.Fatalf("got %q, want UUID|UUID", got)
	}
	if parts[0] == "" || parts[1] == "" {
		t.Fatalf("both UUIDs should be non-empty, got %q", got)
	}
	if parts[0] == parts[1] {
		t.Errorf("field and func UUID collided: %q", got)
	}
}

func TestExistingFuncs_ByteIdentical(t *testing.T) {
	data := BuildTemplateData("req-e", "POST", "/hook", nil)
	// RandomInt with degenerate range must still return min
	if got := ExecuteBodyTemplate(`{{RandomInt 1 1}}`, data, nil, nil); got != "1" {
		t.Errorf("RandomInt 1 1 = %q, want 1", got)
	}
	// RandomHex fixed length
	if got := ExecuteBodyTemplate(`{{RandomHex 4}}`, data, nil, nil); len(got) != 4 {
		t.Errorf("RandomHex 4 = %q, want length 4", got)
	}
}

func TestTemplate_JSONPathBound(t *testing.T) {
	data := BuildTemplateData("req-j", "POST", "/hook", []byte(`{"user":{"name":"Ada"}}`))
	got := ExecuteBodyTemplate(`{{JSONPath "$.user.name"}}`, data, nil, nil)
	if got != "Ada" {
		t.Errorf(`JSONPath $.user.name = %q, want "Ada"`, got)
	}
}

func TestTemplate_BodyFieldUnchanged(t *testing.T) {
	data := BuildTemplateData("req-b", "POST", "/hook", []byte(`{"raw":"body"}`))
	got := ExecuteBodyTemplate(`{"echo":"{{.Body}}"}}`, data, nil, nil)
	if !strings.Contains(got, `{"raw":"body"}`) {
		t.Errorf("got %q, want raw body preserved", got)
	}
}

func TestBuildTemplateData_ParsedBodyNilEmpty(t *testing.T) {
	if got := BuildTemplateData("r", "POST", "/h", nil); got.parsedBody != nil {
		t.Error("nil body → parsedBody must be nil")
	}
	if got := BuildTemplateData("r", "POST", "/h", []byte{}); got.parsedBody != nil {
		t.Error("empty body → parsedBody must be nil")
	}
	if got := BuildTemplateData("r", "POST", "/h", []byte("plain text")); got.parsedBody != nil {
		t.Error("non-JSON body → parsedBody must be nil")
	}
}

func TestBuildTemplateData_ParsedBodyValid(t *testing.T) {
	got := BuildTemplateData("r", "POST", "/h", []byte(`{"a":[1,2]}`))
	if got.parsedBody == nil {
		t.Fatal("valid JSON body → parsedBody must be non-nil")
	}
}

func TestHeaderTemplate_FakerFunc(t *testing.T) {
	data := BuildTemplateData("req-h", "POST", "/hook", nil)
	got := ExecuteHeaderTemplate(`X-Custom: {{Company}}`, data, nil, nil)
	if got == `X-Custom: {{Company}}` {
		t.Error("header template not rendered")
	}
	if !strings.HasPrefix(got, "X-Custom: ") {
		t.Errorf("got %q, want X-Custom: prefix", got)
	}
}

func TestExecuteBodyTemplate_UnknownFunc_FallsBack(t *testing.T) {
	input := `{{Nope}}`
	data := BuildTemplateData("req-n", "POST", "/hook", nil)
	if got := ExecuteBodyTemplate(input, data, nil, nil); got != input {
		t.Errorf("unknown func must fall back to original, got %q", got)
	}
}

func TestExecuteBodyTemplate_JSONPathNonJSONFallsBackEmpty(t *testing.T) {
	// A template using JSONPath on a NON-JSON body renders "" inside an
	// otherwise-valid template — no panic, no whole-template fallback.
	data := BuildTemplateData("req-nj", "POST", "/hook", []byte("plain text"))
	got := ExecuteBodyTemplate(`id={{JSONPath "$.id"}}`, data, nil, nil)
	if got != "id=" {
		t.Errorf(`got %q, want "id=" (empty JSONPath on non-JSON body)`, got)
	}
}
