package executionverifiers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/orchestration"
)

func pluginEffectTestProjection(trackID, band string, energy float64) map[string]any {
	return map[string]any{
		"multitrack_relation": map[string]any{
			"band_occupancy": []any{
				map[string]any{"band": band, "leaders": []any{map[string]any{"track_id": trackID, "unit_energy": energy}}},
			},
		},
	}
}

func pluginEffectTestActionSet(beforeProjection map[string]any) orchestration.ActionSet {
	return orchestration.ActionSet{ID: "plugin_effect_test", Actions: []orchestration.Action{{
		ID: "plugin_effect:t1:p1", Command: "plugin_grabber.apply_control.governed",
		Args: map[string]any{
			"track_id": "t1", "plugin_id": "p1", "control": "eq.cut_region",
			"apply_args":      map[string]any{"track_id": "t1", "plugin_id": "p1", "control": "eq.cut_region", "target": map[string]any{"freq_hz": 200.0, "gain_db": -3.0, "q": 1.0}},
			"before_evidence": map[string]any{"observation": map[string]any{"observation_id": "obs_before", "mom_projection": beforeProjection}},
		},
	}}}
}

func pluginEffectTestReceipt() []orchestration.ActionReceipt {
	return []orchestration.ActionReceipt{{ActionID: "plugin_effect:t1:p1", Status: "applied", Details: map[string]any{"structural_readback": "pass"}}}
}

func TestPluginEffectControlVerifierPassesFreshStructuralAndDirectionalEvidence(t *testing.T) {
	dir := t.TempDir()
	invoker := &recordingHarnessInvoker{Response: harness.InvokeResponse{Status: "ok", Result: map[string]any{
		"observation_id": "obs_after", "mom_projection": pluginEffectTestProjection("t1", "bass", 0.2),
	}}}
	result, err := (PluginEffectControl{Invoker: invoker, PreviousObservationID: "obs_before", EvidenceDir: dir}).Verify(
		context.Background(), pluginEffectTestActionSet(pluginEffectTestProjection("t1", "bass", 0.5)), pluginEffectTestReceipt(),
	)
	if err != nil || result.Status != "pass" || result.Structural != "pass" || result.Acoustic != "pass" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	for _, name := range []string{"plugin_effect_test_before_frequency.png", "plugin_effect_test_after_frequency.png"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || info.Size() == 0 {
			t.Fatalf("missing PNG %s: info=%#v err=%v", name, info, err)
		}
	}
}

func TestPluginEffectControlVerifierRejectsReusedObservationID(t *testing.T) {
	invoker := &recordingHarnessInvoker{Response: harness.InvokeResponse{Status: "ok", Result: map[string]any{
		"observation_id": "obs_before", "mom_projection": pluginEffectTestProjection("t1", "bass", 0.2),
	}}}
	result, err := (PluginEffectControl{Invoker: invoker, PreviousObservationID: "obs_before", EvidenceDir: t.TempDir()}).Verify(
		context.Background(), pluginEffectTestActionSet(pluginEffectTestProjection("t1", "bass", 0.5)), pluginEffectTestReceipt(),
	)
	if err != nil || result.Status != "fail" || result.Acoustic != "fail" || !strings.Contains(result.Summary, "reused") {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestPluginEffectControlVerifierRejectsOppositeDirection(t *testing.T) {
	invoker := &recordingHarnessInvoker{Response: harness.InvokeResponse{Status: "ok", Result: map[string]any{
		"observation_id": "obs_after", "mom_projection": pluginEffectTestProjection("t1", "bass", 0.8),
	}}}
	result, err := (PluginEffectControl{Invoker: invoker, PreviousObservationID: "obs_before", EvidenceDir: t.TempDir()}).Verify(
		context.Background(), pluginEffectTestActionSet(pluginEffectTestProjection("t1", "bass", 0.5)), pluginEffectTestReceipt(),
	)
	if err != nil || result.Status != "fail" || result.Acoustic != "fail" || !strings.Contains(result.Summary, "contradicts") {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestPluginEffectControlVerifierIsInconclusiveWithoutComparableMOM(t *testing.T) {
	invoker := &recordingHarnessInvoker{Response: harness.InvokeResponse{Status: "ok", Result: map[string]any{
		"observation_id": "obs_after", "mom_projection": pluginEffectTestProjection("t2", "bass", 0.2),
	}}}
	result, err := (PluginEffectControl{Invoker: invoker, PreviousObservationID: "obs_before", EvidenceDir: t.TempDir()}).Verify(
		context.Background(), pluginEffectTestActionSet(pluginEffectTestProjection("t1", "bass", 0.5)), pluginEffectTestReceipt(),
	)
	if err != nil || result.Status != "inconclusive" || result.Acoustic != "inconclusive" || !strings.Contains(result.Summary, "lacks comparable") {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}
