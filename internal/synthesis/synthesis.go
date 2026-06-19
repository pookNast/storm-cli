// Package synthesis implements Phase 3: scored synthesis of research findings.
package synthesis

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/pookNast/storm-cli/internal/llm"
	"github.com/pookNast/storm-cli/internal/types"
)

const systemPrompt = `You are an expert research synthesizer. Your task is to synthesize multiple expert perspectives and contradiction analysis into scored findings.`

const userPromptTmpl = `Topic: %s
Role context: %s

Expert perspectives:
%s

Contradiction analysis:
Conflicts: %s
Consensus: %s
Blind spots: %s

Synthesize these into findings. Each finding gets a confidence score 1-10 based on cross-perspective support.
Respond ONLY with a valid JSON object (no markdown fences, no commentary):
{
  "findings": [
    {
      "title": "<concise finding title>",
      "detail": "<detailed explanation>",
      "confidence": <integer 1-10>,
      "supporting_personas": ["<persona names that support this>"]
    }
  ],
  "hidden_connection": "<a non-obvious link across findings>",
  "role_action": "<specific action recommended for the %s role>",
  "frontier_question": "<the most important open question this research raises>"
}`

// synthesisJSON is the raw model output structure.
type synthesisJSON struct {
	Findings         []rawFinding `json:"findings"`
	HiddenConnection string       `json:"hidden_connection"`
	RoleAction       string       `json:"role_action"`
	FrontierQuestion string       `json:"frontier_question"`
}

type rawFinding struct {
	Title              string   `json:"title"`
	Detail             string   `json:"detail"`
	Confidence         float64  `json:"confidence"` // float64: models sometimes emit 7.5; truncated in toFindings
	SupportingPersonas []string `json:"supporting_personas"`
}

// Result holds the outputs of Phase 3.
type Result struct {
	Findings         []types.Finding
	HiddenConnection string
	RoleAction       string
	FrontierQuestion string
}

// Build issues one LLM call for Phase 3 synthesis and returns sorted findings.
// Findings are sorted descending by Confidence. Confidence is clamped to [1,10].
func Build(ctx context.Context, cl llm.Chatter, model, topic, role string,
	perspectives []types.PerspectiveResult, cm types.ContradictionMap) (Result, error) {

	userPrompt := fmt.Sprintf(userPromptTmpl,
		topic, role,
		formatPerspectives(perspectives),
		formatSlice(conflictSummary(cm.Conflicts)),
		formatSlice(cm.Consensus),
		formatSlice(cm.BlindSpots),
		role,
	)

	raw, err := cl.Chat(ctx, model, systemPrompt, userPrompt)
	if err != nil {
		return Result{}, fmt.Errorf("synthesis: chat: %w", err)
	}

	sj, err := parse(raw)
	if err != nil {
		// One repair attempt.
		raw2, err2 := cl.Chat(ctx, model, systemPrompt, userPrompt+llm.RepairSuffix)
		if err2 != nil {
			return Result{}, fmt.Errorf("synthesis: chat: %w", err2)
		}
		sj2, err3 := parse(raw2)
		if err3 != nil {
			return Result{}, fmt.Errorf("synthesis: %w", err3)
		}
		sj = sj2
	}

	findings := toFindings(sj.Findings)
	sort.SliceStable(findings, func(i, j int) bool {
		return findings[i].Confidence > findings[j].Confidence
	})

	return Result{
		Findings:         findings,
		HiddenConnection: sj.HiddenConnection,
		RoleAction:       sj.RoleAction,
		FrontierQuestion: sj.FrontierQuestion,
	}, nil
}

func parse(raw string) (synthesisJSON, error) {
	clean := llm.ExtractJSON(raw)
	var sj synthesisJSON
	if err := json.Unmarshal([]byte(clean), &sj); err != nil {
		return synthesisJSON{}, fmt.Errorf("parse synthesis JSON: %w", err)
	}
	return sj, nil
}

func toFindings(rfs []rawFinding) []types.Finding {
	out := make([]types.Finding, len(rfs))
	for i, rf := range rfs {
		c := int(rf.Confidence) // truncate any float the model emitted
		if c < 1 {
			c = 1
		}
		if c > 10 {
			c = 10
		}
		out[i] = types.Finding{
			Title:              rf.Title,
			Detail:             rf.Detail,
			Confidence:         c,
			SupportingPersonas: rf.SupportingPersonas,
		}
	}
	return out
}

func formatPerspectives(ps []types.PerspectiveResult) string {
	var sb strings.Builder
	for _, p := range ps {
		fmt.Fprintf(&sb, "[%s] %s\n", p.Persona, p.Position)
	}
	return sb.String()
}

func conflictSummary(cs []types.Conflict) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = fmt.Sprintf("%s vs %s", c.ClaimA, c.ClaimB)
	}
	return out
}

func formatSlice(ss []string) string {
	if len(ss) == 0 {
		return "(none)"
	}
	return strings.Join(ss, "; ")
}
