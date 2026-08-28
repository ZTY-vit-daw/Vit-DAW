package executionports

import (
	"context"
	"math"
	"strings"
	"testing"

	"vit-daw-agent/internal/orchestration"
)

// The delta-semantics machine exists because the real broadband compressor
// threshold exposes a structurally degenerate display probe (five identical
// "+11.8" samples, 2026-08-28): these tests pin the transactional probe
// behavior against a fake whose live value_text responds to writes only when
// the plugin actually would.

func deltaAction(delta float64) orchestration.Action {
	return orchestration.Action{ID: "a-delta", Command: "broadband_threshold_adjust", TargetRef: "t1",
		BeforeFingerprint: "track:t1:comp:plg_1:pending",
		Args: map[string]any{
			"write_mode": WriteModeNormalizedBatchV1, "plugin_path": "C:/plugins/Vertigo VSC-2.vst3",
			"param_id": "thr_a", "param_id_ch2": "thr_b",
			"target_value": delta, "target_semantics": deltaSemantics,
		}}
}

// extendDeltaSnapshots feeds the fake the extra rebase snapshots the probe
// cycle consumes (probe rebase, restore rebase) on top of the preflight and
// post-mutation snapshots.
func extendDeltaSnapshots(client *fakeNBVSPClient) {
	client.fakeVSPClient.snapshots = append(client.fakeVSPClient.snapshots,
		stateResultWithGain("epoch-eq", 10, "after", "t1", 0),
		stateResultWithGain("epoch-eq", 11, "after", "t1", 0),
		stateResultWithGain("epoch-eq", 12, "after", "t1", 0),
	)
}

