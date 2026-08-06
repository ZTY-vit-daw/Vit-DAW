package com

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestChangeDeltaClassifiesTypedBehaviorChangesAndPreservesChildren(t *testing.T) {
	before, after := comparableChangeChildren(t)
	after.GainAction = cloneBehavior(before.GainAction)
	after.GainAction.Facts["reduction_depth_db_distribution"] = map[string]any{"p50": 8.0, "p90": 12.0}
	after.GainAction.Facts["gain_action_duty_cycle"] = .75
	after.TransientResponse = cloneBehavior(before.TransientResponse)
	after.TransientResponse.Facts["transient_to_body_contrast_change_db"] = map[string]any{"p50": -3.0}
	after.TransientResponse.Facts["leading_edge_action_db"] = map[string]any{"p50": 5.0}
	after.RecoveryMotion = cloneBehavior(before.RecoveryMotion)
	after.RecoveryMotion.Facts["complete_recovery_ratio"] = .2
	after.RecoveryMotion.Facts["return_time_band_counts"] = map[string]int{"500_to_1000_ms": 4}
	after.LevelEffect = cloneBehavior(before.LevelEffect)
	after.LevelEffect.Facts["rms_delta_db"] = 7.0
	after.LevelEffect.Facts["peak_delta_db"] = 5.0
	after.StereoBehavior = cloneBehavior(before.StereoBehavior)
	after.StereoBehavior.Facts["action_difference_p90_db"] = 3.0
	rehashProjection(&after)

	delta := buildDelta(t, before, after)
	if delta.Status != StatusReady || delta.BehaviorChange == nil || !delta.TrustQuality.CanSupportPostActionEvaluation {
		t.Fatalf("ready delta = status %s behavior %+v trust %+v", delta.Status, delta.BehaviorChange, delta.TrustQuality)
	}
	if delta.BehaviorChange.BeforeProjectionID != before.ProjectionID || delta.BehaviorChange.AfterProjectionID != after.ProjectionID {
		t.Fatalf("child identities lost: %+v", delta.BehaviorChange)
	}
	for _, dimension := range []string{DimensionGainAction, DimensionTransientResponse, DimensionRecoveryMotion, DimensionLevelEffect, DimensionStereoBehavior} {
		if got := changeFor(delta, dimension); got.Classification != ChangeStronger {
			t.Fatalf("%s change = %+v", dimension, got)
		}
	}
	if trigger := changeFor(delta, DimensionTriggerRelation); trigger.Classification != ChangeNonComparable || trigger.Status != StatusMissing {
		t.Fatalf("trigger overclaimed: %+v", trigger)
	}
	assertChangeNoParameterInference(t, delta)
}

func TestChangeDeltaDistinguishesWeakerUnchangedMixedAndUnavailable(t *testing.T) {
	t.Run("weaker", func(t *testing.T) {
		before, after := comparableChangeChildren(t)
		after.GainAction = cloneBehavior(before.GainAction)
		after.GainAction.Facts["reduction_depth_db_distribution"] = map[string]any{"p50": 0.0, "p90": 0.0}
		after.GainAction.Facts["gain_action_duty_cycle"] = .05
		rehashProjection(&after)
		if got := changeFor(buildDelta(t, before, after), DimensionGainAction); got.Classification != ChangeWeaker {
			t.Fatalf("gain = %+v", got)
		}
	})
	t.Run("unchanged", func(t *testing.T) {
		before, after := comparableChangeChildren(t)
		if got := changeFor(buildDelta(t, before, after), DimensionGainAction).Classification; got != ChangeUnchanged {
			t.Fatalf("gain = %s", got)
		}
	})
	t.Run("mixed", func(t *testing.T) {
		before, after := comparableChangeChildren(t)
		after.GainAction = cloneBehavior(before.GainAction)
		after.GainAction.Facts["reduction_depth_db_distribution"] = map[string]any{"p50": 8.0, "p90": 12.0}
		after.GainAction.Facts["gain_action_duty_cycle"] = .01
		rehashProjection(&after)
		if got := changeFor(buildDelta(t, before, after), DimensionGainAction).Classification; got != ChangeMixed {
			t.Fatalf("gain = %s", got)
		}
	})
	t.Run("unavailable child dimension", func(t *testing.T) {
		before, after := comparableChangeChildren(t)
		after.RecoveryMotion = unavailableBehavior("fixture_missing", after.EvidenceRefs[0])
		rehashProjection(&after)
		delta := buildDelta(t, before, after)
		if got := changeFor(delta, DimensionRecoveryMotion); got.Classification != ChangeNonComparable || got.Status != StatusMissing {
			t.Fatalf("missing recovery became zero: %+v", got)
		}
		if delta.Status != StatusPartial {
			t.Fatalf("top level ignored missing dimension: %s", delta.Status)
		}
	})
}

