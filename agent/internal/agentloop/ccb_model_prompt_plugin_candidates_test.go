package agentloop

import (
	"strings"
	"testing"
)

// FIX-PLUGIN-SELECT-1: the machine whitelist's multi-candidate families are
// disclosed to the model as bounded plugin-candidate rows (structural fields
// only), and only for families with more than one certified candidate. A
// single-candidate (v5-shaped) whitelist renders NO directive so today's
// prompt stays byte-identical.

func pluginCandidateDisclosureState(families []map[string]any) *runState {
	ctx := map[string]any{}
	if families != nil {
		// The chat side rides the disclosure inside the free-state loop
		// projection (freeStateLoopMap), so the directive reads it there.
		ctx["free_state_reasoning_loop"] = map[string]any{
			"plugin_candidate_disclosure": map[string]any{
				"schema_version": "free_state_plugin_candidate_disclosure.v1",
				"families":       families,
			},
		}
	}
	return &runState{input: Input{Context: ctx}}
}

func TestPluginCandidateDirectiveRendersDisclosedIdentifiers(t *testing.T) {
	state := pluginCandidateDisclosureState([]map[string]any{{
		"action_domain": "static_eq",
		"capability":    "one bounded band gain move within +/-2 dB",
		"candidates": []map[string]any{
			{"name": "Q10", "manufacturer": "Waves", "format": "VST3", "identifier": "VST3-Q10-abc"},
			{"name": "EMO-F2", "manufacturer": "Other", "format": "VST3", "identifier": "VST3-EMO-F2-def"},
		},
	}})
	directive := messageLoopPluginCandidateDirective(state)
	if directive == "" {
		t.Fatal("multi-candidate disclosure must render a directive")
	}
	for _, want := range []string{"static_eq", "VST3-Q10-abc", "VST3-EMO-F2-def", "plugin_identifier"} {
		if !strings.Contains(directive, want) {
			t.Fatalf("directive must carry %q: %q", want, directive)
		}
	}
	if !strings.Contains(directive, "needs_experiment") {
		t.Fatalf("directive must bind the selection to the needs_experiment proposal: %q", directive)
	}
}

func TestPluginCandidateDirectiveSilentWithoutDisclosure(t *testing.T) {
	if directive := messageLoopPluginCandidateDirective(pluginCandidateDisclosureState(nil)); directive != "" {
		t.Fatalf("directive must stay silent without a disclosure, got %q", directive)
	}
}

// The system prompt embeds the directive only when the disclosure rides the
// context; without it the prompt is byte-identical to the pre-v6 prompt.
func TestNeutralFamilyPromptCarriesPluginCandidateDirective(t *testing.T) {
	withDisclosure := messageLoopNeutralFamilySystemPrompt(pluginCandidateDisclosureState([]map[string]any{{
		"action_domain": "static_eq",
		"capability":    "one bounded band gain move within +/-2 dB",
		"candidates": []map[string]any{
			{"name": "Q10", "manufacturer": "Waves", "format": "VST3", "identifier": "VST3-Q10-abc"},
		},
	}}))
	if !strings.Contains(withDisclosure, "PLUGIN CANDIDATES") || !strings.Contains(withDisclosure, "VST3-Q10-abc") {
		t.Fatal("system prompt must embed the plugin candidate directive")
	}
	without := messageLoopNeutralFamilySystemPrompt(pluginCandidateDisclosureState(nil))
	if strings.Contains(without, "PLUGIN CANDIDATES") {
		t.Fatal("system prompt must not mention plugin candidates without a disclosure")
	}
}
