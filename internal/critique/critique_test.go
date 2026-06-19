package critique

import (
	"context"
	"testing"

	"github.com/pookNast/storm-cli/internal/types"
)

type mockChatter struct {
	resp string
	err  error
}

func (m *mockChatter) Chat(_ context.Context, _, _, _ string) (string, error) {
	return m.resp, m.err
}

var sampleBriefing = types.Briefing{
	Topic: "AI safety",
	Role:  "analyst",
	Perspectives: []types.PerspectiveResult{
		{Persona: "Practitioner", Position: "Works", Evidence: "e", Insight: "i"},
	},
	Findings: []types.Finding{
		{Title: "Finding 1", Detail: "detail", Confidence: 8, SupportingPersonas: []string{"Practitioner"}},
	},
	HiddenConnection: "connection",
	FrontierQuestion: "What next?",
}

const validJSON = `{
  "confidence_audit": "moderate confidence across findings",
  "dominant_bias": "Practitioner",
  "missing_perspectives": ["Regulator","End user"],
  "weakest_link": "Finding 1 lacks longitudinal evidence"
}`

func TestBuild_Valid(t *testing.T) {
	cl := &mockChatter{resp: validJSON}
	crit, err := Build(context.Background(), cl, "m", sampleBriefing)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	if crit.ConfidenceAudit == "" {
		t.Error("ConfidenceAudit should not be empty")
	}
	if crit.DominantBias == "" {
		t.Error("DominantBias should not be empty")
	}
	if len(crit.MissingPerspectives) == 0 {
		t.Error("MissingPerspectives should not be empty")
	}
	if crit.WeakestLink == "" {
		t.Error("WeakestLink should not be empty")
	}
}

func TestBuild_Fenced(t *testing.T) {
	fenced := "```json\n" + validJSON + "\n```"
	cl := &mockChatter{resp: fenced}
	_, err := Build(context.Background(), cl, "m", sampleBriefing)
	if err != nil {
		t.Fatalf("fenced JSON should succeed: %v", err)
	}
}

func TestBuild_Malformed(t *testing.T) {
	cl := &mockChatter{resp: "not json"}
	_, err := Build(context.Background(), cl, "m", sampleBriefing)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestBuild_EmptyContentRejected(t *testing.T) {
	// Valid JSON but all fields empty — must be rejected (would otherwise ship a
	// silently-hollow critique). After the repair retry returns the same empty
	// object, Build must return an error so the orchestrator degrades explicitly.
	empty := `{"confidence_audit":"","dominant_bias":"","missing_perspectives":null,"weakest_link":""}`
	cl := &mockChatter{resp: empty}
	_, err := Build(context.Background(), cl, "m", sampleBriefing)
	if err == nil {
		t.Fatal("expected error for empty critique content")
	}
}

func TestBuild_DominantBiasBalanced(t *testing.T) {
	balanced := `{"confidence_audit":"high","dominant_bias":"balanced","missing_perspectives":[],"weakest_link":"none"}`
	cl := &mockChatter{resp: balanced}
	crit, err := Build(context.Background(), cl, "m", sampleBriefing)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	if crit.DominantBias != "balanced" {
		t.Errorf("DominantBias = %q, want 'balanced'", crit.DominantBias)
	}
}
