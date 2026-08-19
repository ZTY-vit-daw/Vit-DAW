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

func TestFreeStatePromptCarriesExperimentEvaluationContract(t *testing.T) {
	prompt := messageLoopNeutralFamilySystemPrompt(&runState{input: Input{Context: map[string]any{}}})
	for _, fragment := range []string{
		`"experiment_materiality"`, `"experiment_target_response"`, `"experiment_round_decision"`,
		"subthreshold state MUST use evaluation=insufficient_dose",
		"Target response requires a fresh post-action CCB observation",
	} {
		if !strings.Contains(prompt, fragment) {
			t.Fatalf("experiment prompt missing %q", fragment)
		}
	}
}
