package fxm

import (
	"encoding/json"
	"strings"
	"testing"
)

// M2-TIMEFX-2 contract tests: the same-source/same-window/same-route
// differential gate. A faithful kernel artifact must yield a projection with
// real, finite, non-nil per-tap measurements; any identity dimension that
// disagrees between the taps must land the projection in stale/blocked, not
// ready. The tamper vectors below are producer-level (the artifact identity
// is split after decode), pinning the fxm.Build gate through the exact Input
// shape InputFromTimefxEvidence produces.

func timefxTestArtifact() TimefxEvidenceArtifact {
	metrics := func(peak, rms, lufs, crest float64) TimefxTapMetricsJSON {
		return TimefxTapMetricsJSON{SampleFrames: 480000, NonzeroSamples: 400000, Coverage: 1.0,
			PeakDBFS: peak, RMSDBFS: rms, ApproximateLUFS: lufs, CrestDB: crest}
	}
	return TimefxEvidenceArtifact{
		SchemaVersion: TimefxEvidenceSchema, PairID: "tfx2_abc123",
		EvidenceRef:     "vit://dad.timefx_dual_tap/track:1007/t=0..480000@tfx2_abc123#-",
		AnalyzerVersion: "dad.timefx_dual_tap_analyzer.v1",
		ProcessorScope: TimefxProcessorScope{TrackID: "1007", PluginInstanceID: "1021", Family: "reverb",
			TopologyClass: "decay_time_reverb", TopologyGeneration: "gen-1",
			ChainHash: "chainhash-1", ProcessorStateHash: "statehash-1", ScopeRevision: "scope-1"},
		Conditions: TimefxConditions{SourceRevision: "source-1", ClipRevision: "clip-1", RenderRevision: "render-1",
			StartSample: 0, EndSample: 480000, SampleRate: 48000, ChannelCount: 2, ChannelLayout: "stereo",
			RenderMode: "offline_probe", Deterministic: true,
			BaselineTap: TimefxBaselineStage, ProcessedTap: TimefxProcessedStage, TailPolicy: "same_window_pair",
			LatencyCompensated: true},
		Determinism: TimefxDeterminismProof{Status: StatusReady, Correlation: 0.9999, RepeatCount: 2},
		TapMetrics: TimefxTapMetricsPair{
			Baseline:  metrics(-6.0, -18.0, -18.691, 12.0),
			Processed: metrics(-5.5, -15.0, -15.691, 9.5),
		},
		EffectDelta: TimefxArtifactDelta{RMSDeltaDB: 3.0, LUFSDeltaDB: 3.0, PeakDeltaDB: 0.5, CrestDeltaDB: -2.5},
	}
}

func TestInputFromTimefxEvidenceBuildsProjectionWithRealDeltas(t *testing.T) {
	artifact := timefxTestArtifact()
	input, err := InputFromTimefxEvidence(artifact)
	if err != nil {
		t.Fatalf("input assembly failed: %v", err)
	}
	if input.Baseline.Stage != TimefxBaselineStage || input.Processed.Stage != TimefxProcessedStage {
		t.Fatalf("tap stages = %q/%q", input.Baseline.Stage, input.Processed.Stage)
	}
	projection := Build(input)
	if projection.Status != StatusReady {
		// latency_compensated=true here, so ready is expected; anything less
		// means an unexpected limitation leaked in.
		t.Fatalf("status = %q want %q (limitations=%v)", projection.Status, StatusReady, projection.Limitations)
	}
	if !projection.TrustQuality.SameSource || !projection.TrustQuality.SameWindow || !projection.TrustQuality.SameFormat {
		t.Fatalf("identity gates dropped: %+v", projection.TrustQuality)
	}
	if !TimefxDeltaIsMeaningful(projection.EffectDelta) {
		t.Fatalf("delta not meaningful: %+v", projection.EffectDelta)
	}
	if got := *projection.EffectDelta.RMSDB; got != 3.0 {
		t.Fatalf("rms delta = %v want 3.0", got)
	}
	if *projection.EffectDelta.LUFSDB == 0 || *projection.EffectDelta.PeakDB == 0 {
		t.Fatalf("lufs/peak deltas must be non-zero for a wet reverb pair: %+v", projection.EffectDelta)
	}
	if *input.Baseline.RMSDBFS == *input.Processed.RMSDBFS {
		t.Fatalf("bypass equals processed: no differential observed")
	}
}

