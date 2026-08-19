package agentloop

import (
	"strings"
	"testing"
)

func TestNeutralSemanticPromptAdmitsGenericL3ImprovementProposal(t *testing.T) {
	prompt := messageLoopNeutralFamilySystemPrompt(&runState{input: Input{Context: map[string]any{}}})
	for _, fragment := range []string{
		`"status":"needs_experiment"`,
		`"schema_version":"improvement_proposal.v1"`,
		"Do not force a unique processor family",
		"track/clip gain, pan, EQ",
		`"track.band_dynamics"`,
		"never prefix or namespace it with a track ID",
	} {
		if !strings.Contains(prompt, fragment) {
			t.Fatalf("semantic prompt missing L3 improvement contract %q", fragment)
		}
	}
}
