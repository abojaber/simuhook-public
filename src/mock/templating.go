package mock

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"text/template"
	"time"

	"github.com/brianvoe/gofakeit/v7"
)

// TemplateData holds fields accessible in response templates.
type TemplateData struct {
	UUID      string
	Now       string
	Timestamp int64
	RequestID string
	Method    string
	Path      string
	Body      string

	// parsedBody caches the request body parsed as JSON (any), set in
	// BuildTemplateData via parseJSONBody. nil when the body is empty or
	// not valid JSON. Read by the JSONPath template func; never re-parsed.
	parsedBody any
}

// fakerFuncMap is the curated gofakeit surface (27 entries, TPL-001 §0).
// Built once at package init, read-only afterwards. Safe to read from any
// goroutine: text/template copies it at parse time and never mutates the
// shared map; concurrent reads of a never-written map are race-free. The
// two closures (CreditCardNumber, Price) add no state — they just fix the
// gofakeit arg requirements the template author shouldn't see.
var fakerFuncMap = template.FuncMap{
	"Name":      gofakeit.Name,
	"FirstName": gofakeit.FirstName,
	"LastName":  gofakeit.LastName,
	"Email":     gofakeit.Email,
	"Phone":     gofakeit.Phone,
	"City":      gofakeit.City,
	"Country":   gofakeit.Country,
	"State":     gofakeit.State,
	"Zip":       gofakeit.Zip,
	"Street":    gofakeit.Street,
	"Company":   gofakeit.Company,
	"JobTitle":  gofakeit.JobTitle,

	"IPv4Address": gofakeit.IPv4Address,
	"IPv6Address": gofakeit.IPv6Address,
	"DomainName":  gofakeit.DomainName,
	"MACAddress":  gofakeit.MacAddress,
	"UserAgent":   gofakeit.UserAgent,

	"CreditCardNumber": func() string { return gofakeit.CreditCardNumber(nil) },
	"Price":            func() string { return fmt.Sprintf("%.2f", gofakeit.Price(0.01, 999.99)) }, // D11
	"ProductName":      gofakeit.ProductName,
	"ULID":             gofakeit.ID, // D9: gofakeit has no ULID; ID() is the substitution

	"HexColor":  gofakeit.HexColor,
	"Word":      gofakeit.Word,
	"Sentence":  gofakeit.Sentence,  // D10: deprecated upstream, args ignored
	"Paragraph": gofakeit.Paragraph, // D10: deprecated upstream, args ignored
	"Bool":      gofakeit.Bool,
	"UUID":      gofakeit.UUID,
}

// buildTemplateFuncMap assembles the per-request FuncMap: the four existing
// funcs (byte-identical behavior), the request-bound JSONPath closure, and
// the shared faker funcs. The merge writes into a fresh private map — the
// shared fakerFuncMap is never mutated.
func buildTemplateFuncMap(data *TemplateData, headers, query map[string]string) template.FuncMap {
	m := template.FuncMap{
		"Header":    func(key string) string { return headers[key] },
		"Query":     func(key string) string { return query[key] },
		"RandomInt": randomInt,
		"RandomHex": randomHex,
		"JSONPath":  makeJSONPathFunc(data),
	}
	for name, fn := range fakerFuncMap {
		m[name] = fn
	}
	return m
}

// ExecuteBodyTemplate renders a template string with the given data and funcs.
// Returns the rendered string, or the original if parsing fails.
func ExecuteBodyTemplate(tmplStr string, data *TemplateData, headers, query map[string]string) string {
	if !strings.Contains(tmplStr, "{{") {
		return tmplStr
	}

	tmpl, err := template.New("body").Funcs(buildTemplateFuncMap(data, headers, query)).Parse(tmplStr)
	if err != nil {
		return tmplStr
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return tmplStr
	}
	return buf.String()
}

// ExecuteHeaderTemplate renders a single header value as a template.
func ExecuteHeaderTemplate(value string, data *TemplateData, headers, query map[string]string) string {
	if !strings.Contains(value, "{{") {
		return value
	}
	return ExecuteBodyTemplate(value, data, headers, query)
}

func randomInt(min, max int) int {
	if max <= min {
		return min
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max-min)))
	return min + int(n.Int64())
}

func randomHex(n int) string {
	const hexChars = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		idx, _ := rand.Int(rand.Reader, big.NewInt(16))
		b[i] = hexChars[idx.Int64()]
	}
	return string(b)
}

// BuildTemplateData populates a TemplateData from request context.
// The JSON body is parsed exactly once per request (if valid JSON) and
// cached in parsedBody for the JSONPath template func.
func BuildTemplateData(requestID, method, path string, body []byte) *TemplateData {
	td := &TemplateData{
		UUID:      newUUID(),
		Now:       time.Now().UTC().Format(time.RFC3339),
		Timestamp: time.Now().Unix(),
		RequestID: requestID,
		Method:    method,
		Path:      path,
		Body:      string(body),
	}
	if v, ok := parseJSONBody(body); ok {
		td.parsedBody = v
	}
	return td
}

// newUUID generates a UUID v4 string.
func newUUID() string {
	var buf [16]byte
	rand.Read(buf[:])
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
}
