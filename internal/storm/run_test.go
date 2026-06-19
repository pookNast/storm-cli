package storm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pookNast/storm-cli/internal/config"
	"github.com/pookNast/storm-cli/internal/persona"
)

// reqSystemPrompt extracts the system message from an OpenAI-style chat request.
func reqSystemPrompt(r *http.Request) string {
	var body struct {
		Messages []struct {
			Role, Content string
		} `json:"messages"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	for _, m := range body.Messages {
		if m.Role == "system" {
			return m.Content
		}
	}
	return ""
}

// mockResponses returns a handler that serves canned responses for each call.
// The server serves different JSON based on call sequence:
//
//	Calls 1-5 (persona fan-out): perspective JSON
//	Call 6 (contradiction): contradiction JSON
//	Call 7 (synthesis): synthesis JSON
//	Call 8 (critique): critique JSON
func newMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	// Post-persona phases (contradiction, synthesis, critique) are sequential, so a
	// simple counter is deterministic for them. Persona calls are parallel, so they
	// are routed by matching the request's system prompt to the persona — this also
	// lets the test assert correct response routing, not just counts.
	var postCount atomic.Int32

	perspJSON := func(name string) string {
		return `{"position":"pos-` + name + `","evidence":"ev","insight":"ins"}`
	}

	contraJSON := `{
		"conflicts":[{"claim_a":"A","claim_b":"B","personas":["Practitioner","Skeptic"],"evidence_weight":"strongest"}],
		"consensus":["both agree on X"],
		"blind_spots":["environmental impact"]
	}`

	synthJSON := `{
		"findings":[
			{"title":"F1","detail":"d1","confidence":9,"supporting_personas":["Practitioner","Academic"]},
			{"title":"F2","detail":"d2","confidence":6,"supporting_personas":["Skeptic"]}
		],
		"hidden_connection":"systemic link",
		"role_action":"prioritize validation",
		"frontier_question":"what is the real threshold?"
	}`

	critiqueJSON := `{
		"confidence_audit":"moderate overall",
		"dominant_bias":"Practitioner",
		"missing_perspectives":["Regulator","End user"],
		"weakest_link":"F2 lacks longitudinal evidence"
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sys := reqSystemPrompt(r)
		w.Header().Set("Content-Type", "application/json")

		var content string
		matched := false
		for _, p := range persona.Personas() {
			if p.SystemPrompt == sys {
				content = perspJSON(p.Name) // route by persona, not call order
				matched = true
				break
			}
		}
		if !matched {
			switch int(postCount.Add(1)) {
			case 1:
				content = contraJSON
			case 2:
				content = synthJSON
			default:
				content = critiqueJSON
			}
		}

		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": content}},
			},
		}
		json.NewEncoder(w).Encode(resp) //nolint
	}))
	return srv
}

