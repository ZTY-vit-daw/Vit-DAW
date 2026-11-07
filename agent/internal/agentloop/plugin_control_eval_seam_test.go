package agentloop

import (
	"strings"
	"testing"
)

// PLUGIN-CONTROL-EVAL-SEAM-1 S4 (typed-routing rule strip), B4/C4 ablation-hook
// test pattern: default unset keeps the ordinary-family system skeleton's
// typed effect-control routing rules byte-for-byte; VIT_PLUGIN_CONTROL_EVAL_SEAM=1
// removes exactly the eight typed-routing rule lines (the block between
// "- Before freezing an actionable generic EQ semantic_action" and
// "- Mixing is a native Ask Vit conversation task") while the neighboring
// rules survive; only the exact value "1" arms the hook.

var seamS4TypedRuleMarkers = []string{
	"- Before freezing an actionable generic EQ semantic_action",
	"- A <generic_eq_topology> or context.generic_eq_topology block",
	"- semantic_action has no mutation authority.",
	"- For an explicit generic static-EQ parameter request",
	"- For an explicit broadband-compressor parameter request",
	"- One successful inspect_compressor result is sufficient",
	"- A compressor apply result is successful only when",
	"- Generic parameter writes are an explicit-control fallback only",
}

func seamS4VerifiedEntryState() *runState {
	return &runState{input: Input{
		UserText: "把插件的 Mix 参数调到 35%",
		Context: map[string]any{
			"semantic_entry_verified": true,
			"semantic_entry_decision": map[string]any{
				"schema_version": "semantic_entry_decision.v1", "route": "other",
				"control_mode": "ordinary_agent", "user_authorization": "action_requested",
			},
		},
	}}
}

func TestSeamS4ClosedKeepsTypedRoutingRules(t *testing.T) {
	skeleton := messageLoopOrdinaryFamilySystemSkeleton(nil)
	for _, marker := range seamS4TypedRuleMarkers {
		if !strings.Contains(skeleton, marker) {
			t.Fatalf("closed seam must keep rule %q", marker)
		}
	}
	if !strings.Contains(skeleton, "- Mixing is a native Ask Vit conversation task") {
		t.Fatal("closed seam must keep the neighbor rule after the typed block")
	}
	if !strings.Contains(skeleton, "- Treat a simultaneous positive goal and negative listening constraint") {
		t.Fatal("closed seam must keep the neighbor rule before the typed block")
	}
}

// A-arm condition proof, S4 face: seam open → the ordinary loop's system
// prompt no longer steers the model toward typed inspect/apply surfaces.
func TestSeamS4OpenStripsTypedRoutingRulesKeepsNeighbors(t *testing.T) {
	t.Setenv(pluginControlEvalSeamEnv, "1")
	skeleton := messageLoopOrdinaryFamilySystemSkeleton(nil)
	for _, marker := range seamS4TypedRuleMarkers {
		if strings.Contains(skeleton, marker) {
			t.Fatalf("open seam must strip rule %q", marker)
		}
	}
	if !strings.Contains(skeleton, "- Mixing is a native Ask Vit conversation task") {
		t.Fatal("open seam must keep the neighbor rule after the typed block")
	}
	if !strings.Contains(skeleton, "- Treat a simultaneous positive goal and negative listening constraint") {
		t.Fatal("open seam must keep the neighbor rule before the typed block")
	}
	if !strings.Contains(skeleton, "clip.strip_silence.suggest") {
		t.Fatal("open seam must keep unrelated skeleton rules")
	}
}

// Seam + verified-entry composition: the seam trim is state-independent and
// the verified-entry trim's active half (removing the semantic EQ proposal
// example) keeps working alongside it. In the composed mode the A arm actually
// runs in, the typed rules stay stripped and no partial trim or panic occurs.
func TestSeamS4OpenWithVerifiedEntryStable(t *testing.T) {
	t.Setenv(pluginControlEvalSeamEnv, "1")
	skeleton := messageLoopOrdinaryFamilySystemSkeleton(seamS4VerifiedEntryState())
	for _, marker := range seamS4TypedRuleMarkers {
		if strings.Contains(skeleton, marker) {
			t.Fatalf("open seam with verified entry must strip rule %q", marker)
		}
	}
	if strings.Contains(skeleton, `{"final":true,"reply":"concrete EQ proposal`) {
		t.Fatal("verified-entry example trim must keep working with the seam open")
	}
	if !strings.Contains(skeleton, "- Mixing is a native Ask Vit conversation task") {
		t.Fatal("composed trim must keep the neighbor rules")
	}
}

// Closed seam + verified entry keeps the existing verified-entry trim behavior
// unchanged (regression guard): the trim's active half removes the semantic
// EQ proposal example. Its rule-block half's start marker
// ("- For an actionable generic static-EQ listening goal") does not occur in
// the current skeleton text — a pre-existing stale marker recorded in the card
// receipt, deliberately not "fixed" here because any change to the unset-path
// skeleton would violate the seam's byte-identical red line.
func TestSeamS4ClosedVerifiedEntryTrimUnchanged(t *testing.T) {
	skeleton := messageLoopOrdinaryFamilySystemSkeleton(seamS4VerifiedEntryState())
	if strings.Contains(skeleton, `{"final":true,"reply":"concrete EQ proposal`) {
		t.Fatal("closed seam must keep the verified-entry trim removing the EQ proposal example")
	}
	for _, marker := range []string{"- For an explicit generic static-EQ parameter request", "- Generic parameter writes are an explicit-control fallback only"} {
		if !strings.Contains(skeleton, marker) {
			t.Fatalf("closed seam verified trim must keep typed rule %q", marker)
		}
	}
}

func TestSeamS4OnlyExactOneArmsTheStrip(t *testing.T) {
	closed := messageLoopOrdinaryFamilySystemSkeleton(nil)
	for _, value := range []string{"", "0", "true", "off"} {
		t.Setenv(pluginControlEvalSeamEnv, value)
		if pluginControlEvalSeamOpen() {
			t.Fatalf("env value %q must not open the seam", value)
		}
		if got := messageLoopOrdinaryFamilySystemSkeleton(nil); got != closed {
			t.Fatalf("env value %q must keep the skeleton byte-for-byte", value)
		}
	}
}