func TestInputFromTimefxEvidenceRejectsUnfaithfulArtifacts(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*TimefxEvidenceArtifact)
	}{
		{"schema_mismatch", func(a *TimefxEvidenceArtifact) { a.SchemaVersion = "dad.compressor_dual_tap_evidence.v1" }},
		{"family_unsupported", func(a *TimefxEvidenceArtifact) { a.ProcessorScope.Family = "eq" }},
		{"determinism_failed", func(a *TimefxEvidenceArtifact) { a.Determinism.Status = "suspect" }},
		{"determinism_missing", func(a *TimefxEvidenceArtifact) { a.Determinism.Status = "" }},
		{"zero_signal", func(a *TimefxEvidenceArtifact) { a.TapMetrics.Processed.NonzeroSamples = 0 }},
		{"nan_inf", func(a *TimefxEvidenceArtifact) { a.TapMetrics.Baseline.NaNInfSamples = 2 }},
		{"tap_order_swapped", func(a *TimefxEvidenceArtifact) {
			a.Conditions.BaselineTap, a.Conditions.ProcessedTap = TimefxProcessedStage, TimefxBaselineStage
		}},
		{"tail_policy_invalid", func(a *TimefxEvidenceArtifact) { a.Conditions.TailPolicy = "exact_window_no_tail" }},
		{"evidence_ref_wrong_kind", func(a *TimefxEvidenceArtifact) {
			a.EvidenceRef = "vit://dad.compressor_dual_tap/track:1007/t=0..480000@tfx2_abc123#-"
		}},
		{"pair_identity_invalid", func(a *TimefxEvidenceArtifact) {
			a.EvidenceRef = "vit://dad.timefx_dual_tap/track:1007/t=0..480000@other_pair#-"
		}},
	}
	for _, testCase := range cases {
		artifact := timefxTestArtifact()
		testCase.mutate(&artifact)
		if _, err := InputFromTimefxEvidence(artifact); err == nil {
			t.Fatalf("%s: assembly must fail closed", testCase.name)
		} else if !strings.Contains(err.Error(), "timefx_") {
			t.Fatalf("%s: rejection reason vocabulary = %q", testCase.name, err.Error())
		}
	}
}

func TestDecodeTimefxEvidenceArtifactRejectsUnknownFields(t *testing.T) {
	artifact := timefxTestArtifact()
	raw, err := json.Marshal(artifact)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	tampered := strings.Replace(string(raw), `{"schema_version"`, `{"surprise_field":1,"schema_version"}`, 1)
	if _, err := DecodeTimefxEvidenceArtifact([]byte(tampered)); err == nil {
		t.Fatalf("unknown fields must be rejected")
	}
	if _, err := DecodeTimefxEvidenceArtifact(raw); err != nil {
		t.Fatalf("faithful artifact must decode: %v", err)
	}
}

