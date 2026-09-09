package runtime

// mpr_render.go turns kekkai-mpr wire events (contracts/mpr-wire.md) into
// the transcript or --raw lines (contracts/mpr-cli.md). The proxy ships raw
// bodies; everything presentational — SSE reassembly, layout, color — lives
// here so the baked script stays a dumb pipe (research.md R5).

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// wireEvent is the union of the proxy's event shapes.
type wireEvent struct {
	Type        string `json:"type"`
	ID          int    `json:"id"`
	TS          string `json:"ts"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	Status      int    `json:"status"`
	ContentType string `json:"content_type"`
	DurationMS  int    `json:"duration_ms"`
	Body        string `json:"body"`
	Truncated   bool   `json:"truncated"`
	Count       int    `json:"count"`
	Text        string `json:"text"`
}

// Raw output shapes: field order is the contract's, hence structs not maps.
type rawRequest struct {
	Type      string `json:"type"`
	ID        int    `json:"id"`
	TS        string `json:"ts"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	Body      any    `json:"body"`
	Truncated bool   `json:"truncated,omitempty"`
}

type rawResponse struct {
	Type       string `json:"type"`
	ID         int    `json:"id"`
	TS         string `json:"ts"`
	Status     int    `json:"status"`
	DurationMS int    `json:"duration_ms"`
	Body       any    `json:"body"`
	Truncated  bool   `json:"truncated,omitempty"`
}

type rawNotice struct {
	Type  string `json:"type"`
	TS    string `json:"ts,omitempty"`
	Text  string `json:"text,omitempty"`
	Count int    `json:"count,omitempty"`
}

const (
	separatorWidth = 80
	messagesPath   = "/v1/messages"
)

type mprRenderer struct {
	raw   bool
	color bool
}

func (r mprRenderer) c(code, s string) string { return paint(r.color, code, s) }

// render returns the output lines for one event; unknown types yield none.
func (r mprRenderer) render(ev wireEvent) []string {
	switch ev.Type {
	case "request":
		body := parseBody(ev)
		if r.raw {
			return []string{compactJSON(rawRequest{"request", ev.ID, ev.TS, ev.Method, ev.Path, body, ev.Truncated})}
		}
		return r.renderRequest(ev, body)
	case "response":
		body := parseBody(ev)
		if r.raw {
			return []string{compactJSON(rawResponse{"response", ev.ID, ev.TS, ev.Status, ev.DurationMS, body, ev.Truncated})}
		}
		return r.renderResponse(ev, body)
	case "notice":
		if r.raw {
			return []string{compactJSON(rawNotice{Type: "notice", TS: ev.TS, Text: ev.Text})}
		}
		return []string{r.c(ansiYellow, fmt.Sprintf("---- notice %s %s ----", localClock(ev.TS), ev.Text))}
	case "dropped":
		if r.raw {
			return []string{compactJSON(rawNotice{Type: "dropped", Count: ev.Count})}
		}
		return []string{r.c(ansiYellow, fmt.Sprintf("---- notice %s %d events dropped (slow reader) ----",
			time.Now().Format("15:04:05"), ev.Count))}
	}
	return nil
}

// parseBody yields the JSON value of a body, the reassembled message for an
// SSE body, or the raw string when neither applies.
func parseBody(ev wireEvent) any {
	if strings.HasPrefix(ev.ContentType, "text/event-stream") {
		if msg, ok := reassembleSSE(ev.Body); ok {
			return msg
		}
		return ev.Body
	}
	var v any
	if err := json.Unmarshal([]byte(ev.Body), &v); err != nil {
		return ev.Body
	}
	return v
}

// --- transcript ---------------------------------------------------------

func (r mprRenderer) renderRequest(ev wireEvent, body any) []string {
	head := fmt.Sprintf("==== #%d REQUEST  %s %s %s ", ev.ID, localClock(ev.TS), ev.Method, ev.Path)
	lines := []string{r.c(ansiCyanBold, padSeparator(head))}
	m, ok := body.(map[string]any)
	if !ok || pathOnly(ev.Path) != messagesPath {
		return r.truncatedTrailer(lines, ev)
	}
	lines = append(lines, "model: "+str(m["model"]))
	// Every top-level scalar/param not rendered structurally below goes on
	// one line — nothing in the request is elided (FR-008).
	var params []string
	for _, k := range sortedKeys(m) {
		switch k {
		case "model", "system", "tools", "messages":
			continue
		}
		params = append(params, k+": "+compactJSON(m[k]))
	}
	if len(params) > 0 {
		lines = append(lines, strings.Join(params, "  "))
	}
	if sys, ok := m["system"]; ok {
		lines = append(lines, r.c(ansiMagenta, "system:"))
		lines = append(lines, r.contentLines(sys)...)
	}
	if tools, ok := m["tools"].([]any); ok && len(tools) > 0 {
		lines = append(lines, r.c(ansiMagenta, fmt.Sprintf("tools (%d):", len(tools))))
		for _, t := range tools {
			tm, _ := t.(map[string]any)
			desc := splitLines(str(tm["description"]))
			first := ""
			if len(desc) > 0 {
				first = " " + desc[0]
			}
			lines = append(lines, "  "+r.c(ansiYellow, str(tm["name"])+":")+first)
			for _, d := range desc[min(1, len(desc)):] {
				lines = append(lines, "    "+d)
			}
			if schema, ok := tm["input_schema"]; ok {
				lines = append(lines, "    input_schema: "+compactJSON(schema))
			}
		}
	}
	if msgs, ok := m["messages"].([]any); ok {
		lines = append(lines, r.c(ansiMagenta, "messages:"))
		for _, msg := range msgs {
			mm, _ := msg.(map[string]any)
			lines = append(lines, r.roleLine(str(mm["role"]), ""))
			lines = append(lines, r.contentLines(mm["content"])...)
		}
	}
	return r.truncatedTrailer(lines, ev)
}

