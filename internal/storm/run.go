// Package storm is the orchestrator: it runs the 4 STORM phases in order.
package storm

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/pookNast/storm-cli/internal/briefing"
	"github.com/pookNast/storm-cli/internal/client"
	"github.com/pookNast/storm-cli/internal/config"
	"github.com/pookNast/storm-cli/internal/contradiction"
	"github.com/pookNast/storm-cli/internal/critique"
	"github.com/pookNast/storm-cli/internal/obs"
	"github.com/pookNast/storm-cli/internal/persona"
	"github.com/pookNast/storm-cli/internal/synthesis"
	"github.com/pookNast/storm-cli/internal/types"
)

// Run executes the 4-phase STORM loop and returns the assembled Briefing.
// Errors from any phase are propagated immediately (non-zero exit upstream).
func Run(ctx context.Context, cfg config.Config, topic, role string) (types.Briefing, error) {
	cl := client.New(cfg.APIBase, cfg.APIKey, cfg.Timeout)
	topicHash := obs.TopicHash(topic)

	// Phase 1 — Perspective Engine (parallel fan-out)
	t0 := time.Now()
	slog.InfoContext(ctx, "phase1_start", "phase", "perspective", "topic_hash", topicHash, "role", role, "model", cfg.Model)
	perspectives, err := persona.FanOut(ctx, cl, cfg.Model, topic, role)
	if err != nil {
		return types.Briefing{}, fmt.Errorf("phase1 perspective: %w", err)
	}
	slog.InfoContext(ctx, "phase1_done", "phase", "perspective", "personas", len(perspectives), "ms", time.Since(t0).Milliseconds())

	// Phase 2 — Contradiction Map
	t1 := time.Now()
	slog.InfoContext(ctx, "phase2_start", "phase", "contradiction")
	cm, err := contradiction.Build(ctx, cl, cfg.Model, perspectives)
	if err != nil {
		return types.Briefing{}, fmt.Errorf("phase2 contradiction: %w", err)
	}
	slog.InfoContext(ctx, "phase2_done", "phase", "contradiction",
		"conflicts", len(cm.Conflicts), "consensus", len(cm.Consensus), "blind_spots", len(cm.BlindSpots),
		"ms", time.Since(t1).Milliseconds())

	// Phase 3 — Scored Synthesis
	t2 := time.Now()
	slog.InfoContext(ctx, "phase3_start", "phase", "synthesis", "model", cfg.SynthModel)
	synthResult, err := synthesis.Build(ctx, cl, cfg.SynthModel, topic, role, perspectives, cm)
	if err != nil {
		return types.Briefing{}, fmt.Errorf("phase3 synthesis: %w", err)
	}
	slog.InfoContext(ctx, "phase3_done", "phase", "synthesis", "findings", len(synthResult.Findings), "ms", time.Since(t2).Milliseconds())

	// Assemble partial briefing for Phase 4 input
	partial := types.Briefing{
		Topic:            topic,
		Role:             role,
		Perspectives:     perspectives,
		ContradictionMap: cm,
		Findings:         synthResult.Findings,
		HiddenConnection: synthResult.HiddenConnection,
		RoleAction:       synthResult.RoleAction,
		FrontierQuestion: synthResult.FrontierQuestion,
	}

	// Phase 4 — Self-Critique
	t3 := time.Now()
	slog.InfoContext(ctx, "phase4_start", "phase", "critique")
	crit, err := critique.Build(ctx, cl, cfg.Model, partial)
	if err != nil {
		// Resilience: a flaky self-critique must never discard a good 3-phase briefing.
		// Degrade to a present-but-explicit critique and log loudly (not a silent failure).
		slog.WarnContext(ctx, "phase4_degraded", "phase", "critique",
			"err", err.Error(), "ms", time.Since(t3).Milliseconds())
		crit = types.Critique{
			ConfidenceAudit:     "unavailable — self-critique model returned unparseable output after retry; treat findings with extra caution",
			DominantBias:        "unknown",
			MissingPerspectives: []string{},
			WeakestLink:         "unknown — critique unavailable",
		}
	} else {
		slog.InfoContext(ctx, "phase4_done", "phase", "critique", "ms", time.Since(t3).Milliseconds())
	}

	b := partial
	b.Critique = crit

	// Validate briefing renders without error
	if _, err := briefing.RenderJSON(b); err != nil {
		return types.Briefing{}, fmt.Errorf("briefing render check: %w", err)
	}

	return b, nil
}
