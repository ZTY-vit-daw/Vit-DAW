package capabilityadapters

import (
	"testing"

	"vit-daw-agent/internal/orchestration"
)

func executablePluginCut() orchestration.ProjectCut {
	cut := orchestration.ProjectCut{ProjectUUID: "project-1", ProjectEpoch: "epoch-1", Consistency: "strong"}
	cut.Hash = cut.ComputeHash()
	return cut
}

func TestFreezePluginEffectControlFreezesResolvedPreimage(t *testing.T) {
	proposal, actionSet, err := FreezePluginEffectControl(PluginEffectControlPlan{
		TrackID: "track-1", PluginID: "plugin-1", Control: "cut mud",
		ApplyArgs:          map[string]any{"target": map[string]any{"frequency_hz": 200.0}},
		ResolvedParameters: []PluginResolvedParameter{{ParameterID: "gain", OldNormalizedValue: 0.5}},
		ProfileSource:      "verified-vps", ProfileSignature: "sha256:test",
	}, executablePluginCut(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.CapabilityID != PluginEffectControlCapabilityID || actionSet.CapabilityID != PluginEffectControlCapabilityID {
		t.Fatalf("unexpected capability: proposal=%q actionSet=%q", proposal.CapabilityID, actionSet.CapabilityID)
	}
	if len(actionSet.Actions) != 1 {
		t.Fatalf("expected one action, got %d", len(actionSet.Actions))
	}
	action := actionSet.Actions[0]
	if action.Command != "plugin_grabber.apply_control.governed" || !action.Compensatable {
		t.Fatalf("unexpected governed action: %#v", action)
	}
	ids, ok := action.Args["expected_parameter_ids"].([]string)
	if !ok || len(ids) != 1 || ids[0] != "gain" {
		t.Fatalf("unexpected frozen identities: %#v", action.Args["expected_parameter_ids"])
	}
	if action.BeforeFingerprint == "" || actionSet.Hash == "" || proposal.ActionSetHash != actionSet.Hash {
		t.Fatalf("missing immutable freeze metadata: action=%#v proposal=%#v set=%#v", action, proposal, actionSet)
	}
}

func TestFreezePluginEffectControlRejectsMissingPreimage(t *testing.T) {
	_, _, err := FreezePluginEffectControl(PluginEffectControlPlan{
		TrackID: "track-1", PluginID: "plugin-1", Control: "cut mud",
	}, executablePluginCut(), 1)
	if err == nil {
		t.Fatal("expected missing resolve-only preimage to be rejected")
	}
}
