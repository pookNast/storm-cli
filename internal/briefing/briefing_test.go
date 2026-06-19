package briefing

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pookNast/storm-cli/internal/types"
)

func sampleBriefing() types.Briefing {
	return types.Briefing{
		Topic: "AI safety",
		Role:  "analyst",
		Perspectives: []types.PerspectiveResult{
			{Persona: "Practitioner", Position: "Works", Evidence: "cases", Insight: "ops"},
			{Persona: "Skeptic", Position: "Risky", Evidence: "failures", Insight: "bias"},
		},
		ContradictionMap: types.ContradictionMap{
			Conflicts: []types.Conflict{
				{ClaimA: "Works", ClaimB: "Risky", Personas: []string{"Practitioner", "Skeptic"}, EvidenceWeight: "strongest"},
			},
			Consensus:  []string{"More research needed"},
			BlindSpots: []string{"Long-term effects"},
		},
		Findings: []types.Finding{
			{Title: "F1", Detail: "detail1", Confidence: 9, SupportingPersonas: []string{"Practitioner"}},
			{Title: "F2", Detail: "detail2", Confidence: 6, SupportingPersonas: []string{"Skeptic"}},
		},
		HiddenConnection: "Both relate to trust",
		RoleAction:       "Prioritize audits",
		FrontierQuestion: "What is the real threshold?",
		Critique: types.Critique{
			ConfidenceAudit:     "moderate",
			DominantBias:        "Practitioner",
			MissingPerspectives: []string{"Regulator"},
			WeakestLink:         "F2",
		},
	}
}

func TestRenderJSON_Valid(t *testing.T) {
	b := sampleBriefing()
	data, err := RenderJSON(b)
	if err != nil {
		t.Fatalf("RenderJSON() error: %v", err)
	}
	// Must be valid JSON
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("RenderJSON() produced invalid JSON: %v", err)
	}
	// Must be indented
	if !strings.Contains(string(data), "\n") {
		t.Error("RenderJSON() should be indented")
	}
}

func TestRenderJSON_ContradictionMapPresent(t *testing.T) {
	data, err := RenderJSON(sampleBriefing())
	if err != nil {
		t.Fatalf("RenderJSON() error: %v", err)
	}
	var out map[string]any
	json.Unmarshal(data, &out) //nolint
	cm, ok := out["contradiction_map"].(map[string]any)
	if !ok {
		t.Fatal("contradiction_map missing from JSON")
	}
	for _, field := range []string{"conflicts", "consensus", "blind_spots"} {
		if _, ok := cm[field]; !ok {
			t.Errorf("contradiction_map missing field %q", field)
		}
	}
}

func TestRenderMarkdown_AllSections(t *testing.T) {
	md := RenderMarkdown(sampleBriefing())
	sections := []string{
		"## Expert Perspectives",
		"## Contradiction Map",
		"### Conflicts",
		"### Consensus",
		"### Blind Spots",
		"## Research Findings",
		"## Hidden Connection",
		"## Role-Specific Action",
		"## Frontier Question",
		"## Self-Critique",
	}
	for _, s := range sections {
		if !strings.Contains(md, s) {
			t.Errorf("Markdown missing section %q", s)
		}
	}
}

func TestRenderMarkdown_EmptyBriefing_NoPanic(t *testing.T) {
	// Must not panic on zero-value Briefing
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("RenderMarkdown panicked: %v", r)
		}
	}()
	RenderMarkdown(types.Briefing{})
}

func TestRenderMarkdown_TableCellSanitization(t *testing.T) {
	b := sampleBriefing()
	// Inject pipe and newline into finding title and supporting persona
	b.Findings[0].Title = "a|b"
	b.Findings[0].SupportingPersonas = []string{"x\ny"}
	md := RenderMarkdown(b)

	// Find the table rows (lines starting with "|")
	var tableRows []string
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "|") {
			tableRows = append(tableRows, line)
		}
	}
	if len(tableRows) == 0 {
		t.Fatal("no table rows found in markdown")
	}

	for _, row := range tableRows {
		// Each row should start and end with "|" and have exactly 4 columns (5 pipes)
		// Skip header/separator rows containing "---"
		if strings.Contains(row, "---") {
			continue
		}
		// Count delimiter pipes: trim leading/trailing pipe then split
		trimmed := strings.Trim(row, "|")
		cols := strings.Split(trimmed, "|")
		if len(cols) != 4 {
			t.Errorf("table row has wrong column count (%d), row: %q", len(cols), row)
		}
		// No raw newlines should appear mid-row
		if strings.ContainsAny(row, "\r") {
			t.Errorf("table row contains carriage return: %q", row)
		}
	}

	// The U+2223 replacement for "|" should appear in the output
	if !strings.Contains(md, "∣") {
		t.Error("expected U+2223 (∣) replacement for pipe in table cell, not found")
	}
}

func TestRenderMarkdown_FindingSortVisible(t *testing.T) {
	b := sampleBriefing()
	md := RenderMarkdown(b)
	// F1 (confidence 9) should appear before F2 (confidence 6) in the table
	idx1 := strings.Index(md, "F1")
	idx2 := strings.Index(md, "F2")
	if idx1 < 0 || idx2 < 0 {
		t.Skip("findings not found in markdown")
	}
	if idx1 > idx2 {
		t.Error("F1 (higher confidence) should appear before F2 in markdown")
	}
}
