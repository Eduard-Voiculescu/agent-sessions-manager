package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// renderTranscript reads a Claude Code JSONL file and turns it into a
// readable plain-text rendering suitable for a viewport.
//
// Lightweight parse: we only know about a few "type" values. Anything we
// don't recognize is rendered as a dim "[meta:...]" line so the user still
// sees something happened there.
func renderTranscript(path string) (string, error) {
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
			b.WriteString(mutedStyle.Render("[unparseable line]") + "\n")
			continue
		}

		switch entry.Type {
		case "user", "assistant", "message":
			// Older transcripts use type="message" with the role nested in
			// message.role; newer ones use type="user"/"assistant" directly.
			role, text := extractMessage(entry.Message)
			if role == "" {
				role = entry.Type
			}
			if text == "" {
				continue
			}
			b.WriteString(formatTurn(role, text))
		default:
			// Skip meta entries — they're noise in a transcript view.
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return b.String(), nil
}

func extractMessage(raw json.RawMessage) (role, text string) {
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
	return msg.Role, contentText(msg.Content)
}

// contentText collapses Claude's content (either a string or an array of
// content blocks) into a single readable string. Tool-use blocks are summarized.
func contentText(raw json.RawMessage) string {
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
		Type  string          `json:"type"`
		Text  string          `json:"text"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var parts []string
		for _, blk := range blocks {
			switch blk.Type {
			case "text":
				parts = append(parts, blk.Text)
			case "tool_use":
				parts = append(parts, fmt.Sprintf("[tool: %s]", blk.Name))
			case "tool_result":
				parts = append(parts, "[tool result]")
			default:
				if blk.Type != "" {
					parts = append(parts, "["+blk.Type+"]")
				}
			}
		}
		return strings.Join(parts, "\n")
	}
	return string(raw)
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