func TestChangeDeltaComparabilityMutationsNeverBecomeZeroEffect(t *testing.T) {
	mutations := map[string]func(*Projection, *Projection){
		"identical projection": func(b, a *Projection) { a.ProjectionID = b.ProjectionID },
		"unchanged state":      func(b, a *Projection) { a.ProcessorScope.ProcessorStateHash = b.ProcessorScope.ProcessorStateHash },
		"unchanged render":     func(b, a *Projection) { a.Conditions.RenderRevision = b.Conditions.RenderRevision },
		"source":               func(b, a *Projection) { a.Conditions.SourceRevision = "other" },
		"clip":                 func(b, a *Projection) { a.Conditions.ClipRevision = "other" },
		"window":               func(b, a *Projection) { a.Conditions.EndSample++ },
		"sample rate":          func(b, a *Projection) { a.Conditions.SampleRate++ },
		"layout":               func(b, a *Projection) { a.Conditions.ChannelLayout = "other" },
		"tap":                  func(b, a *Projection) { a.Conditions.InputTap = "other" },
		"render mode":          func(b, a *Projection) { a.Conditions.RenderMode = "realtime" },
		"latency":              func(b, a *Projection) { a.Conditions.LatencyMethod = "other" },
		"analyzer":             func(b, a *Projection) { a.Conditions.AnalyzerVersion = "other" },
		"resolution":           func(b, a *Projection) { a.Conditions.AnalysisResolutions[0].HopMS++ },
		"processor":            func(b, a *Projection) { a.ProcessorScope.PluginInstanceID = "other" },
		"topology":             func(b, a *Projection) { a.ProcessorScope.TopologyGeneration = "other" },
		"chain":                func(b, a *Projection) { a.ProcessorScope.ChainHash = "other" },
		"input equivalence":    func(b, a *Projection) { a.EvidenceInputs.InputTrace.SHA256 = strings.Repeat("f", 64) },
		"child trust":          func(b, a *Projection) { a.Status = StatusSuspect },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			before, after := comparableChangeChildren(t)
			mutate(&before, &after)
			delta := buildDelta(t, before, after)
			if delta.Status == StatusReady || delta.TrustQuality.CanSupportPostActionEvaluation {
				t.Fatalf("failed comparability promoted delta: %+v", delta.TrustQuality)
			}
			for _, change := range delta.BehaviorChange.Dimensions {
				if change.Classification != ChangeNonComparable {
					t.Fatalf("%s became %s instead of non-comparable", change.Dimension, change.Classification)
				}
			}
		})
	}
}

func TestChangeDeltaInputEquivalenceRequiresCompactTraceFingerprint(t *testing.T) {
	before, after := comparableChangeChildren(t)
	before.EvidenceInputs.InputTrace = InputTraceIdentity{}
	after.EvidenceInputs.InputTrace = InputTraceIdentity{}
	delta := buildDelta(t, before, after)
	if delta.Status != StatusSuspect || delta.TrustQuality.InputEquivalent || !contains(delta.TrustQuality.BlockedReasons, "input_trace_not_equivalent") {
		t.Fatalf("missing input fingerprint promoted attribution: %+v", delta.TrustQuality)
	}
}

