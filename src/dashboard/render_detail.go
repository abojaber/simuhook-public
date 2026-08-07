package dashboard

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// UXR-015 — request-detail coloring. This file holds the pure helpers and the
// segment-wise assembler shared by detailLines (enter expansion) and
// inspectLines (i inspector). All text reaching these helpers has already been
// sanitized; nothing here ever sniffs rendered ANSI bytes.

// lineKind classifies a built detail/inspect line so it can be styled
// segment-wise. Classification happens at build-site (detailLines/inspectLines
// know the kind of every line they emit); nothing sniffs the rendered bytes.
type lineKind int

const (
	kindFixed     lineKind = iota // fixed field: cyan-bold label + default value
	kindKV                        // header/query line: cyan key + default value
	kindKVResp                    // response header line: magenta key + default value (UXR-017)
	kindBlockHead                 // "headers:" / "query:" block header (whole line label)
	kindMuted                     // placeholders ("(none)"/"(empty)") and cap markers
	kindBodyReq                   // non-JSON request body (yellow 220)
	kindBodyResp                  // non-JSON response body (green 76)
	kindJSONReq                   // JSON request body content (syntax-highlighted)
	kindJSONResp                  // JSON response body content (syntax-highlighted)
	kindReqHeader                 // REQUEST section header (cyan bold)
)

// styledLine pairs a plain (sanitized, untruncated) line with the kind that
// decides how it is coloured. cont marks a body continuation line (no label),
// which is highlighted/tinted as a whole.
type styledLine struct {
	plain string
	kind  lineKind
	cont  bool
}

// detailGutter is the plain gutter prepended to every detail/inspect line. It
// is rendered muted separately so colour codes apply only to content.
const detailGutter = "  │ "

// band wraps a fully styled detail/inspect line in a full-width section
// background band, padded to the viewport width.
//
// Not a naive outer wrap: every styled segment already ends with "\x1b[0m",
// which would clear the background (UXR-008 — segment-wise is required for
// the same reason). band therefore RE-ASSERTS the background after every
// reset and at the line start, then pads with plain spaces (the last
// re-assertion keeps the background active across the padding).
//
// ANSI surgery is safe: sanitize() strips every ESC/control byte from user
// content before styling, so the only "\x1b" bytes in `line` are the ones
// our own package-level lipgloss styles emitted. Replacing "\x1b[0m" can
// never touch user data. Width is measured on the original content
// (lipgloss.Width — ANSI-aware, never len()).
func band(line string, bg lipgloss.Color, width int) string {
	prefix := "\x1b[48;5;" + string(bg) + "m" // assumes a 256-index colour
	body := strings.ReplaceAll(line, "\x1b[0m", "\x1b[0m"+prefix)
	var sb strings.Builder
	sb.WriteString(prefix)
	sb.WriteString(body)
	if pad := width - lipgloss.Width(line); pad > 0 {
		sb.WriteString(strings.Repeat(" ", pad)) // background still active
	}
	sb.WriteString("\x1b[0m")
	return sb.String()
}

// splitKV splits a plain line at the first colon. label includes the colon;
// rest keeps everything after it (leading gap + value). label + rest == line
// byte-for-byte, so %-14s alignment is preserved (splitting at ": " instead
// would eat the first gap space). No colon -> ok=false.
func splitKV(line string) (label, rest string, ok bool) {
	i := strings.IndexByte(line, ':')
	if i < 0 {
		return "", "", false
	}
	return line[:i+1], line[i+1:], true
}

// splitIndent splits leading whitespace from a kv label so the indentation
// stays unstyled while only the key receives the cyan style.
func splitIndent(label string) (indent, key string) {
	trimmed := strings.TrimLeft(label, " ")
	return label[:len(label)-len(trimmed)], trimmed
}

// isMarkerLine reports whether a body line is one of our own cap/truncation
// markers ("… truncated"), which must stay muted rather than tinted or
// highlighted.
func isMarkerLine(line string) bool {
	return strings.HasPrefix(line, "… truncated")
}