// TestTimefxDifferentialGateBlocksIdentityMismatch pins the ③ contract:
// same-source / same-window / same-route (format) — any disagreement between
// the taps must produce stale status with the matching blocked reason, never
// ready, and the projection must not support observation.
func TestTimefxDifferentialGateBlocksIdentityMismatch(t *testing.T) {
	base := func() Input {
		input, err := InputFromTimefxEvidence(timefxTestArtifact())
		if err != nil {
			t.Fatalf("input assembly failed: %v", err)
		}
		return input
	}

	sourceMismatch := base()
	sourceMismatch.Processed.SourceRevision = "source-2"
	sourceMismatch.Processed.Window.SourceRevision = "source-2"
	projection := Build(sourceMismatch)
	if projection.Status != StatusStale {
		t.Fatalf("source mismatch status = %q want stale", projection.Status)
	}
	if !containsReason(projection.TrustQuality.BlockedReasons, "source_revision_mismatch") {
		t.Fatalf("blocked reasons = %v", projection.TrustQuality.BlockedReasons)
	}
	if projection.TrustQuality.SameSource {
		t.Fatalf("same_source must not survive a source revision mismatch")
	}
	if projection.TrustQuality.CanSupportActionPreflight {
		t.Fatalf("stale projection must not support action preflight")
	}

	windowMismatch := base()
	windowMismatch.Processed.Window.StartSeconds = 1.0
	windowMismatch.Processed.Window.EndSeconds = 11.0
	projection = Build(windowMismatch)
	if projection.Status != StatusStale || !containsReason(projection.TrustQuality.BlockedReasons, "measurement_window_mismatch") {
		t.Fatalf("window mismatch status=%q reasons=%v", projection.Status, projection.TrustQuality.BlockedReasons)
	}

	formatMismatch := base()
	formatMismatch.Processed.Window.SampleRate = 44100
	projection = Build(formatMismatch)
	if projection.Status != StatusStale || !containsReason(projection.TrustQuality.BlockedReasons, "sample_format_mismatch") {
		t.Fatalf("format mismatch status=%q reasons=%v", projection.Status, projection.TrustQuality.BlockedReasons)
	}

	routingMismatch := base()
	routingMismatch.Processed.ChainStateHash = "statehash-other"
	projection = Build(routingMismatch)
	if projection.BaselineRef.ChainStateHash != "statehash-1" || projection.ProcessedRef.ChainStateHash != "statehash-other" {
		t.Fatalf("chain state hashes must be disclosed per tap for routing audits")
	}
}

// TestTimefxRepeatVarianceDowngradesTrust pins the modulated-family rule: a
// partial determinism proof (Pro-R style repeat-render variance) keeps the
// pair measurable, drops the projection to partial, and removes preflight
// support while observation support survives.
func TestTimefxRepeatVarianceDowngradesTrust(t *testing.T) {
	artifact := timefxTestArtifact()
	artifact.Determinism.Status = StatusPartial
	artifact.Determinism.Correlation = 0.981
	input, err := InputFromTimefxEvidence(artifact)
	if err != nil {
		t.Fatalf("input assembly failed: %v", err)
	}
	if input.Processed.Quality.Deterministic {
		t.Fatalf("processed tap must not claim determinism under a partial proof")
	}
	projection := Build(input)
	if projection.Status != StatusPartial {
		t.Fatalf("status = %q want partial", projection.Status)
	}
	if projection.TrustQuality.Deterministic {
		t.Fatalf("trust deterministic must be false")
	}
	if projection.TrustQuality.CanSupportActionPreflight {
		t.Fatalf("partial determinism must not support action preflight")
	}
	if !projection.TrustQuality.CanSupportObservation {
		t.Fatalf("observation support survives repeat variance")
	}
	if !TimefxDeltaIsMeaningful(projection.EffectDelta) {
		t.Fatalf("delta stays meaningful under a partial proof")
	}
}

// TestTimefxLatencyDeclarationLimitsTrust pins the honesty rule: a pair whose
// plugin reports nonzero latency is not latency compensated, the projection
// drops out of ready, and the limitation is disclosed.
func TestTimefxLatencyDeclarationLimitsTrust(t *testing.T) {
	artifact := timefxTestArtifact()
	artifact.Conditions.LatencyCompensated = false
	artifact.Conditions.ReportedLatencySamps = 32
	input, err := InputFromTimefxEvidence(artifact)
	if err != nil {
		t.Fatalf("input assembly failed: %v", err)
	}
	projection := Build(input)
	if projection.Status != StatusPartial {
		t.Fatalf("status = %q want partial", projection.Status)
	}
	if projection.TrustQuality.LatencyCompensated {
		t.Fatalf("latency compensated must be false")
	}
	if projection.TrustQuality.CanSupportActionPreflight {
		t.Fatalf("uncompensated pair must not support action preflight")
	}
	if len(projection.Limitations) == 0 {
		t.Fatalf("the latency limitation must be disclosed")
	}
	if !projection.TrustQuality.CanSupportObservation {
		t.Fatalf("observation support survives the latency limitation")
	}
}

func containsReason(reasons []string, reason string) bool {
	for _, value := range reasons {
		if value == reason {
			return true
		}
	}
	return false
}
