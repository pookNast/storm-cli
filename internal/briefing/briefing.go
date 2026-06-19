// Package briefing provides pure rendering functions for Briefing artifacts.
package briefing

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pookNast/storm-cli/internal/types"
)

// RenderJSON returns indented JSON for the briefing. Pure function, no network.
func RenderJSON(b types.Briefing) ([]byte, error) {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("briefing: marshal JSON: %w", err)
	}
	return data, nil
}

// RenderMarkdown renders the briefing as a structured Markdown document.
// All sections including the Contradiction Map are present as top-level sections.
// Pure function, no panics on empty fields.
func RenderMarkdown(b types.Briefing) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "# STORM Research Briefing\n\n")
	fmt.Fprintf(&sb, "**Topic:** %s\n", safe(b.Topic))
	fmt.Fprintf(&sb, "**Role:** %s\n\n", safe(b.Role))
	sb.WriteString("> ⚠️ AI-generated. Evidence and citations are produced by a language model and may be inaccurate or fabricated — verify independently before relying on this for decisions.\n\n")

	// Perspectives
	sb.WriteString("## Expert Perspectives\n\n")
	if len(b.Perspectives) == 0 {
		sb.WriteString("_No perspectives available._\n\n")
	} else {
		for _, p := range b.Perspectives {
			fmt.Fprintf(&sb, "### %s\n\n", safe(p.Persona))
			fmt.Fprintf(&sb, "**Position:** %s\n\n", safe(p.Position))
			fmt.Fprintf(&sb, "**Evidence:** %s\n\n", safe(p.Evidence))
			fmt.Fprintf(&sb, "**Unique Insight:** %s\n\n", safe(p.Insight))
		}
	}

	// Contradiction Map (first-class section — never collapsed into synthesis)
	sb.WriteString("## Contradiction Map\n\n")

	sb.WriteString("### Conflicts\n\n")
	if len(b.ContradictionMap.Conflicts) == 0 {
		sb.WriteString("_No conflicts identified._\n\n")
	} else {
		for i, c := range b.ContradictionMap.Conflicts {
			fmt.Fprintf(&sb, "**Conflict %d** (evidence weight: %s)\n\n", i+1, safe(c.EvidenceWeight))
			fmt.Fprintf(&sb, "- Claim A: %s\n", safe(c.ClaimA))
			fmt.Fprintf(&sb, "- Claim B: %s\n", safe(c.ClaimB))
			fmt.Fprintf(&sb, "- Personas: %s\n\n", strings.Join(c.Personas, ", "))
		}
	}

	sb.WriteString("### Consensus\n\n")
	if len(b.ContradictionMap.Consensus) == 0 {
		sb.WriteString("_No consensus claims identified._\n\n")
	} else {
		for _, cs := range b.ContradictionMap.Consensus {
			fmt.Fprintf(&sb, "- %s\n", safe(cs))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("### Blind Spots\n\n")
	if len(b.ContradictionMap.BlindSpots) == 0 {
		sb.WriteString("_No blind spots identified._\n\n")
	} else {
		for _, bs := range b.ContradictionMap.BlindSpots {
			fmt.Fprintf(&sb, "- %s\n", safe(bs))
		}
		sb.WriteString("\n")
	}

	// Scored Findings
	sb.WriteString("## Research Findings\n\n")
	if len(b.Findings) == 0 {
		sb.WriteString("_No findings available._\n\n")
	} else {
		sb.WriteString("| # | Confidence | Title | Supporting Personas |\n")
		sb.WriteString("|---|-----------|-------|--------------------|\n")
		for i, f := range b.Findings {
			fmt.Fprintf(&sb, "| %d | %d/10 | %s | %s |\n",
				i+1, f.Confidence, sanitizeCell(safe(f.Title)), sanitizeCell(strings.Join(f.SupportingPersonas, ", ")))
		}
		sb.WriteString("\n")
		for i, f := range b.Findings {
			fmt.Fprintf(&sb, "### Finding %d: %s\n\n", i+1, safe(f.Title))
			fmt.Fprintf(&sb, "%s\n\n", safe(f.Detail))
		}
	}

	// Synthesis outputs
	sb.WriteString("## Hidden Connection\n\n")
	fmt.Fprintf(&sb, "%s\n\n", safe(b.HiddenConnection))

	sb.WriteString("## Role-Specific Action\n\n")
	fmt.Fprintf(&sb, "%s\n\n", safe(b.RoleAction))

	sb.WriteString("## Frontier Question\n\n")
	fmt.Fprintf(&sb, "%s\n\n", safe(b.FrontierQuestion))

	// Critique
	sb.WriteString("## Self-Critique\n\n")
	fmt.Fprintf(&sb, "**Confidence Audit:** %s\n\n", safe(b.Critique.ConfidenceAudit))
	fmt.Fprintf(&sb, "**Dominant Bias:** %s\n\n", safe(b.Critique.DominantBias))

	sb.WriteString("**Missing Perspectives:**\n\n")
	if len(b.Critique.MissingPerspectives) == 0 {
		sb.WriteString("_None identified._\n\n")
	} else {
		for _, mp := range b.Critique.MissingPerspectives {
			fmt.Fprintf(&sb, "- %s\n", safe(mp))
		}
		sb.WriteString("\n")
	}

	fmt.Fprintf(&sb, "**Weakest Link:** %s\n\n", safe(b.Critique.WeakestLink))

	return sb.String()
}

// safe returns the string or a placeholder for empty fields (no panics).
func safe(s string) string {
	if s == "" {
		return "_not provided_"
	}
	return s
}

// sanitizeCell makes a string safe for use in a Markdown table cell by
// replacing pipe characters with U+2223 (∣) and stripping newlines.
func sanitizeCell(s string) string {
	s = strings.NewReplacer("\n", " ", "\r", " ").Replace(s)
	s = strings.ReplaceAll(s, "|", "∣")
	return s
}