// isJSONBody reports whether raw should take formatBody's JSON pretty-print
// path. It must exactly mirror formatBody's JSON-path condition (non-empty,
// under the JSON size cap, and json.Valid) so the highlight decision in
// inspectLines never disagrees with the pretty-print decision.
func isJSONBody(raw string) bool {
	return raw != "" && len(raw) <= maxInspectJSONBytes && json.Valid([]byte(raw))
}

// isJSONWordChar reports whether b is a JSON "word" character, used for the
// token-boundary check on true/false/null (truex must not match true).
func isJSONWordChar(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// highlightJSON tokenizes a single-line pretty-printed JSON line and styles
// it: object keys in keyStyle (cyan 39 for requests / magenta 213 for
// responses — UXR-017), numbers yellow (220), true/false/null magenta (213),
// punctuation muted (240). String values render in the default foreground.
// It is pure/tea-free and unit-testable. Input is expected to have already
// been sanitized and capped by formatBody. Any token that does not match the
// JSON grammar returns the whole line unchanged (whole-line passthrough,
// never partial highlighting) — this is what makes truncated JSON lines
// render unstyled (AC15).
func highlightJSON(line string, keyStyle lipgloss.Style) string {
	var sb strings.Builder
	i := 0
	for i < len(line) {
		c := line[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			sb.WriteByte(c)
			i++
		case c == '{' || c == '}' || c == '[' || c == ']' || c == ',' || c == ':':
			sb.WriteString(styleJSONPunct.Render(string(c)))
			i++
		case c == '"':
			// Scan to the closing unescaped quote (backslash escapes the
			// next byte).
			j := i + 1
			closed := false
			for j < len(line) {
				if line[j] == '\\' {
					j += 2
					continue
				}
				if line[j] == '"' {
					closed = true
					j++
					break
				}
				j++
			}
			if !closed {
				// Unterminated string -> not valid JSON -> passthrough.
				return line
			}
			// Look ahead over whitespace: a ':' marks an object KEY.
			k := j
			for k < len(line) && (line[k] == ' ' || line[k] == '\t' || line[k] == '\r' || line[k] == '\n') {
				k++
			}
			if k < len(line) && line[k] == ':' {
				sb.WriteString(keyStyle.Render(line[i:j]))
			} else {
				sb.WriteString(line[i:j]) // string value — unstyled
			}
			i = j
		case c == '-' || (c >= '0' && c <= '9'):
			// Number: -?[0-9]+(\.[0-9]+)?([eE][+-]?[0-9]+)?
			j := i
			if line[j] == '-' {
				j++
			}
			digits := 0
			for j < len(line) && line[j] >= '0' && line[j] <= '9' {
				j++
				digits++
			}
			if digits == 0 {
				return line
			}
			if j < len(line) && line[j] == '.' {
				j++
				frac := 0
				for j < len(line) && line[j] >= '0' && line[j] <= '9' {
					j++
					frac++
				}
				if frac == 0 {
					return line
				}
			}
			if j < len(line) && (line[j] == 'e' || line[j] == 'E') {
				j++
				if j < len(line) && (line[j] == '+' || line[j] == '-') {
					j++
				}
				exp := 0
				for j < len(line) && line[j] >= '0' && line[j] <= '9' {
					j++
					exp++
				}
				if exp == 0 {
					return line
				}
			}
			sb.WriteString(styleJSONNumber.Render(line[i:j]))
			i = j
		case c == 't' || c == 'f' || c == 'n':
			var word string
			switch c {
			case 't':
				word = "true"
			case 'f':
				word = "false"
			case 'n':
				word = "null"
			}
			if i+len(word) <= len(line) && line[i:i+len(word)] == word &&
				(i+len(word) == len(line) || !isJSONWordChar(line[i+len(word)])) {
				sb.WriteString(styleJSONBool.Render(word))
				i += len(word)
				continue
			}
			return line
		default:
			// Unrecognized -> whole-line passthrough, never partial styling.
			return line
		}
	}
	return sb.String()
}

// renderStyledLine builds one fully styled detail/inspect line from a plain
// styledLine. Ordering invariant (UXR-015): the plain line was built at
// build-site (after sanitize), truncated here on the PLAIN string, then split
// into label/rest segments which are styled individually. The muted gutter is
// prepended last. The returned line never contains a newline.
func renderStyledLine(sl styledLine, contentW int) string {
	line := truncPlain(sl.plain, contentW)
	if sl.cont {
		switch sl.kind {
		case kindBodyReq:
			return styleMuted.Render(detailGutter) + styleRequestBody.Render(line)
		case kindBodyResp:
			return styleMuted.Render(detailGutter) + styleResponseBody.Render(line)
		case kindJSONReq:
			return styleMuted.Render(detailGutter) + highlightJSON(line, styleJSONKey)
		case kindJSONResp:
			return styleMuted.Render(detailGutter) + highlightJSON(line, styleJSONKeyResp)
		}
	}
	switch sl.kind {
	case kindReqHeader:
		return styleLabel.Render(detailGutter + line)
	case kindFixed:
		label, rest, ok := splitKV(line)
		if !ok {
			return styleMuted.Render(detailGutter + line)
		}
		return styleMuted.Render(detailGutter) + styleLabel.Render(label) + rest
	case kindKV:
		label, rest, ok := splitKV(line)
		if !ok {
			return styleMuted.Render(detailGutter + line)
		}
		indent, key := splitIndent(label)
		return styleMuted.Render(detailGutter) + indent + styleKey.Render(key) + rest
	case kindKVResp:
		label, rest, ok := splitKV(line)
		if !ok {
			return styleMuted.Render(detailGutter + line)
		}
		indent, key := splitIndent(label)
		return styleMuted.Render(detailGutter) + indent + styleKeyResp.Render(key) + rest
	case kindBlockHead:
		return styleMuted.Render(detailGutter) + styleLabel.Render(line)
	case kindMuted:
		return styleMuted.Render(detailGutter + line)
	case kindBodyReq:
		label, rest, ok := splitKV(line)
		if !ok {
			return styleMuted.Render(detailGutter + line)
		}
		return styleMuted.Render(detailGutter) + styleLabel.Render(label) + styleRequestBody.Render(rest)
	case kindBodyResp:
		label, rest, ok := splitKV(line)
		if !ok {
			return styleMuted.Render(detailGutter + line)
		}
		return styleMuted.Render(detailGutter) + styleLabel.Render(label) + styleResponseBody.Render(rest)
	case kindJSONReq:
		label, rest, ok := splitKV(line)
		if !ok {
			return styleMuted.Render(detailGutter + line)
		}
		return styleMuted.Render(detailGutter) + styleLabel.Render(label) + highlightJSON(rest, styleJSONKey)
	case kindJSONResp:
		label, rest, ok := splitKV(line)
		if !ok {
			return styleMuted.Render(detailGutter + line)
		}
		return styleMuted.Render(detailGutter) + styleLabel.Render(label) + highlightJSON(rest, styleJSONKeyResp)
	}
	return styleMuted.Render(detailGutter + line)
}

// appendBodyBlock appends a formatted body (via formatBody) to the given line
// list. label is "request body:" or "response body:". The body is classified
// once (isJSONBody): JSON bodies are syntax-highlighted per line, plain bodies
// get a whole-section tint (yellow request / green response). "(empty)"
// placeholders and "… truncated" markers stay muted.
func appendBodyBlock(lines *[]styledLine, label, raw string, isRequest bool) {
	if raw == "" {
		*lines = append(*lines, styledLine{plain: fmt.Sprintf("%-14s %s", label, "(empty)"), kind: kindMuted})
		return
	}
	bodyLines := formatBody(raw)
	if len(bodyLines) == 0 {
		*lines = append(*lines, styledLine{plain: fmt.Sprintf("%-14s %s", label, "(empty)"), kind: kindMuted})
		return
	}
	kind := kindBodyReq
	if !isRequest {
		kind = kindBodyResp
	}
	if isJSONBody(raw) {
		if isRequest {
			kind = kindJSONReq
		} else {
			kind = kindJSONResp
		}
	}
	for i, bl := range bodyLines {
		if isMarkerLine(bl) {
			*lines = append(*lines, styledLine{plain: bl, kind: kindMuted})
			continue
		}
		if i == 0 {
			*lines = append(*lines, styledLine{plain: fmt.Sprintf("%-14s %s", label, bl), kind: kind})
		} else {
			*lines = append(*lines, styledLine{plain: "  " + bl, kind: kind, cont: true})
		}
	}
}
