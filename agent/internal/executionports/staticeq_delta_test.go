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
// post-mutation snapshots; extra revisions cover the delta refinement loop's
// post-write rebases.
func extendDeltaSnapshots(client *fakeNBVSPClient) {
	client.fakeVSPClient.snapshots = append(client.fakeVSPClient.snapshots,
		stateResultWithGain("epoch-eq", 10, "after", "t1", 0),
		stateResultWithGain("epoch-eq", 11, "after", "t1", 0),
		stateResultWithGain("epoch-eq", 12, "after", "t1", 0),
		stateResultWithGain("epoch-eq", 13, "after", "t1", 0),
		stateResultWithGain("epoch-eq", 14, "after", "t1", 0),
		stateResultWithGain("epoch-eq", 15, "after", "t1", 0),
		stateResultWithGain("epoch-eq", 16, "after", "t1", 0),
		stateResultWithGain("epoch-eq", 17, "after", "t1", 0),
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

// TestStaticEQVSPPortDeltaSemanticsRefinesCurvedTaperOntoTarget pins the
// 2026-08-30 p03 finding: the planning probe's average slope over a ±0.25
// normalized window misses a bent compressor taper near its limit (VSC-2
// threshold: 10.8 dB target, 10.1 dB achieved), and the refinement loop
// converges onto the target through measured secant steps instead of failing
// the action.
func TestStaticEQVSPPortDeltaSemanticsRefinesCurvedTaperOntoTarget(t *testing.T) {
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
	if err != nil || receipt.Status != "applied" {
		t.Fatalf("curved taper must converge onto the target: receipt=%+v err=%v", receipt, err)
	}
	// current 0 dB (norm 1), delta -1 dB -> target -1 dB on the k=4 taper
	if achieved, _ := receipt.Details["actual_readback_value"].(float64); math.Abs(achieved-(-1)) > ThresholdDeltaToleranceDB {
		t.Fatalf("actual_readback_value=%v want -1 within %.2f", achieved, ThresholdDeltaToleranceDB)
	}
	trace, _ := receipt.Details["delta_refinement"].([]map[string]any)
	if len(trace) == 0 {
		t.Fatalf("converged receipt must carry the refinement trace: %+v", receipt.Details)
	}
	refineSeen, restoreSeen := false, false
	for _, request := range client.requests {
		if strings.Contains(request, ":refine-") && !strings.Contains(request, "restore") {
			refineSeen = true
		}
		if strings.HasSuffix(request, ":refine-restore") {
			restoreSeen = true
		}
	}
	if !refineSeen {
		t.Fatalf("refinement writes not visible in requests: %v", client.requests)
	}
	if restoreSeen {
		t.Fatalf("converged action must not restore: %v", client.requests)
	}
	before, _ := receipt.Details["before_revision"].(string)
	after, _ := receipt.Details["after_revision"].(string)
	if before == "" || after == "" || before == after {
		t.Fatalf("refined receipt revisions not distinct: before=%q after=%q", before, after)
	}
}

// TestStaticEQVSPPortDeltaSemanticsRefinementExhaustionRestores pins the
// fail-closed shape: a display quantized coarser than the tolerance can never
// confirm the target, so the loop exhausts, restores the pre-action position,
// and the action fails without a net parameter move.
func TestStaticEQVSPPortDeltaSemanticsRefinementExhaustionRestores(t *testing.T) {
	client := newFakeNBVSPClient("thr_a", "thr_b", -20, 0, false)
	client.curveExponent = 4
	client.displayQuantum = 1 // integer-dB readout: -1.3 dB target is never confirmable
	client.params["thr_a"].normalized = 1
	client.params["thr_b"].normalized = 1
	extendDeltaSnapshots(client)
	port := &StaticEQVSPPort{Client: client, CommandName: "broadband_threshold_adjust"}
	actionSet := orchestration.ActionSet{ProjectCutHash: "cut-eq", Actions: []orchestration.Action{deltaAction(-1.3)}}
	if err := port.Preflight(context.Background(), actionSet, eqCut()); err != nil {
		t.Fatal(err)
	}
	receipt, err := port.Apply(context.Background(), actionSet.Actions[0], "execution:comp:a-delta")
	if err == nil || receipt.Status == "applied" {
		t.Fatalf("coarse display must fail closed: receipt=%+v err=%v", receipt, err)
	}
	if !strings.Contains(err.Error(), "not achieved") {
		t.Fatalf("unexpected error: %v", err)
	}
	restoreSeen := false
	for _, request := range client.requests {
		if strings.HasSuffix(request, ":refine-restore") {
			restoreSeen = true
		}
	}
	if !restoreSeen {
		t.Fatalf("exhausted refinement must restore the pre-action position: %v", client.requests)
	}
	if got := client.params["thr_a"].normalized; got != 1 {
		t.Fatalf("thr_a left at %v; refine restore failed", got)
	}
	if got := client.params["thr_b"].normalized; got != 1 {
		t.Fatalf("thr_b left at %v; refine restore failed", got)
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

// ---- single-channel (FAM1-S1 de_esser) normalized-batch shapes -------------
//
// GLM ruling on D2-FAM1-S1 ③: the normalized-batch Preflight guard demanding a
// non-empty param_id_ch2 is a PA dual-channel carrier shape assumption, not a
// safety property. One shared threshold parameter (FabFilter Pro-DS,
// pluginprobe 2026-08-31: id "1", stereo in/out) is a legal shape — plugin_path
// required, param_id_ch2 optional, one-channel batch = one revision advance.
// The dual-channel control beside these pins the PA carriers byte-for-byte
// through the same relaxation.

func deEsserSingleChannelAction(mutate func(map[string]any)) orchestration.Action {
	args := map[string]any{
		"write_mode": WriteModeNormalizedBatchV1, "plugin_path": "C:/plugins/FabFilter Pro-DS.vst3",
		"param_id": "deess_thresh", "target_value": -1.0, "target_semantics": deltaSemantics,
	}
	if mutate != nil {
		mutate(args)
	}
	return orchestration.Action{ID: "a-deess", Command: "de_esser_threshold_adjust", TargetRef: "t1",
		BeforeFingerprint: "track:t1:deess:deess_thresh:pending", Args: args}
}

func newSingleChannelFake() *fakeNBVSPClient {
	client := &fakeNBVSPClient{
		fakeVSPClient: fakeVSPClient{snapshots: eqSnapshots()},
		domainMin: -60, domainMax: 0, withCandidate: true,
		params: map[string]*fakeNBParam{"deess_thresh": {normalized: 0.4}},
	}
	extendDeltaSnapshots(client)
	return client
}

// RED-1 of the ruling: a single-channel delta action must pass Preflight
// without any param_id_ch2 argument.
func TestStaticEQVSPPreflightAdmitsSingleChannelNormalizedBatch(t *testing.T) {
	client := newSingleChannelFake()
	port := &StaticEQVSPPort{Client: client, CommandName: "de_esser_threshold_adjust"}
	actionSet := orchestration.ActionSet{ProjectCutHash: "cut-eq", Actions: []orchestration.Action{deEsserSingleChannelAction(nil)}}
	if err := port.Preflight(context.Background(), actionSet, eqCut()); err != nil {
		t.Fatalf("single-channel normalized batch rejected by preflight: %v", err)
	}
}

// RED-2 control: the relaxation must leave the dual-channel carrier shapes
// byte-for-byte intact — dual-channel delta still plans exactly two channels
// in its one forward mutation, and the guard keeps its plugin_path tooth.
func TestStaticEQVSPPreflightRelaxationKeepsDualChannelShapeIntact(t *testing.T) {
	client := newFakeNBVSPClient("thr_a", "thr_b", -20, 0, false)
	client.curveExponent = 1
	extendDeltaSnapshots(client)
	port := &StaticEQVSPPort{Client: client, CommandName: "broadband_threshold_adjust"}
	actionSet := orchestration.ActionSet{ProjectCutHash: "cut-eq", Actions: []orchestration.Action{deltaAction(2)}}
	if err := port.Preflight(context.Background(), actionSet, eqCut()); err != nil {
		t.Fatal(err)
	}
	receipt, err := port.Apply(context.Background(), actionSet.Actions[0], "execution:comp:a-delta")
	if err != nil || receipt.Status != "applied" {
		t.Fatalf("dual-channel delta regressed: receipt=%+v err=%v", receipt, err)
	}
	parameters, ok := client.batchArgs["parameters"].([]map[string]any)
	if !ok || len(parameters) != 2 {
		t.Fatalf("dual-channel batch shape drifted: %+v", client.batchArgs)
	}
	if parameters[0]["parameter_id"] != "thr_a" || parameters[1]["parameter_id"] != "thr_b" {
		t.Fatalf("dual-channel ids drifted: %+v", parameters)
	}
	if receipt.Details["param_id_ch2"] != "thr_b" {
		t.Fatalf("dual-channel receipt details drifted: %+v", receipt.Details)
	}

	// The retained half of the guard: a normalized-batch action without any
	// plugin_path stays rejected.
	identityLess := deltaAction(2)
	delete(identityLess.Args, "plugin_path")
	port2 := &StaticEQVSPPort{Client: newFakeNBVSPClient("thr_a", "thr_b", -20, 0, false), CommandName: "broadband_threshold_adjust"}
	set2 := orchestration.ActionSet{ProjectCutHash: "cut-eq", Actions: []orchestration.Action{identityLess}}
	if err := port2.Preflight(context.Background(), set2, eqCut()); err == nil {
		t.Fatal("normalized batch without plugin_path accepted")
	}
}

// RED-3 of the ruling: a non-delta (absolute) single-channel action must be
// plannable — the Apply call site filters the empty ch2 id instead of passing
// it into eqPlanGainChannels, which hard-errors on empty IDs.
func TestStaticEQVSPPortSingleChannelAbsoluteAppliesOneEntryBatch(t *testing.T) {
	client := &fakeNBVSPClient{
		fakeVSPClient: fakeVSPClient{snapshots: eqSnapshots()},
		domainMin: -60, domainMax: 0, withCandidate: true,
		params: map[string]*fakeNBParam{"deess_thresh": {normalized: 0.4}},
	}
	port := &StaticEQVSPPort{Client: client, CommandName: "de_esser_threshold_adjust"}
	action := deEsserSingleChannelAction(func(args map[string]any) { delete(args, "target_semantics") })
	actionSet := orchestration.ActionSet{ProjectCutHash: "cut-eq", Actions: []orchestration.Action{action}}
	if err := port.Preflight(context.Background(), actionSet, eqCut()); err != nil {
		t.Fatal(err)
	}
	receipt, err := port.Apply(context.Background(), action, "execution:deess:a-deess")
	if err != nil || receipt.Status != "applied" {
		t.Fatalf("single-channel absolute regressed: receipt=%+v err=%v", receipt, err)
	}
	parameters, ok := client.batchArgs["parameters"].([]map[string]any)
	if !ok || len(parameters) != 1 || parameters[0]["parameter_id"] != "deess_thresh" {
		t.Fatalf("one-entry batch expected: %+v", client.batchArgs)
	}
	const wantNormalized = (-1.0 - (-60)) / 60 // linear [-60,0] domain
	if got := parameters[0]["normalized_value"].(float64); math.Abs(got-wantNormalized) > 1e-9 {
		t.Fatalf("normalized=%v want %v", got, wantNormalized)
	}
	if receipt.Details["readback_verified"] != true || receipt.Details["param_id_ch2"] != "" {
		t.Fatalf("single-channel receipt details=%+v", receipt.Details)
	}
	channels, _ := receipt.Details["normalized_channels"].([]map[string]any)
	if len(channels) != 1 {
		t.Fatalf("channel records=%+v", receipt.Details["normalized_channels"])
	}
}

// The production de_esser shape end to end: single-channel delta planning
// through the probe cycle onto a one-entry batch write.
func TestStaticEQVSPPortSingleChannelDeltaProbesSlopeAndApplies(t *testing.T) {
	client := newSingleChannelFake()
	port := &StaticEQVSPPort{Client: client, CommandName: "de_esser_threshold_adjust"}
	actionSet := orchestration.ActionSet{ProjectCutHash: "cut-eq", Actions: []orchestration.Action{deEsserSingleChannelAction(nil)}}
	if err := port.Preflight(context.Background(), actionSet, eqCut()); err != nil {
		t.Fatal(err)
	}
	receipt, err := port.Apply(context.Background(), actionSet.Actions[0], "execution:deess:a-deess")
	if err != nil || receipt.Status != "applied" {
		t.Fatalf("single-channel delta regressed: receipt=%+v err=%v", receipt, err)
	}
	parameters, ok := client.batchArgs["parameters"].([]map[string]any)
	if !ok || len(parameters) != 1 || parameters[0]["parameter_id"] != "deess_thresh" {
		t.Fatalf("one-entry delta batch expected: %+v", client.batchArgs)
	}
	// current -36 dB (norm 0.4), delta -1 dB -> target -37 dB at norm 23/60
	wantNormalized := (-37.0 - (-60)) / 60
	if got := client.params["deess_thresh"].normalized; math.Abs(got-wantNormalized) > 1e-9 {
		t.Fatalf("deess_thresh normalized=%v want %v", got, wantNormalized)
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
	if achieved := receipt.Details["actual_readback_value"]; math.Abs(achieved.(float64)-(-37.0)) > ThresholdDeltaToleranceDB {
		t.Fatalf("actual_readback_value=%v want -37", achieved)
	}
}
