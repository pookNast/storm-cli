// Package persona implements Phase 1: parallel fan-out to 5 locked expert personas.
package persona

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pookNast/storm-cli/internal/llm"
	"github.com/pookNast/storm-cli/internal/types"
	"golang.org/x/sync/errgroup"
)

// locked5 are the 5 immutable STORM personas.
var locked5 = []types.Persona{
	{
		Name: "Practitioner",
		SystemPrompt: "You are a seasoned practitioner with deep hands-on experience. " +
			"Focus on real-world implementation, operational realities, and practical tradeoffs.",
	},
	{
		Name: "Academic",
		SystemPrompt: "You are a rigorous academic researcher. " +
			"Focus on peer-reviewed evidence, theoretical frameworks, and methodological rigor.",
	},
	{
		Name: "Skeptic",
		SystemPrompt: "You are a critical skeptic. " +
			"Identify hidden assumptions, challenge consensus claims, and surface counter-evidence.",
	},
	{
		Name: "Economist",
		SystemPrompt: "You are an economist specializing in incentives and resource allocation. " +
			"Focus on economic forces, cost-benefit analysis, and second-order effects.",
	},
	{
		Name: "Historian",
		SystemPrompt: "You are a historian with expertise in long-run trends. " +
			"Focus on historical precedents, pattern recognition across eras, and path dependence.",
	},
}

// perspectiveJSON is the model-facing schema we instruct the LLM to emit.
type perspectiveJSON struct {
	Position string `json:"position"`
	Evidence string `json:"evidence"`
	Insight  string `json:"insight"`
}

const userPromptTmpl = `Topic: %s
Role context: %s

Respond ONLY with a valid JSON object matching this schema (no markdown fences, no commentary):
{"position":"<your core claim>","evidence":"<key supporting evidence>","insight":"<unique angle others may miss>"}`

// FanOut issues all 5 persona calls concurrently via errgroup.
// Results are returned in locked5 order. The first error cancels remaining calls.
func FanOut(ctx context.Context, cl llm.Chatter, model, topic, role string) ([]types.PerspectiveResult, error) {
	results := make([]types.PerspectiveResult, len(locked5))
	eg, egCtx := errgroup.WithContext(ctx)

	for i, p := range locked5 {
		i, p := i, p // capture loop vars
		eg.Go(func() error {
			userPrompt := fmt.Sprintf(userPromptTmpl, topic, role)
			raw, err := cl.Chat(egCtx, model, p.SystemPrompt, userPrompt)
			if err != nil {
				return fmt.Errorf("persona %s: %w", p.Name, err)
			}
			pj, err := parsePerspective(raw)
			if err != nil {
				// One repair attempt.
				raw2, err2 := cl.Chat(egCtx, model, p.SystemPrompt, userPrompt+llm.RepairSuffix)
				if err2 != nil {
					return fmt.Errorf("persona %s: %w", p.Name, err2)
				}
				pj2, err3 := parsePerspective(raw2)
				if err3 != nil {
					return fmt.Errorf("persona %s parse: %w", p.Name, err3)
				}
				pj = pj2
			}
			results[i] = types.PerspectiveResult{
				Persona:  p.Name,
				Position: pj.Position,
				Evidence: pj.Evidence,
				Insight:  pj.Insight,
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}

// Personas returns the locked 5 personas (exported for testing).
func Personas() []types.Persona {
	return locked5
}

func parsePerspective(raw string) (perspectiveJSON, error) {
	clean := llm.ExtractJSON(raw)
	var pj perspectiveJSON
	if err := json.Unmarshal([]byte(clean), &pj); err != nil {
		return perspectiveJSON{}, fmt.Errorf("parse perspective JSON: %w", err)
	}
	return pj, nil
}
