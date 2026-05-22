package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// renderTranscript reads a Claude Code JSONL file and turns it into a
// readable plain-text rendering suitable for a viewport. Meta entries are
// dropped entirely so the user sees the actual conversation.
//
// When full is true, no trimming is applied to thinking / tool_use /
// tool_result blocks — the user has asked to see everything.
func renderTranscript(path string, full bool) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var b strings.Builder
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry struct {
			Type    string          `json:"type"`
			Message json.RawMessage `json:"message"`
		}
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if entry.Type != "user" && entry.Type != "assistant" && entry.Type != "message" {
			continue
		}
		role, text := extractMessage(entry.Message, full)
		if role == "" {
			role = entry.Type
		}
		if text == "" {
			continue
		}
		b.WriteString(formatTurn(role, text))
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return b.String(), nil
}

func extractMessage(raw json.RawMessage, full bool) (role, text string) {
	if len(raw) == 0 {
		return "", ""
	}
	var msg struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return "", ""
	}
	return msg.Role, contentText(msg.Content, full)
}

// contentText collapses Claude's content (either a string or an array of
// content blocks) into a readable string. Thinking, tool_use, and tool_result
// blocks get their payloads summarized so the reader can follow what
// actually happened.
func contentText(raw json.RawMessage, full bool) string {
	if len(raw) == 0 {
		return ""
	}
	// String form.
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
	}
	// Array of blocks.
	var blocks []struct {
		Type     string          `json:"type"`
		Text     string          `json:"text"`
		Thinking string          `json:"thinking"`
		Name     string          `json:"name"`
		Input    json.RawMessage `json:"input"`
		Content  json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return string(raw)
	}

	var parts []string
	for _, blk := range blocks {
		switch blk.Type {
		case "text":
			if blk.Text != "" {
				parts = append(parts, blk.Text)
			}
		case "thinking":
			parts = append(parts, renderThinking(blk.Thinking, full))
		case "tool_use":
			parts = append(parts, renderToolUse(blk.Name, blk.Input, full))
		case "tool_result":
			parts = append(parts, renderToolResult(blk.Content, full))
		default:
			if blk.Type != "" {
				parts = append(parts, mutedStyle.Render("["+blk.Type+"]"))
			}
		}
	}
	return strings.Join(parts, "\n")
}

func renderThinking(thinking string, full bool) string {
	header := mutedStyle.Render("[thinking]")
	text := thinking
	if !full {
		text = clamp(thinking, 3, 300)
	}
	body := indent(text, "  ")
	if body == "" {
		return header
	}
	return header + "\n" + mutedStyle.Render(body)
}

func renderToolUse(name string, input json.RawMessage, full bool) string {
	label := accentStyle.Render(fmt.Sprintf("[%s]", name))
	summary := summarizeToolInput(name, input, full)
	if summary == "" {
		return label
	}
	return label + " " + summary
}

func renderToolResult(content json.RawMessage, full bool) string {
	header := mutedStyle.Render("[tool result]")
	body := toolResultText(content)
	if body == "" {
		return header
	}
	if !full {
		body = clamp(body, 3, 300)
	}
	return header + "\n" + mutedStyle.Render(indent(body, "  "))
}

// summarizeToolInput formats a tool's input compactly. Known tools get a
// purpose-built one-line summary; unknown tools fall back to a generic
// "key=value" trim.
func summarizeToolInput(name string, input json.RawMessage, full bool) string {
	if len(input) == 0 {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal(input, &obj); err != nil {
		return ""
	}

	pick := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := obj[k]; ok {
				if s, ok := v.(string); ok && s != "" {
					return s
				}
			}
		}
		return ""
	}

	maybeOneLine := func(s string) string {
		if full {
			return strings.TrimSpace(s)
		}
		return oneLine(s)
	}

	switch strings.ToLower(name) {
	case "bash":
		return maybeOneLine(pick("command"))
	case "read", "write", "edit", "notebookedit":
		return pick("file_path", "path", "notebook_path")
	case "grep":
		pat := pick("pattern", "query")
		path := pick("path")
		if path != "" {
			return pat + "   " + mutedStyle.Render("in "+path)
		}
		return pat
	case "glob":
		return pick("pattern")
	case "webfetch", "webfetch_url":
		return pick("url")
	case "task", "agent":
		return maybeOneLine(pick("description", "prompt"))
	}

	// Fallback: dump the first string value we find.
	for _, k := range []string{"description", "prompt", "command", "query", "path", "file_path"} {
		if s := pick(k); s != "" {
			return maybeOneLine(s)
		}
	}
	return ""
}

// toolResultText extracts a readable text from a tool_result block. The
// content can be a string OR an array of blocks (text/image/etc.).
func toolResultText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var parts []string
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				parts = append(parts, b.Text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

// clamp returns at most maxLines of s; if there are more, an ellipsis line
// is added. The output is also truncated to maxChars total to keep the
// transcript view scannable.
func clamp(s string, maxLines, maxChars int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	more := 0
	if len(lines) > maxLines {
		more = len(lines) - maxLines
		lines = lines[:maxLines]
	}
	out := strings.Join(lines, "\n")
	if len(out) > maxChars {
		out = out[:maxChars] + "…"
		// any extra-lines count we computed is now subsumed by the char trim
		more = 0
	}
	if more > 0 {
		out += fmt.Sprintf("\n[+%d more line(s)]", more)
	}
	return out
}

func indent(s, prefix string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

func oneLine(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		s = s[:idx] + " …"
	}
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}

func formatTurn(role, text string) string {
	var roleLine string
	switch strings.ToLower(role) {
	case "user":
		roleLine = accentStyle.Render("▶ user")
	case "assistant":
		roleLine = titleStyle.Render("◀ assistant")
	default:
		roleLine = mutedStyle.Render(role)
	}
	return roleLine + "\n" + text + "\n\n"
}