func (r mprRenderer) renderResponse(ev wireEvent, body any) []string {
	head := fmt.Sprintf("==== #%d RESPONSE %s %d %s ", ev.ID, localClock(ev.TS), ev.Status, duration(ev.DurationMS))
	code := ansiCyanBold
	if ev.Status >= 400 {
		code = ansiRed
	}
	lines := []string{r.c(code, padSeparator(head))}
	m, _ := body.(map[string]any)
	switch {
	case ev.Status >= 400 || str(m["type"]) == "error":
		if errm, ok := m["error"].(map[string]any); ok {
			lines = append(lines, r.c(ansiRed, "error "+str(errm["type"])+": "+str(errm["message"])))
		} else if m == nil {
			lines = append(lines, r.c(ansiRed, strings.TrimSpace(ev.Body)))
		} else {
			lines = append(lines, r.c(ansiRed, compactJSON(m)))
		}
	case str(m["type"]) == "message":
		lines = append(lines, r.roleLine("assistant", " "+str(m["model"])+" stop_reason="+str(m["stop_reason"])))
		lines = append(lines, r.contentLines(m["content"])...)
		if u, ok := m["usage"].(map[string]any); ok {
			lines = append(lines, fmt.Sprintf("usage: input %s (cache read %s, cache write %s) output %s",
				num(u["input_tokens"]), num(u["cache_read_input_tokens"]),
				num(u["cache_creation_input_tokens"]), num(u["output_tokens"])))
		}
	}
	return r.truncatedTrailer(lines, ev)
}

func (r mprRenderer) truncatedTrailer(lines []string, ev wireEvent) []string {
	if ev.Truncated {
		lines = append(lines, r.c(ansiRed, "[truncated at 16 MiB]"))
	}
	return lines
}

func (r mprRenderer) roleLine(role, suffix string) string {
	code := ansiGreen
	if role == "assistant" {
		code = ansiBlue
	}
	return r.c(code, "["+role+"]") + suffix
}

// contentLines renders a `content` value: a string, or a list of blocks.
func (r mprRenderer) contentLines(content any) []string {
	switch v := content.(type) {
	case string:
		return splitLines(v)
	case []any:
		var lines []string
		for _, b := range v {
			if bm, ok := b.(map[string]any); ok {
				lines = append(lines, r.blockLines(bm)...)
			}
		}
		return lines
	}
	return nil
}

func (r mprRenderer) blockLines(b map[string]any) []string {
	switch t := str(b["type"]); t {
	case "text":
		return splitLines(str(b["text"]))
	case "tool_use":
		return append([]string{r.c(ansiYellow, "tool_use "+str(b["name"])+" "+str(b["id"]))},
			splitLines(prettyJSON(b["input"]))...)
	case "tool_result":
		head := "tool_result " + str(b["tool_use_id"])
		if isErr, _ := b["is_error"].(bool); isErr {
			head += " [is_error]"
		}
		return append([]string{r.c(ansiYellow, head)}, r.contentLines(b["content"])...)
	case "thinking":
		return append([]string{r.c(ansiMagenta, "thinking:")}, splitLines(str(b["thinking"]))...)
	case "redacted_thinking":
		return []string{r.c(ansiMagenta, "[redacted thinking]")}
	case "image", "document":
		src, _ := b["source"].(map[string]any)
		switch str(src["type"]) {
		case "base64":
			return []string{fmt.Sprintf("[%s %s %s]", t, str(src["media_type"]), humanSize(base64Size(str(src["data"]))))}
		case "url":
			return []string{fmt.Sprintf("[%s url %s]", t, str(src["url"]))}
		}
		return []string{"[" + t + "]"}
	default:
		return []string{"[" + t + "] " + compactJSON(b)}
	}
}

