// Package critique implements Phase 4: self-critique of the assembled briefing.
package critique

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pookNast/storm-cli/internal/llm"
	"github.com/pookNast/storm-cli/internal/types"
)

const systemPrompt = `You are an epistemically rigorous self-critic. Your task is to review a research briefing and identify its weaknesses, biases, and gaps.`

const userPromptTmpl = `Review this research briefing on the topic "%s":

Perspectives analyzed: %s
Findings (count: %d):
%s
Hidden connection: %s
Frontier question: %s

Respond ONLY with a valid JSON object. Begin your reply with the character { and end with }. No markdown fences, no preamble, no commentary, no explanation:
{
  "confidence_audit": "<assessment of overall confidence and reliability of findings>",
  "dominant_bias": "<which persona or viewpoint dominated, or 'balanced' if equitable>",
  "missing_perspectives": ["<stakeholder or viewpoint not represented>"],
  "weakest_link": "<the finding or claim most likely to be wrong or overstated>"
}`

// Build issues one LLM call to critique the assembled briefing.
func Build(ctx context.Context, cl llm.Chatter, model string, b types.Briefing) (types.Critique, error) {
	personaNames := make([]string, len(b.Perspectives))
	for i, p := range b.Perspectives {
		personaNames[i] = p.Persona
	}

	findingSummary := formatFindings(b.Findings)
	userPrompt := fmt.Sprintf(userPromptTmpl,
		b.Topic,
		strings.Join(personaNames, ", "),
		len(b.Findings),
		findingSummary,
		b.HiddenConnection,
		b.FrontierQuestion,
	)

	// First attempt.
	crit, err := attempt(ctx, cl, model, userPrompt)
	if err == nil && !isEmpty(crit) {
		return crit, nil
	}

	// One repair attempt: models occasionally return prose, or a valid-but-empty
	// JSON skeleton. Re-ask with a stricter instruction.
	crit, rerr := attempt(ctx, cl, model, userPrompt+llm.RepairSuffix)
	if rerr != nil {
		return types.Critique{}, fmt.Errorf("critique: %w", rerr)
	}
	if isEmpty(crit) {
		return types.Critique{}, fmt.Errorf("critique: model returned empty critique after retry")
	}
	return crit, nil
}

// attempt issues one chat call and parses the result.
func attempt(ctx context.Context, cl llm.Chatter, model, userPrompt string) (types.Critique, error) {
	raw, err := cl.Chat(ctx, model, systemPrompt, userPrompt)
	if err != nil {
		return types.Critique{}, fmt.Errorf("chat: %w", err)
	}
	return parse(raw)
}

// isEmpty reports whether the model produced no usable critique content.
func isEmpty(c types.Critique) bool {
	return c.ConfidenceAudit == "" && c.DominantBias == "" &&
		c.WeakestLink == "" && len(c.MissingPerspectives) == 0
}

func parse(raw string) (types.Critique, error) {
	clean := llm.ExtractJSON(raw)
	var c types.Critique
	if err := json.Unmarshal([]byte(clean), &c); err != nil {
		return types.Critique{}, fmt.Errorf("parse critique JSON: %w", err)
	}
	return c, nil
}

func formatFindings(fs []types.Finding) string {
	var sb strings.Builder
	for i, f := range fs {
		fmt.Fprintf(&sb, "%d. [confidence %d] %s\n", i+1, f.Confidence, f.Title)
	}
	return sb.String()
}