func TestChangeDeltaStableAndCompactContextPreservesTypedChanges(t *testing.T) {
	before, after := comparableChangeChildren(t)
	d1 := Build(Input{Mode: ModeChangeDelta, Change: &ChangeDeltaInput{Before: &before, After: &after}, CreatedAt: "2026-08-04T00:00:00Z"})
	d2 := Build(Input{Mode: ModeChangeDelta, Change: &ChangeDeltaInput{Before: &before, After: &after}, CreatedAt: "2027-01-01T00:00:00Z"})
	if d1.ProjectionID != d2.ProjectionID {
		t.Fatalf("delta ID unstable: %s != %s", d1.ProjectionID, d2.ProjectionID)
	}
	context := ContextProjection(d1)
	raw := strings.ToLower(string(mustJSON(t, context)))
	for _, forbidden := range []string{"aligned_envelope_frames", "input_event_candidates", "raw_samples", "frame_trace", "gain_trace", "\"threshold_db\":"} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("change context leaked %q", forbidden)
		}
	}
	if len(d1.LLMContext.CompactFacts) > 24 || !d1.LLMContext.DoNotIncludeRawPackage || d1.BehaviorChange == nil {
		t.Fatalf("change compact context invalid: %+v", d1.LLMContext)
	}
}

func comparableChangeChildren(t *testing.T) (Projection, Projection) {
	t.Helper()
	beforeArtifact := pairedFixture(fixtureOptions{makeupDB: 2, action: decayingAction(.12), peakAction: leadingEdgeAction(3)})
	beforeArtifact.PairID = "before_pair"
	beforeArtifact.EvidenceRef = "dad.compressor_dual_tap:before_pair"
	beforeArtifact.ProcessorScope.ProcessorStateHash = "state_before"
	beforeArtifact.ProcessorScope.ScopeRevision = "scope_before"
	beforeArtifact.Conditions.RenderRevision = "render_before"
	afterArtifact := beforeArtifact
	afterArtifact.PairID = "after_pair"
	afterArtifact.EvidenceRef = "dad.compressor_dual_tap:after_pair"
	afterArtifact.ProcessorScope.ProcessorStateHash = "state_after"
	afterArtifact.ProcessorScope.ScopeRevision = "scope_after"
	afterArtifact.Conditions.RenderRevision = "render_after"
	before := Build(Input{Mode: ModePairedIO, Paired: &beforeArtifact, CreatedAt: "2026-08-04T00:00:00Z"})
	after := Build(Input{Mode: ModePairedIO, Paired: &afterArtifact, CreatedAt: "2026-08-04T00:00:00Z"})
	if before.Status != StatusReady || after.Status != StatusReady {
		t.Fatalf("fixture children not ready: %s %s", before.Status, after.Status)
	}
	if before.EvidenceInputs.InputTrace.SHA256 == "" || before.EvidenceInputs.InputTrace.SHA256 != after.EvidenceInputs.InputTrace.SHA256 {
		t.Fatalf("fixture inputs not equivalent")
	}
	return before, after
}

func buildDelta(t *testing.T, before, after Projection) Projection {
	t.Helper()
	return Build(Input{Mode: ModeChangeDelta, Change: &ChangeDeltaInput{Before: &before, After: &after}, CreatedAt: "2026-08-04T00:00:00Z"})
}

func cloneBehavior(value *BehaviorProjection) *BehaviorProjection {
	if value == nil {
		return nil
	}
	raw, _ := json.Marshal(value)
	var out BehaviorProjection
	_ = json.Unmarshal(raw, &out)
	return &out
}

func rehashProjection(p *Projection) { p.ProjectionID = stableProjectionID(*p) }
func changeFor(p Projection, dimension string) BehaviorDimensionChange {
	if p.BehaviorChange != nil {
		for _, change := range p.BehaviorChange.Dimensions {
			if change.Dimension == dimension {
				return change
			}
		}
	}
	return BehaviorDimensionChange{}
}
func assertChangeNoParameterInference(t *testing.T, p Projection) {
	t.Helper()
	raw := strings.ToLower(string(mustJSON(t, p)))
	for _, key := range []string{"\"threshold_db\":", "\"ratio\":", "\"attack_ms\":", "\"release_ms\":", "\"makeup_gain_db\":", "\"channel_link\":"} {
		if strings.Contains(raw, key) {
			t.Fatalf("change inferred parameter %s", key)
		}
	}
}
