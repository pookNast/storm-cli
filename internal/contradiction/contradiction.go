// Package contradiction implements Phase 2: contradiction map extraction.
package contradiction

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pookNast/storm-cli/internal/llm"
	"github.com/pookNast/storm-cli/internal/types"
)

const systemPrompt = `You are a rigorous analytical synthesizer. Your task is to analyze multiple expert perspectives and identify contradictions, consensus, and blind spots.`

const userPromptTmpl = `You have the following expert perspectives on the topic:

%s

Analyze these perspectives and respond ONLY with a valid JSON object (no markdown fences, no commentary) matching this schema:
{
  "conflicts": [
    {
      "claim_a": "<first conflicting claim>",
      "claim_b": "<second conflicting claim>",
      "personas": ["<persona names involved>"],
      "evidence_weight": "<strongest|weakest|moderate>"
    }
  ],
  "consensus": ["<claim all personas agree on>"],
  "blind_spots": ["<topic or angle no persona addressed>"]
}`

// Build issues one LLM call to extract the ContradictionMap from perspective results.
// On a parse failure it retries once with repairSuffix appended.
func Build(ctx context.Context, cl llm.Chatter, model string, perspectives []types.PerspectiveResult) (types.ContradictionMap, error) {
	perspSummary := formatPerspectives(perspectives)
	userPrompt := fmt.Sprintf(userPromptTmpl, perspSummary)

	raw, err := cl.Chat(ctx, model, systemPrompt, userPrompt)
	if err != nil {
		return types.ContradictionMap{}, fmt.Errorf("contradiction: chat: %w", err)
	}

	cm, err := parse(raw)
	if err == nil {
		return cm, nil
	}

	// One repair attempt.
	raw2, err2 := cl.Chat(ctx, model, systemPrompt, userPrompt+llm.RepairSuffix)
	if err2 != nil {
		return types.ContradictionMap{}, fmt.Errorf("contradiction: chat: %w", err2)
	}
	cm2, err3 := parse(raw2)
	if err3 != nil {
		return types.ContradictionMap{}, fmt.Errorf("contradiction: %w", err3)
	}
	return cm2, nil
}

func parse(raw string) (types.ContradictionMap, error) {
	clean := llm.ExtractJSON(raw)
	var cm types.ContradictionMap
	if err := json.Unmarshal([]byte(clean), &cm); err != nil {
		return types.ContradictionMap{}, fmt.Errorf("parse contradiction JSON: %w", err)
	}
	return cm, nil
}

func formatPerspectives(ps []types.PerspectiveResult) string {
	var sb strings.Builder
	for _, p := range ps {
		fmt.Fprintf(&sb, "[%s]\nPosition: %s\nEvidence: %s\nInsight: %s\n\n",
			p.Persona, p.Position, p.Evidence, p.Insight)
	}
	return sb.String()
}
