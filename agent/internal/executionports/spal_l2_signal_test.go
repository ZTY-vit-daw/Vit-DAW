package executionports

import (
	"context"
	"errors"
	"testing"

	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/spal"
)

type signalCaptureMutationPort struct {
	manifest spal.ExecutionManifest
	applied  bool
}

func (p *signalCaptureMutationPort) Preflight(context.Context, orchestration.ActionSet, orchestration.ProjectCut) error {
	return nil
}

func (p *signalCaptureMutationPort) Apply(_ context.Context, action orchestration.Action, _ string) (orchestration.ActionReceipt, error) {
	p.applied = true
	return orchestration.ActionReceipt{
		ActionID: action.ID,
		Status:   "applied",
		Details: map[string]any{
			"structural_readback": "pass",
			"spal_manifest_id":    p.manifest.ID,
			"binding_id":          p.manifest.Binding.ID,
		},
	}, nil
}

type fakeL2SignalCapturer struct {
	probes map[string]spal.RenderProbe
	errs   map[string]error
	phases []string
}

func (f *fakeL2SignalCapturer) CaptureSPALSignal(_ context.Context, _ spal.ExecutionManifest, phase, _ string) (spal.RenderProbe, error) {
	f.phases = append(f.phases, phase)
	if err := f.errs[phase]; err != nil {
		return spal.RenderProbe{}, err
	}
	return f.probes[phase], nil
}

func TestSPALSignalCapturePortPersistsBeforeAndAfterEvidence(t *testing.T) {
	manifest, action, _ := testSPALManifestAction(t)
	manifest.Instruction.SignalProbeScope = testSignalScope()
	set, err := manifest.ToActionSet("static_mix.low_end_relation.v0", testSignalCut())
	if err != nil {
		t.Fatal(err)
	}
	action = set.Actions[0]
	mutation := &signalCaptureMutationPort{manifest: manifest}
	capturer := &fakeL2SignalCapturer{probes: map[string]spal.RenderProbe{
		"before": testRenderProbe("before", -20),
		"after":  testRenderProbe("after", -23),
	}}
	port := &SPALSignalCapturePort{Mutation: mutation, Signal: capturer}
	receipt, err := port.Apply(context.Background(), action, "exec:signal")
	if err != nil || !mutation.applied || len(capturer.phases) != 2 || capturer.phases[0] != "before" || capturer.phases[1] != "after" {
		t.Fatalf("signal capture lifecycle=%#v receipt=%#v err=%v", capturer.phases, receipt, err)
	}
	evidence, err := spal.SignalProbeEvidenceFromAny(receipt.Details["spal_signal_evidence"])
	if err != nil || evidence.Before == nil || evidence.After == nil || evidence.Verify(manifest.Instruction.ExpectedSignalChange).Status != "pass" {
		t.Fatalf("receipt did not retain durable signal evidence: %#v err=%v", evidence, err)
	}
	if len(receipt.EvidenceRefs) != 2 {
		t.Fatalf("probe evidence refs did not join the receipt: %#v", receipt)
	}
}

func TestSPALSignalCaptureDoesNotBlockStructuralMutationWhenProbeUnavailable(t *testing.T) {
	manifest, _, _ := testSPALManifestAction(t)
	manifest.Instruction.SignalProbeScope = testSignalScope()
	set, err := manifest.ToActionSet("static_mix.low_end_relation.v0", testSignalCut())
	if err != nil {
		t.Fatal(err)
	}
	mutation := &signalCaptureMutationPort{manifest: manifest}
	port := &SPALSignalCapturePort{
		Mutation: mutation,
		Signal:   &fakeL2SignalCapturer{errs: map[string]error{"before": errors.New("offline renderer unavailable")}},
	}
	receipt, err := port.Apply(context.Background(), set.Actions[0], "exec:signal")
	if err != nil || !mutation.applied || receipt.Status != "applied" {
		t.Fatalf("unavailable signal probe blocked a structurally valid action: receipt=%#v err=%v", receipt, err)
	}
	evidence, decodeErr := spal.SignalProbeEvidenceFromAny(receipt.Details["spal_signal_evidence"])
	if decodeErr != nil || len(evidence.CaptureErrors) == 0 || evidence.Verify(manifest.Instruction.ExpectedSignalChange).Status != "inconclusive" {
		t.Fatalf("probe failure was not preserved as inconclusive evidence: %#v err=%v", evidence, decodeErr)
	}
}

func TestL2ProbeCommandAndEventUseExactTargetBand(t *testing.T) {
	manifest, _, _ := testSPALManifestAction(t)
	manifest.Instruction.SignalProbeScope = testSignalScope()
	command := l2ProbeCommand(manifest, "probe-before")
	if command["analysis_band_low_hz"] != 80.0 || command["analysis_band_high_hz"] != 110.0 || command["track_id"] != manifest.Binding.Instance.TrackID {
		t.Fatalf("L2 command did not carry frozen target band: %#v", command)
	}
	event := map[string]any{
		"command": "l2_render_probe_ready", "feature_type": "l2_render_probe", "status": "ready",
		"tap_point": "track_post_fader", "render_mode": "offline_probe", "track_id": manifest.Binding.Instance.TrackID,
		"clip_id": "clip-1", "render_revision": "after", "evidence_ref": "dad.l2_render_probe:after",
		"quality_evidence": map[string]any{"nonzero": true, "sum_abs": 1.0, "max_abs": 0.2, "nan_inf_count": 0},
		"bands": map[string]any{
			"bass":        map[string]any{"min_hz": 60.0, "max_hz": 250.0, "energy_db": -10.0},
			"spal_target": map[string]any{"min_hz": 80.0, "max_hz": 110.0, "energy_db": -23.0},
		},
	}
	probe, err := l2EventToRenderProbe(event, manifest)
	if err != nil || len(probe.Bands) != 2 {
		t.Fatalf("L2 event did not decode: probe=%#v err=%v", probe, err)
	}
	result := spal.VerifySignalDirection(manifest.Instruction.ExpectedSignalChange, testRenderProbe("before", -20), probe)
	if result.Status != "pass" || result.AfterDB != -23 {
		t.Fatalf("target-specific L2 band was not used: %#v", result)
	}
}

func testSignalScope() spal.SignalProbeScope {
	return spal.SignalProbeScope{TapPoint: "track_post_fader", RenderMode: "offline_probe", ClipID: "clip-1"}
}

func testSignalCut() orchestration.ProjectCut {
	cut := orchestration.ProjectCut{ProjectUUID: "project-1", ProjectEpoch: "epoch-1", BaseProjectRevision: "7", Consistency: "strong"}
	cut.Hash = cut.ComputeHash()
	return cut
}

func testRenderProbe(revision string, energy float64) spal.RenderProbe {
	return spal.RenderProbe{
		TapPoint: "track_post_fader", RenderMode: "offline_probe", RenderRevision: revision,
		TrackID: "bass", EvidenceRef: "dad.l2_render_probe:" + revision,
		Bands: []spal.BandEnergy{{MinHz: 60, MaxHz: 250, EnergyDB: -10}, {MinHz: 80, MaxHz: 110, EnergyDB: energy}},
	}
}
