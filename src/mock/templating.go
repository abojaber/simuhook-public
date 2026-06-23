package mock

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"text/template"
	"time"
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
}

// ExecuteBodyTemplate renders a template string with the given data and funcs.
// Returns the rendered string, or the original if parsing fails.
func ExecuteBodyTemplate(tmplStr string, data *TemplateData, headers, query map[string]string) string {
	if !strings.Contains(tmplStr, "{{") {
		return tmplStr
	}

	funcMap := template.FuncMap{
		"Header":    func(key string) string { return headers[key] },
		"Query":     func(key string) string { return query[key] },
		"RandomInt": randomInt,
		"RandomHex": randomHex,
	}

	tmpl, err := template.New("body").Funcs(funcMap).Parse(tmplStr)
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
func BuildTemplateData(requestID, method, path string, body []byte) *TemplateData {
	return &TemplateData{
		UUID:      newUUID(),
		Now:       time.Now().UTC().Format(time.RFC3339),
		Timestamp: time.Now().Unix(),
		RequestID: requestID,
		Method:    method,
		Path:      path,
		Body:      string(body),
	}
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
