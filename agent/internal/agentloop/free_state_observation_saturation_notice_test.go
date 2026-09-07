package agentloop

import (
	"strings"
	"testing"
)

// DIAG3-3: the observation-saturation notice is the mechanical message
// channel that tells the model face "per-track observation duty is closed;
// the queue is still open; the hypothesis frontier is empty; here is the
// budget". It is runtime state disclosure in the same family as the
// continuation-budget directive — never domain guidance. The whitelist below
// pins the discipline: dimension names, coverage status, frontier size, and
// budget facts are allowed; any track identity, processor domain name,
// processor family, or view-steering verb is a failure.

func saturationNoticeState() *runState {
	return &runState{input: Input{
		Context: map[string]any{
			"task_contract": map[string]any{"kind": "improvement"},
			"free_state_reasoning_loop": map[string]any{
				"schema_version": "free_state_reasoning_loop.v1",
				"observation_saturation_notice": map[string]any{
					"schema_version":      "free_state_observation_saturation_notice.v1",
					"coverage_status":     "per_track_primary_complete",
					"open_dimensions":     []any{"level_headroom", "frequency_occupancy", "dynamics", "stereo_space", "transient_event"},
					"frontier_candidates": 0,
					"continuation_used":   7,
					"continuation_budget": 11,
				},
			},
		},
	}}
}

func TestObservationSaturationNoticeRendersMechanicalRuntimeFacts(t *testing.T) {
	directive := messageLoopFreeStateObservationSaturationDirective(saturationNoticeState())
	if directive == "" {
		t.Fatal("saturation notice directive must render when the notice is present")
	}
	for _, fragment := range []string{
		"per_track_primary_complete",
		"level_headroom", "dynamics", "transient_event",
		"frontier_candidates=0",
		"7/11",
	} {
		if !strings.Contains(directive, fragment) {
			t.Fatalf("saturation notice missing mechanical fact %q in %q", fragment, directive)
		}
	}
}

func TestObservationSaturationNoticeWhitelistMechanicalFactsOnly(t *testing.T) {
	directive := messageLoopFreeStateObservationSaturationDirective(saturationNoticeState())
	for _, banned := range []string{
		// Track identities never enter the notice.
		"synthetic-track", "track:10",
		// Processor domains and families never enter the notice.
		"broadband_compression", "static_eq", "track_gain", "de_esser", "multiband_dynamics",
		"compressor", "limiter",
		// View steering never enters the notice: the model owns the choice.
		"mix.multitrack_relationship", "mix.frequency_relationship",
		"request track.", "call ccb",
	} {
		if strings.Contains(strings.ToLower(directive), strings.ToLower(banned)) {
			t.Fatalf("saturation notice carries non-mechanical content %q in %q", banned, directive)
		}
	}
}

func TestObservationSaturationNoticeAbsentWithoutNotice(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"task_contract":             map[string]any{"kind": "improvement"},
		"free_state_reasoning_loop": map[string]any{"schema_version": "free_state_reasoning_loop.v1"},
	}}}
	if directive := messageLoopFreeStateObservationSaturationDirective(state); directive != "" {
		t.Fatalf("no notice on the loop must render no directive, got %q", directive)
	}
}

func TestObservationSaturationNoticeRidesPromptContextProjection(t *testing.T) {
	state := saturationNoticeState()
	projected := messageLoopFreeStatePromptContext(state)
	if _, ok := projected["observation_saturation_notice"]; !ok {
		t.Fatal("compact prompt projection must disclose the saturation notice")
	}
	// The notice rides the neutral-family system prompt as the same-family
	// mechanical directive channel.
	prompt := messageLoopNeutralFamilySystemPrompt(state)
	if !strings.Contains(prompt, "OBSERVATION SATURATION") {
		t.Fatal("neutral-family system prompt must carry the saturation notice directive")
	}
}