func TestStaticEQVSPPortDeltaSemanticsProbesSlopeAndApplies(t *testing.T) {
	client := newFakeNBVSPClient("thr_a", "thr_b", -20, 0, false)
	client.curveExponent = 1 // linear domain rendered as the realistic "+x.xx" text
	extendDeltaSnapshots(client)
	port := &StaticEQVSPPort{Client: client, CommandName: "broadband_threshold_adjust"}
	actionSet := orchestration.ActionSet{ProjectCutHash: "cut-eq", Actions: []orchestration.Action{deltaAction(2)}}
	if err := port.Preflight(context.Background(), actionSet, eqCut()); err != nil {
		t.Fatal(err)
	}
	receipt, err := port.Apply(context.Background(), actionSet.Actions[0], "execution:comp:a-delta")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != "applied" {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	// current -10 dB (norm 0.5), delta +2 dB -> target -8 dB at norm 0.6
	if got := client.params["thr_a"].normalized; math.Abs(got-0.6) > 1e-9 {
		t.Fatalf("thr_a normalized=%v want 0.6", got)
	}
	if got := client.params["thr_b"].normalized; math.Abs(got-0.6) > 1e-9 {
		t.Fatalf("thr_b normalized=%v want 0.6", got)
	}
	probeSeen, restoreSeen := false, false
	for _, request := range client.requests {
		if strings.HasSuffix(request, ":probe") {
			probeSeen = true
		}
		if strings.HasSuffix(request, ":probe-restore") {
			restoreSeen = true
		}
	}
	if !probeSeen || !restoreSeen {
		t.Fatalf("probe cycle not visible in requests: %v", client.requests)
	}
	calibration, ok := receipt.Details["delta_calibration"].(map[string]any)
	if !ok {
		t.Fatalf("receipt missing delta_calibration: %+v", receipt.Details)
	}
	if slope, _ := calibration["slope_db_per_normalized"].(float64); math.Abs(slope-20) > 1e-6 {
		t.Fatalf("slope=%v want 20", calibration["slope_db_per_normalized"])
	}
	if achieved := receipt.Details["actual_readback_value"]; math.Abs(achieved.(float64)-(-8)) > ThresholdDeltaToleranceDB {
		t.Fatalf("actual_readback_value=%v want -8", achieved)
	}
}

func TestStaticEQVSPPortDeltaSemanticsFailsClosedOnFrozenDisplay(t *testing.T) {
	client := newFakeNBVSPClient("thr_a", "thr_b", -20, 0, false)
	client.frozenPhysicalText = true
	extendDeltaSnapshots(client)
	port := &StaticEQVSPPort{Client: client, CommandName: "broadband_threshold_adjust"}
	actionSet := orchestration.ActionSet{ProjectCutHash: "cut-eq", Actions: []orchestration.Action{deltaAction(-1)}}
	if err := port.Preflight(context.Background(), actionSet, eqCut()); err != nil {
		t.Fatal(err)
	}
	receipt, err := port.Apply(context.Background(), actionSet.Actions[0], "execution:comp:a-delta")
	if err == nil || receipt.Status == "applied" {
		t.Fatalf("frozen display must fail closed: receipt=%+v err=%v", receipt, err)
	}
	if !strings.Contains(err.Error(), "does not respond to writes") {
		t.Fatalf("unexpected error: %v", err)
	}
	// both probe directions ran and every probe was restored: no net move
	if got := client.params["thr_a"].normalized; math.Abs(got-0.5) > 1e-9 {
		t.Fatalf("thr_a left at %v; probe restore failed", got)
	}
}

func TestStaticEQVSPPortDeltaSemanticsRejectsUnreachableTarget(t *testing.T) {
	client := newFakeNBVSPClient("thr_a", "thr_b", -20, 0, false)
	client.curveExponent = 1
	client.params["thr_a"].normalized = 1 // threshold at domain max 0 dB
	client.params["thr_b"].normalized = 1
	extendDeltaSnapshots(client)
	port := &StaticEQVSPPort{Client: client, CommandName: "broadband_threshold_adjust"}
	actionSet := orchestration.ActionSet{ProjectCutHash: "cut-eq", Actions: []orchestration.Action{deltaAction(2)}}
	if err := port.Preflight(context.Background(), actionSet, eqCut()); err != nil {
		t.Fatal(err)
	}
	receipt, err := port.Apply(context.Background(), actionSet.Actions[0], "execution:comp:a-delta")
	if err == nil || receipt.Status == "applied" {
		t.Fatalf("out-of-range delta must fail: receipt=%+v err=%v", receipt, err)
	}
	if !strings.Contains(err.Error(), "outside the reachable normalized range") {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := client.params["thr_a"].normalized; got != 1 {
		t.Fatalf("thr_a left at %v; probe restore failed", got)
	}
}

func TestStaticEQVSPPortDeltaSemanticsGateRejectsCurvatureMiss(t *testing.T) {
	client := newFakeNBVSPClient("thr_a", "thr_b", -20, 0, false)
	client.curveExponent = 4 // strongly curved: linear estimate from the probe misses the target
	client.params["thr_a"].normalized = 1
	client.params["thr_b"].normalized = 1
	extendDeltaSnapshots(client)
	port := &StaticEQVSPPort{Client: client, CommandName: "broadband_threshold_adjust"}
	actionSet := orchestration.ActionSet{ProjectCutHash: "cut-eq", Actions: []orchestration.Action{deltaAction(-1)}}
	if err := port.Preflight(context.Background(), actionSet, eqCut()); err != nil {
		t.Fatal(err)
	}
	receipt, err := port.Apply(context.Background(), actionSet.Actions[0], "execution:comp:a-delta")
	if err == nil || receipt.Status == "applied" {
		t.Fatalf("curvature miss must not count as applied: receipt=%+v err=%v", receipt, err)
	}
	if !strings.Contains(err.Error(), "delta physical target") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStaticEQVSPPortAbsoluteSemanticsUnchanged(t *testing.T) {
	// No target_semantics arg: the absolute curve path stays in charge even
	// when the fake bends like a compressor threshold.
	client := newFakeNBVSPClient("p315_c1", "p315_c2", -24, 24, true)
	port := &StaticEQVSPPort{Client: client}
	actionSet := orchestration.ActionSet{ProjectCutHash: "cut-eq", Actions: []orchestration.Action{nbAction()}}
	if err := port.Preflight(context.Background(), actionSet, eqCut()); err != nil {
		t.Fatal(err)
	}
	receipt, err := port.Apply(context.Background(), actionSet.Actions[0], "execution:eq:a-nb")
	if err != nil || receipt.Status != "applied" {
		t.Fatalf("absolute path regressed: receipt=%+v err=%v", receipt, err)
	}
	if _, present := receipt.Details["delta_calibration"]; present {
		t.Fatalf("absolute path must not carry delta calibration details")
	}
}
