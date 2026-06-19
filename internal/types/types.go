// Package types defines the domain types for the STORM research CLI.
package types

// Persona represents one of the 5 locked expert perspectives.
type Persona struct {
	Name         string `json:"name"`
	SystemPrompt string `json:"system_prompt"`
}

// PerspectiveResult holds structured output from a single persona's analysis.
type PerspectiveResult struct {
	Persona  string `json:"persona"`
	Position string `json:"position"`
	Evidence string `json:"evidence"`
	Insight  string `json:"insight"`
}

// Conflict represents a pair of contradicting claims across personas.
type Conflict struct {
	ClaimA         string   `json:"claim_a"`
	ClaimB         string   `json:"claim_b"`
	Personas       []string `json:"personas"`
	EvidenceWeight string   `json:"evidence_weight"` // e.g. "strongest" / "weakest"
}

// ContradictionMap is a first-class output artifact — never collapsed into synthesis.
type ContradictionMap struct {
	Conflicts  []Conflict `json:"conflicts"`
	Consensus  []string   `json:"consensus"`
	BlindSpots []string   `json:"blind_spots"`
}

// Finding is a scored research insight (Confidence 1–10, sorted desc by cross-perspective support).
type Finding struct {
	Title              string   `json:"title"`
	Detail             string   `json:"detail"`
	Confidence         int      `json:"confidence"` // 1–10
	SupportingPersonas []string `json:"supporting_personas"`
}

// Critique is Phase 4 self-review output.
type Critique struct {
	ConfidenceAudit     string   `json:"confidence_audit"`
	DominantBias        string   `json:"dominant_bias"`
	MissingPerspectives []string `json:"missing_perspectives"`
	WeakestLink         string   `json:"weakest_link"`
}

// Briefing is the final assembled research output.
type Briefing struct {
	Topic            string              `json:"topic"`
	Role             string              `json:"role"`
	Perspectives     []PerspectiveResult `json:"perspectives"`
	ContradictionMap ContradictionMap    `json:"contradiction_map"`
	Findings         []Finding           `json:"findings"`
	HiddenConnection string              `json:"hidden_connection"`
	RoleAction       string              `json:"role_action"`
	FrontierQuestion string              `json:"frontier_question"`
	Critique         Critique            `json:"critique"`
}
