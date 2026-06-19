package synthesis

import (
	"context"
	"sync/atomic"
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
	{Persona: "Practitioner", Position: "X works", Evidence: "e", Insight: "i"},
	{Persona: "Skeptic", Position: "X fails", Evidence: "e2", Insight: "i2"},
}

var sampleCM = types.ContradictionMap{
	Conflicts:  []types.Conflict{{ClaimA: "X works", ClaimB: "X fails", Personas: []string{"Practitioner", "Skeptic"}, EvidenceWeight: "strongest"}},
	Consensus:  []string{"More study needed"},
	BlindSpots: []string{"Cost analysis"},
}

const validJSON = `{
  "findings": [
    {"title":"F1","detail":"d1","confidence":9,"supporting_personas":["Practitioner"]},
    {"title":"F2","detail":"d2","confidence":5,"supporting_personas":["Skeptic"]},
    {"title":"F3","detail":"d3","confidence":7,"supporting_personas":["Academic"]}
  ],
  "hidden_connection": "both point to systemic issue",
  "role_action": "focus on validation",
  "frontier_question": "what is the real cost?"
}`

func TestBuild_Valid(t *testing.T) {
	cl := &mockChatter{resp: validJSON}
	res, err := Build(context.Background(), cl, "m", "topic", "analyst", samplePerspectives, sampleCM)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	if len(res.Findings) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(res.Findings))
	}
	if res.HiddenConnection == "" {
		t.Error("HiddenConnection should not be empty")
	}
	if res.RoleAction == "" {
		t.Error("RoleAction should not be empty")
	}
	if res.FrontierQuestion == "" {
		t.Error("FrontierQuestion should not be empty")
	}
}

func TestBuild_SortedDescByConfidence(t *testing.T) {
	cl := &mockChatter{resp: validJSON}
	res, err := Build(context.Background(), cl, "m", "topic", "analyst", samplePerspectives, sampleCM)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	for i := 1; i < len(res.Findings); i++ {
		if res.Findings[i].Confidence > res.Findings[i-1].Confidence {
			t.Errorf("findings not sorted desc: [%d].Confidence=%d > [%d].Confidence=%d",
				i, res.Findings[i].Confidence, i-1, res.Findings[i-1].Confidence)
		}
	}
}

func TestBuild_ConfidenceClamp(t *testing.T) {
	badJSON := `{
    "findings": [
      {"title":"lo","detail":"d","confidence":0,"supporting_personas":[]},
      {"title":"hi","detail":"d","confidence":15,"supporting_personas":[]}
    ],
    "hidden_connection":"h","role_action":"r","frontier_question":"q"
  }`
	cl := &mockChatter{resp: badJSON}
	res, err := Build(context.Background(), cl, "m", "topic", "r", samplePerspectives, sampleCM)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	for _, f := range res.Findings {
		if f.Confidence < 1 || f.Confidence > 10 {
			t.Errorf("confidence %d out of [1,10] range", f.Confidence)
		}
	}
}

func TestBuild_FloatConfidence(t *testing.T) {
	// Models sometimes emit a float confidence (e.g. 7.5). It must parse and
	// truncate, not abort the whole run.
	floatJSON := `{
    "findings": [
      {"title":"f","detail":"d","confidence":7.5,"supporting_personas":[]}
    ],
    "hidden_connection":"h","role_action":"r","frontier_question":"q"
  }`
	cl := &mockChatter{resp: floatJSON}
	res, err := Build(context.Background(), cl, "m", "topic", "r", samplePerspectives, sampleCM)
	if err != nil {
		t.Fatalf("float confidence should parse, got error: %v", err)
	}
	if len(res.Findings) != 1 || res.Findings[0].Confidence != 7 {
		t.Errorf("expected truncated confidence 7, got %v", res.Findings)
	}
}

func TestBuild_Malformed(t *testing.T) {
	cl := &mockChatter{resp: "not json"}
	_, err := Build(context.Background(), cl, "m", "topic", "r", samplePerspectives, sampleCM)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestBuild_RepairPath(t *testing.T) {
	mock := &repairMock{valid: validJSON}
	res, err := Build(context.Background(), mock, "m", "topic", "analyst", samplePerspectives, sampleCM)
	if err != nil {
		t.Fatalf("Build repair path error: %v", err)
	}
	if len(res.Findings) != 3 {
		t.Errorf("Findings count = %d, want 3", len(res.Findings))
	}
	if got := mock.calls.Load(); got != 2 {
		t.Errorf("Chat calls = %d, want 2 (1 prose + 1 repair)", got)
	}
}

func TestBuild_Fenced(t *testing.T) {
	fenced := "```json\n" + validJSON + "\n```"
	cl := &mockChatter{resp: fenced}
	_, err := Build(context.Background(), cl, "m", "topic", "r", samplePerspectives, sampleCM)
	if err != nil {
		t.Fatalf("fenced JSON should succeed: %v", err)
	}
}