// --- SSE reassembly -------------------------------------------------------

// reassembleSSE folds a Messages streaming body into the non-streamed
// message shape. Returns false when no message_start was seen (not a
// Messages stream). A mid-stream error event is attached as `error`; an
// error before any message is returned as the error object itself.
func reassembleSSE(body string) (any, bool) {
	type acc struct{ text, thinking, partialJSON strings.Builder }
	var msg map[string]any
	var blocks []map[string]any
	accs := map[int]*acc{}
	var errObj map[string]any

	blockAt := func(i int) map[string]any {
		for len(blocks) <= i {
			blocks = append(blocks, nil)
		}
		if blocks[i] == nil {
			blocks[i] = map[string]any{"type": "unknown"}
		}
		if accs[i] == nil {
			accs[i] = &acc{}
		}
		return blocks[i]
	}

	for _, data := range sseData(body) {
		var d map[string]any
		if json.Unmarshal([]byte(data), &d) != nil {
			continue
		}
		idx := int(toFloat(d["index"]))
		switch str(d["type"]) {
		case "message_start":
			if m, ok := d["message"].(map[string]any); ok {
				msg = m
			}
		case "content_block_start":
			if cb, ok := d["content_block"].(map[string]any); ok {
				blockAt(idx)
				blocks[idx] = cb
			}
		case "content_block_delta":
			delta, _ := d["delta"].(map[string]any)
			b := blockAt(idx)
			a := accs[idx]
			switch str(delta["type"]) {
			case "text_delta":
				a.text.WriteString(str(delta["text"]))
			case "input_json_delta":
				a.partialJSON.WriteString(str(delta["partial_json"]))
			case "thinking_delta":
				a.thinking.WriteString(str(delta["thinking"]))
			case "signature_delta":
				b["signature"] = delta["signature"]
			}
		case "message_delta":
			if msg == nil {
				msg = map[string]any{}
			}
			if delta, ok := d["delta"].(map[string]any); ok {
				for k, v := range delta {
					msg[k] = v
				}
			}
			if u, ok := d["usage"].(map[string]any); ok {
				usage, _ := msg["usage"].(map[string]any)
				if usage == nil {
					usage = map[string]any{}
				}
				for k, v := range u {
					usage[k] = v
				}
				msg["usage"] = usage
			}
		case "error":
			errObj = d
		}
	}

	if msg == nil {
		if errObj != nil {
			return errObj, true
		}
		return nil, false
	}
	content := make([]any, 0, len(blocks))
	for i, b := range blocks {
		if b == nil {
			continue
		}
		if a := accs[i]; a != nil {
			if a.text.Len() > 0 {
				b["text"] = str(b["text"]) + a.text.String()
			}
			if a.thinking.Len() > 0 {
				b["thinking"] = str(b["thinking"]) + a.thinking.String()
			}
			if a.partialJSON.Len() > 0 {
				var in any
				if json.Unmarshal([]byte(a.partialJSON.String()), &in) == nil {
					b["input"] = in
				} else {
					b["input_partial"] = a.partialJSON.String()
				}
			}
		}
		content = append(content, b)
	}
	msg["content"] = content
	if errObj != nil {
		msg["error"] = errObj["error"]
	}
	return msg, true
}

// sseData returns each event's joined `data:` payload, in order.
func sseData(body string) []string {
	var out []string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			out = append(out, strings.Join(cur, "\n"))
			cur = nil
		}
	}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			flush()
			continue
		}
		if v, ok := strings.CutPrefix(line, "data:"); ok {
			cur = append(cur, strings.TrimPrefix(v, " "))
		}
	}
	flush()
	return out
}

// --- helpers ----------------------------------------------------------------

func padSeparator(head string) string {
	if len(head) >= separatorWidth {
		return head
	}
	return head + strings.Repeat("=", separatorWidth-len(head))
}

func localClock(ts string) string {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return ts
	}
	return t.Local().Format("15:04:05")
}

func duration(ms int) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}

func pathOnly(p string) string {
	if i := strings.IndexByte(p, '?'); i >= 0 {
		return p[:i]
	}
	return p
}

func str(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case nil:
		return ""
	}
	return compactJSON(v)
}

func num(v any) string {
	return fmt.Sprintf("%.0f", toFloat(v))
}

func toFloat(v any) float64 {
	f, _ := v.(float64)
	return f
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// compactJSON / prettyJSON keep < > & literal: this is a transcript for
// humans, not HTML.
func compactJSON(v any) string { return marshal(v, "") }
func prettyJSON(v any) string  { return marshal(v, "  ") }

func marshal(v any, indent string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", indent)
	if err := enc.Encode(v); err != nil {
		return fmt.Sprint(v)
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

func base64Size(data string) int {
	n := len(strings.TrimRight(data, "="))
	return n * 3 / 4
}

func humanSize(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}