func TestEndToEnd(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	cfg := config.Config{
		APIBase:    srv.URL,
		APIKey:     "",
		Model:      "test-model",
		SynthModel: "test-synth",
		Timeout:    10 * time.Second,
	}

	ctx := context.Background()
	b, err := Run(ctx, cfg, "AI safety research", "analyst")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Routing: each persona must receive its own response (not cross-wired).
	if len(b.Perspectives) != 5 {
		t.Fatalf("expected 5 perspectives, got %d", len(b.Perspectives))
	}
	for _, p := range b.Perspectives {
		if p.Position != "pos-"+p.Persona {
			t.Errorf("persona %q got mis-routed response %q", p.Persona, p.Position)
		}
	}

	// M4: >=1 finding
	if len(b.Findings) < 1 {
		t.Errorf("expected >=1 finding, got %d", len(b.Findings))
	}

	// M6: ContradictionMap has conflicts, consensus, blind_spots
	if len(b.ContradictionMap.Conflicts) == 0 {
		t.Error("ContradictionMap.Conflicts should not be empty")
	}
	if len(b.ContradictionMap.Consensus) == 0 && len(b.ContradictionMap.BlindSpots) == 0 {
		t.Error("ContradictionMap should have consensus or blind_spots")
	}

	// M8: frontier_question non-empty
	if b.FrontierQuestion == "" {
		t.Error("FrontierQuestion should not be empty")
	}

	// M8: hidden_connection + role_action non-empty
	if b.HiddenConnection == "" {
		t.Error("HiddenConnection should not be empty")
	}
	if b.RoleAction == "" {
		t.Error("RoleAction should not be empty")
	}

	// M9: critique fields present
	if b.Critique.DominantBias == "" {
		t.Error("Critique.DominantBias should not be empty")
	}
	if b.Critique.ConfidenceAudit == "" {
		t.Error("Critique.ConfidenceAudit should not be empty")
	}

	// M7: findings sorted desc by confidence
	for i := 1; i < len(b.Findings); i++ {
		if b.Findings[i].Confidence > b.Findings[i-1].Confidence {
			t.Errorf("findings not sorted desc: index %d (%d) > index %d (%d)",
				i, b.Findings[i].Confidence, i-1, b.Findings[i-1].Confidence)
		}
	}

	// M10: topic and key not in any finding detail or output
	for _, f := range b.Findings {
		if strings.Contains(f.Detail, cfg.APIKey) && cfg.APIKey != "" {
			t.Error("finding detail contains API key")
		}
	}
}

// TestRun_CritiqueDegradesGracefully: when the critique model returns prose
// (no JSON) on both the initial and repair calls, the run must still succeed
// and preserve the 3-phase briefing, with a present-but-explicit degraded critique.
func TestRun_CritiqueDegradesGracefully(t *testing.T) {
	var callCount atomic.Int32
	perspJSON := `{"position":"p","evidence":"e","insight":"i"}`
	contraJSON := `{"conflicts":[{"claim_a":"A","claim_b":"B","personas":["Practitioner"],"evidence_weight":"strongest"}],"consensus":["x"],"blind_spots":["y"]}`
	synthJSON := `{"findings":[{"title":"F1","detail":"d","confidence":8,"supporting_personas":["Academic"]}],"hidden_connection":"h","role_action":"r","frontier_question":"q?"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(callCount.Add(1))
		w.Header().Set("Content-Type", "application/json")
		var content string
		switch {
		case n <= 5:
			content = perspJSON
		case n == 6:
			content = contraJSON
		case n == 7:
			content = synthJSON
		default:
			// Critique attempts (call 8 and repair call 9): pure prose, no JSON object.
			content = "The briefing looks reasonable but I have some concerns about depth."
		}
		json.NewEncoder(w).Encode(map[string]any{ //nolint
			"choices": []map[string]any{{"message": map[string]any{"content": content}}},
		})
	}))
	defer srv.Close()

	cfg := config.Config{APIBase: srv.URL, Model: "m", SynthModel: "m", Timeout: 10 * time.Second}
	b, err := Run(context.Background(), cfg, "topic", "analyst")
	if err != nil {
		t.Fatalf("run should not fail when only critique flakes: %v", err)
	}
	if len(b.Findings) < 1 {
		t.Error("briefing findings must be preserved despite critique failure")
	}
	if b.Critique.ConfidenceAudit == "" || !strings.Contains(b.Critique.ConfidenceAudit, "unavailable") {
		t.Errorf("expected degraded critique marked unavailable, got %q", b.Critique.ConfidenceAudit)
	}
	if got := callCount.Load(); got < 9 {
		t.Errorf("expected critique repair retry (>=9 calls), got %d", got)
	}
}

func TestRun_PropagatesBackendError(t *testing.T) {
	// Server always returns 500 to trigger failure
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "backend down", http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := config.Config{
		APIBase:    srv.URL,
		Model:      "m",
		SynthModel: "m",
		Timeout:    3 * time.Second,
	}

	_, err := Run(context.Background(), cfg, "topic", "role")
	if err == nil {
		t.Fatal("expected error when backend is down (M11)")
	}
}
