package contradiction

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/pookNast/storm-cli/internal/types"
)

// mockChatter returns a fixed response.
type mockChatter struct {
	resp string
	err  error
}

func (m *mockChatter) Chat(_ context.Context, _, _, _ string) (string, error) {
	return m.resp, m.err
}

// repairMock returns prose on the first call and valid JSON on the second.
type repairMock struct {
	calls atomic.Int64
	valid string
}

func (m *repairMock) Chat(_ context.Context, _, _, _ string) (string, error) {
	n := m.calls.Add(1)
	if n == 1 {
		return "Here is my prose answer, not JSON.", nil
	}
	return m.valid, nil
}

var samplePerspectives = []types.PerspectiveResult{
	{Persona: "Practitioner", Position: "Works in practice", Evidence: "Case studies", Insight: "Ops matter"},
	{Persona: "Academic", Position: "Theory says otherwise", Evidence: "Papers", Insight: "Methodology gap"},
}

const validJSON = `{
  "conflicts": [{"claim_a":"Works in practice","claim_b":"Theory says otherwise","personas":["Practitioner","Academic"],"evidence_weight":"strongest"}],
  "consensus": ["Both agree more research is needed"],
  "blind_spots": ["Environmental impact"]
}`

func TestBuild_Valid(t *testing.T) {
	cl := &mockChatter{resp: validJSON}
	cm, err := Build(context.Background(), cl, "m", samplePerspectives)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	if len(cm.Conflicts) != 1 {
		t.Errorf("Conflicts count = %d, want 1", len(cm.Conflicts))
	}
	if len(cm.Consensus) != 1 {
		t.Errorf("Consensus count = %d, want 1", len(cm.Consensus))
	}
	if len(cm.BlindSpots) != 1 {
		t.Errorf("BlindSpots count = %d, want 1", len(cm.BlindSpots))
	}
	if cm.Conflicts[0].EvidenceWeight != "strongest" {
		t.Errorf("EvidenceWeight = %q", cm.Conflicts[0].EvidenceWeight)
	}
}

func TestBuild_Fenced(t *testing.T) {
	fenced := "```json\n" + validJSON + "\n```"
	cl := &mockChatter{resp: fenced}
	_, err := Build(context.Background(), cl, "m", samplePerspectives)
	if err != nil {
		t.Fatalf("Build() with fenced JSON error: %v", err)
	}
}

func TestBuild_Malformed(t *testing.T) {
	cl := &mockChatter{resp: "not json at all"}
	_, err := Build(context.Background(), cl, "m", samplePerspectives)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestBuild_EmptyResponse(t *testing.T) {
	cl := &mockChatter{resp: "{}"}
	cm, err := Build(context.Background(), cl, "m", samplePerspectives)
	if err != nil {
		t.Fatalf("empty JSON should not error: %v", err)
	}
	if cm.Conflicts == nil && cm.Consensus == nil && cm.BlindSpots == nil {
		// all nil slices is acceptable for empty response
	}
}

func TestBuild_RepairPath(t *testing.T) {
	mock := &repairMock{valid: validJSON}
	cm, err := Build(context.Background(), mock, "m", samplePerspectives)
	if err != nil {
		t.Fatalf("Build repair path error: %v", err)
	}
	if len(cm.Conflicts) != 1 {
		t.Errorf("Conflicts count = %d, want 1", len(cm.Conflicts))
	}
	if got := mock.calls.Load(); got != 2 {
		t.Errorf("Chat calls = %d, want 2 (1 prose + 1 repair)", got)
	}
}
