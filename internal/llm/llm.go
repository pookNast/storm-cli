// Package llm provides shared LLM primitives used across STORM phases.
package llm

import (
	"context"
	"strings"
)

// Chatter is the interface satisfied by *client.Client.
type Chatter interface {
	Chat(ctx context.Context, model, system, user string) (string, error)
}

// RepairSuffix is appended to a user prompt when the model's first reply was not valid JSON.
const RepairSuffix = "\n\nYour previous reply was not valid JSON. Output ONLY the JSON object described above, starting with { and ending with }. Nothing else."

// ExtractJSON removes optional ```json ... ``` code fences from LLM output and
// extracts the outermost JSON object if the model wrapped it in prose.
func ExtractJSON(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// drop first line (```json or ```)
		idx := strings.Index(s, "\n")
		if idx >= 0 {
			s = s[idx+1:]
		}
		// drop trailing ```
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
		s = strings.TrimSpace(s)
	}
	// Extract the outermost JSON object if the model wrapped it in prose.
	if i, j := strings.Index(s, "{"), strings.LastIndex(s, "}"); i >= 0 && j > i {
		s = s[i : j+1]
	}
	return s
}
