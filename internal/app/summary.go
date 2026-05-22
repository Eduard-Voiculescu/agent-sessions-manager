package app

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"time"
)

// recentAssistantCount is how many trailing assistant turns we keep
// for display in the detail pane.
const recentAssistantCount = 5

// transcriptSummary is the cached, lightweight digest of a session's JSONL
// shown in the detail pane. It's recomputed when the file's mtime advances.
type transcriptSummary struct {
	mtime             time.Time
	bytes             int64
	userTurns         int
	assistantTurns    int
	recentAssistants  []string  // chronological — oldest first, newest last
	lastAssistantAt   time.Time
}

// summarize parses path and returns turn counts plus the last assistant turn.
// Meta entries are ignored. Both old (type=message) and new (type=user|assistant)
// formats are handled.
func summarize(path string) (transcriptSummary, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return transcriptSummary{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return transcriptSummary{}, err
	}
	defer f.Close()

	sum := transcriptSummary{mtime: fi.ModTime(), bytes: fi.Size()}

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
		// Detail pane is constrained — always summarize, never full.
		role, text := extractMessage(entry.Message, false)
		if role == "" {
			role = entry.Type
		}
		if text == "" {
			continue
		}
		switch strings.ToLower(role) {
		case "user":
			sum.userTurns++
		case "assistant":
			sum.assistantTurns++
			sum.recentAssistants = append(sum.recentAssistants, text)
			if len(sum.recentAssistants) > recentAssistantCount {
				sum.recentAssistants = sum.recentAssistants[len(sum.recentAssistants)-recentAssistantCount:]
			}
			sum.lastAssistantAt = fi.ModTime()
		}
	}
	return sum, nil
}
