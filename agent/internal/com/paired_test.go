package com

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fixtureOptions struct {
	channelCount int
	hopSamples   int
	makeupDB     float64
	action       func(sample int64, channel int) float64
	peakAction   func(sample int64, channel int) float64
	parallel     bool
}

func TestPairedGainActionSeparatesFixedGainAndMakeupCompensation(t *testing.T) {
	fixed := pairedFixture(fixtureOptions{makeupDB: 6})
	p := Build(Input{Mode: ModePairedIO, Paired: &fixed, CreatedAt: "2026-08-04T00:00:00Z"})
	if p.GainAction == nil || factString(p.GainAction, "classification") != "no_time_varying_action_observed" {
		t.Fatalf("fixed gain invented action: %+v", p.GainAction)
	}
	if math.Abs(factFloat(p.GainAction, "steady_gain_baseline_db")-6) > .05 {
		t.Fatalf("fixed gain baseline = %+v", p.GainAction.Facts)
	}

	compressed := pairedFixture(fixtureOptions{makeupDB: 6, action: programAction})
	p = Build(Input{Mode: ModePairedIO, Paired: &compressed, CreatedAt: "2026-08-04T00:00:00Z"})
	if factString(p.GainAction, "classification") != "time_varying_gain_action_observed" || factFloat(p.GainAction, "gain_action_duty_cycle") <= .1 {
		t.Fatalf("makeup-compensated action not found: %+v", p.GainAction)
	}
	if factString(p.GainAction, "event_action_consistency") != "consistent" {
		t.Fatalf("known repeated action consistency missing: %+v", p.GainAction)
	}
	if factFloat(p.LevelEffect, "rms_delta_db") <= 0 {
		t.Fatalf("fixture did not prove scalar makeup confounder: %+v", p.LevelEffect)
	}
	assertNoParameterInference(t, p)
}

func TestPairedGainActionRejectsNearFloorRatioArtifactsAndFindsSlowMotion(t *testing.T) {
	a := pairedFixture(fixtureOptions{action: programAction})
	for i := range a.AlignedEnvelopeFrames {
		if i%7 == 0 {
			a.AlignedEnvelopeFrames[i].InputPeakDBFS = repeat(-82, a.Conditions.ChannelCount)
			a.AlignedEnvelopeFrames[i].InputRMSDBFS = repeat(-89, a.Conditions.ChannelCount)
			a.AlignedEnvelopeFrames[i].OutputPeakDBFS = repeat(-35, a.Conditions.ChannelCount)
			a.AlignedEnvelopeFrames[i].OutputRMSDBFS = repeat(-40, a.Conditions.ChannelCount)
		}
	}
	p := Build(Input{Mode: ModePairedIO, Paired: &a})
	distribution, _ := p.GainAction.Facts["reduction_depth_db_distribution"].(map[string]any)
	if distribution["max"].(float64) > 7 {
		t.Fatalf("near-floor ratio artifact entered gain action: %+v", distribution)
	}

	slow := pairedFixture(fixtureOptions{action: func(sample int64, channel int) float64 {
		return 2 + 2*math.Sin(2*math.Pi*float64(sample)/48000/2)
	}})
	p = Build(Input{Mode: ModePairedIO, Paired: &slow})
	if factString(p.GainAction, "classification") != "time_varying_gain_action_observed" {
		t.Fatalf("sustained slow gain motion not observed: %+v", p.GainAction)
	}
}

func TestPairedTransientClassifiesRoundedPreservedAndMixed(t *testing.T) {
	tests := []struct {
		name string
		peak func(int64, int) float64
		want string
	}{
		{name: "rounded", peak: leadingEdgeAction(10), want: "more_rounded"},
		{name: "preserved", peak: func(sample int64, channel int) float64 { return 0 }, want: "more_preserved"},
		{name: "unchanged", peak: programAction, want: "unchanged_within_tolerance"},
		{name: "mixed", peak: alternatingLeadingEdgeAction(), want: "mixed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := pairedFixture(fixtureOptions{action: programAction, peakAction: tt.peak})
			p := Build(Input{Mode: ModePairedIO, Paired: &a})
			if p.TransientResponse == nil || factString(p.TransientResponse, "classification") != tt.want {
				t.Fatalf("transient = %+v", p.TransientResponse)
			}
			if containsJSONKey(mustJSON(t, p), "attack_ms") || containsJSONKey(mustJSON(t, p), "lookahead_ms") {
				t.Fatal("transient behavior inferred control time")
			}
		})
	}
}

func TestPairedRecoveryDistinguishesCompleteAndIncomplete(t *testing.T) {
	complete := pairedFixture(fixtureOptions{action: decayingAction(.12)})
	p := Build(Input{Mode: ModePairedIO, Paired: &complete})
	if got := factString(p.RecoveryMotion, "classification"); got != "recovery_observed" {
		t.Fatalf("complete recovery = %s %+v", got, p.RecoveryMotion)
	}
	candidate, _ := p.RecoveryMotion.Facts["periodic_modulation_candidate"].(map[string]any)
	if candidate["status"] != "candidate" || candidate["confidence"].(float64) > .75 || candidate["observed_relation"] == nil {
		t.Fatalf("bounded periodic candidate missing: %+v", candidate)
	}
	if _, ok := candidate["alternatives"]; !ok {
		t.Fatalf("periodic candidate omitted alternatives: %+v", candidate)
	}
	incomplete := pairedFixture(fixtureOptions{action: func(sample int64, channel int) float64 {
		if sample >= 3*48000 {
			return 0
		}
		return decayingAction(2.0)(sample, channel)
	}})
	p = Build(Input{Mode: ModePairedIO, Paired: &incomplete})
	if got := factString(p.RecoveryMotion, "classification"); got != "incomplete_before_next_event" {
		t.Fatalf("incomplete recovery = %s %+v", got, p.RecoveryMotion)
	}
	assertNoParameterInference(t, p)
}

func TestPairedRecoveryDoesNotCallSilentGapIncomplete(t *testing.T) {
	a := pairedFixture(fixtureOptions{action: func(sample int64, channel int) float64 {
		if sample >= 3*48000 {
			return 0
		}
		return decayingAction(2.0)(sample, channel)
	}})
	for i := range a.AlignedEnvelopeFrames {
		phase := a.AlignedEnvelopeFrames[i].StartSample % 48000
		if phase > 6000 {
			a.AlignedEnvelopeFrames[i].InputPeakDBFS = repeat(-160, a.Conditions.ChannelCount)
			a.AlignedEnvelopeFrames[i].InputRMSDBFS = repeat(-160, a.Conditions.ChannelCount)
		}
	}
	p := Build(Input{Mode: ModePairedIO, Paired: &a})
	if p.RecoveryMotion.Status != StatusMissing || dimensionStatus(p, DimensionRecoveryMotion) != NotIdentifiable {
		t.Fatalf("silent gap became incomplete recovery: %+v %+v", p.RecoveryMotion, p.Identifiability)
	}
	if p.Status != StatusPartial {
		t.Fatalf("top-level readiness ignored unavailable recovery: %s", p.Status)
	}
}

func TestRecoveryObservationCoverageAllowsSparseUnobservableWindowsOnly(t *testing.T) {
	if got := recoveryObservationCoverage(43, 1); got < .8 {
		t.Fatalf("high recovery coverage was rejected: %v", got)
	}
	if got := recoveryObservationCoverage(3, 1); got >= .8 {
		t.Fatalf("low recovery coverage was promoted: %v", got)
	}
	if got := recoveryObservationCoverage(0, 0); got != 0 {
		t.Fatalf("empty recovery coverage = %v", got)
	}
}

func TestPairedStereoAndMonoBoundaries(t *testing.T) {
	linked := pairedFixture(fixtureOptions{channelCount: 2, action: programAction})
	p := Build(Input{Mode: ModePairedIO, Paired: &linked})
	if factString(p.StereoBehavior, "classification") != "closely_coherent_action" {
		t.Fatalf("linked = %+v", p.StereoBehavior)
	}
	asymmetric := pairedFixture(fixtureOptions{channelCount: 2, action: func(sample int64, channel int) float64 {
		if channel == 0 {
			return programAction(sample, channel)
		}
		return .1 * programAction(sample, channel)
	}})
	p = Build(Input{Mode: ModePairedIO, Paired: &asymmetric})
	if factString(p.StereoBehavior, "classification") != "asymmetric_action" {
		t.Fatalf("asymmetric = %+v", p.StereoBehavior)
	}
	mono := pairedFixture(fixtureOptions{channelCount: 1, action: programAction})
	p = Build(Input{Mode: ModePairedIO, Paired: &mono})
	if p.StereoBehavior.Status != StatusNotApplicable || dimensionStatus(p, DimensionStereoBehavior) != NotApplicable {
		t.Fatalf("mono = %+v %+v", p.StereoBehavior, p.Identifiability)
	}
}

func TestPairedConfoundersAndQualityGatesDoNotPromoteFalseReady(t *testing.T) {
	t.Run("parallel", func(t *testing.T) {
		a := pairedFixture(fixtureOptions{action: programAction, parallel: true})
		p := Build(Input{Mode: ModePairedIO, Paired: &a, Derivation: PairedDerivationConstraints{ParallelBlendKnown: true, ParallelBlendActive: true}})
		if p.Status != StatusPartial || dimensionStatus(p, DimensionGainAction) != Bounded || p.GainAction.Status != StatusPartial {
			t.Fatalf("parallel confounder promoted: status=%s gain=%+v id=%s", p.Status, p.GainAction, dimensionStatus(p, DimensionGainAction))
		}
	})
	t.Run("insufficient resolution", func(t *testing.T) {
		a := pairedFixture(fixtureOptions{hopSamples: 512, action: programAction})
		p := Build(Input{Mode: ModePairedIO, Paired: &a})
		if p.TransientResponse.Status != StatusMissing || dimensionStatus(p, DimensionTransientResponse) != NotIdentifiable || p.Status != StatusPartial {
			t.Fatalf("coarse evidence promoted transient: %+v %+v", p.TransientResponse, p.TrustQuality)
		}
	})
	t.Run("silence", func(t *testing.T) {
		a := pairedFixture(fixtureOptions{})
		a.QualityEvidence.Input.NonzeroSamples = 0
		a.QualityEvidence.Input.RMSDBFS = -160
		for i := range a.AlignedEnvelopeFrames {
			a.AlignedEnvelopeFrames[i].InputPeakDBFS = repeat(-160, a.Conditions.ChannelCount)
			a.AlignedEnvelopeFrames[i].InputRMSDBFS = repeat(-160, a.Conditions.ChannelCount)
		}
		p := Build(Input{Mode: ModePairedIO, Paired: &a})
		if p.TrustQuality.CanSupportBehaviorObservation || dimensionStatus(p, DimensionGainAction) != NotIdentifiable || p.Status == StatusReady {
			t.Fatalf("silence promoted behavior: %+v %+v", p.TrustQuality, p.Identifiability)
		}
	})
	t.Run("scope mismatch", func(t *testing.T) {
		a := pairedFixture(fixtureOptions{action: programAction})
		a.ProcessorScope.SupportClass = "multiband"
		p := Build(Input{Mode: ModePairedIO, Paired: &a})
		if p.Status != StatusSuspect || p.TrustQuality.CanSupportBehaviorObservation {
			t.Fatalf("multiband artifact promoted: %+v", p.TrustQuality)
		}
	})
}

func TestPairedComparabilityGatesIndependentlyBlockBehaviorAttribution(t *testing.T) {
	mutations := map[string]func(*PairedEvidenceArtifact){
		"schema":   func(a *PairedEvidenceArtifact) { a.SchemaVersion = "wrong" },
		"pair":     func(a *PairedEvidenceArtifact) { a.EvidenceRef = "dad.compressor_dual_tap:other" },
		"target":   func(a *PairedEvidenceArtifact) { a.ProcessorScope.TrackID = "" },
		"scope":    func(a *PairedEvidenceArtifact) { a.ProcessorScope.ScopeRevision = "" },
		"revision": func(a *PairedEvidenceArtifact) { a.Conditions.SourceRevision = "" },
		"window":   func(a *PairedEvidenceArtifact) { a.Conditions.EndSample = a.Conditions.StartSample },
		"format":   func(a *PairedEvidenceArtifact) { a.Conditions.SampleRate = 0 },
		"tap order": func(a *PairedEvidenceArtifact) {
			a.Conditions.InputTap, a.Conditions.OutputTap = a.Conditions.OutputTap, a.Conditions.InputTap
		},
		"determinism": func(a *PairedEvidenceArtifact) { a.DeterminismProof.Correlation = .8 },
		"alignment":   func(a *PairedEvidenceArtifact) { a.LatencyAlignment.ResidualErrorSamples = 4 },
		"quality":     func(a *PairedEvidenceArtifact) { a.QualityEvidence.Input.NaNInfSamples = 1 },
		"trace":       func(a *PairedEvidenceArtifact) { a.AlignedEnvelopeFrames = nil },
		"trace gap": func(a *PairedEvidenceArtifact) {
			a.AlignedEnvelopeFrames[10].StartSample++
			a.AlignedEnvelopeFrames[10].EndSample++
		},
		"event out of range": func(a *PairedEvidenceArtifact) {
			a.InputEventCandidates[0].Sample = a.Conditions.EndSample
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			a := pairedFixture(fixtureOptions{action: programAction})
			mutate(&a)
			p := Build(Input{Mode: ModePairedIO, Paired: &a})
			if p.Status == StatusReady || p.TrustQuality.CanSupportBehaviorObservation {
				t.Fatalf("failed gate promoted behavior: status=%s trust=%+v", p.Status, p.TrustQuality)
			}
			for _, dimension := range []string{DimensionGainAction, DimensionTransientResponse, DimensionRecoveryMotion, DimensionLevelEffect, DimensionStereoBehavior} {
				if got := dimensionStatus(p, dimension); got == Identified || got == Bounded {
					t.Fatalf("failed gate promoted %s as %s", dimension, got)
				}
			}
		})
	}
}

func TestPairedExpectedIdentityMismatchIsStale(t *testing.T) {
	a := pairedFixture(fixtureOptions{action: programAction})
	expected := Input{Mode: ModePairedIO, Paired: &a,
		ProcessorScope: ProcessorScope{TrackID: "different", PluginInstanceID: a.ProcessorScope.PluginInstanceID},
		Conditions: Conditions{SourceRevision: a.Conditions.SourceRevision, ClipRevision: a.Conditions.ClipRevision,
			RenderRevision: a.Conditions.RenderRevision, StartSample: a.Conditions.StartSample, EndSample: a.Conditions.EndSample,
			SampleRate: a.Conditions.SampleRate, ChannelCount: a.Conditions.ChannelCount, ChannelLayout: a.Conditions.ChannelLayout,
			RenderMode: a.Conditions.RenderMode, AnalyzerVersion: a.AnalyzerVersion}}
	p := Build(expected)
	if p.Status != StatusStale || p.GainAction.Status != StatusStale || dimensionStatus(p, DimensionGainAction) != NotIdentifiable {
		t.Fatalf("identity mismatch did not propagate stale: status=%s gain=%+v trust=%+v", p.Status, p.GainAction, p.TrustQuality)
	}
}

func TestPairedMissingRevisionIsSuspectNotStale(t *testing.T) {
	a := pairedFixture(fixtureOptions{action: programAction})
	a.Conditions.SourceRevision = ""
	p := Build(Input{Mode: ModePairedIO, Paired: &a})
	if p.Status != StatusSuspect || p.TrustQuality.SameSource {
		t.Fatalf("missing revision misclassified: status=%s trust=%+v", p.Status, p.TrustQuality)
	}
}

func TestPairedProjectionIsDeterministicAndContextLeaksNoRawTrace(t *testing.T) {
	a := pairedFixture(fixtureOptions{action: programAction, peakAction: leadingEdgeAction(3)})
	originalFirst := a.AlignedEnvelopeFrames[0].StartSample
	p1 := Build(Input{Mode: ModePairedIO, Paired: &a, CreatedAt: "2026-08-04T00:00:00Z"})
	if a.AlignedEnvelopeFrames[0].StartSample != originalFirst {
		t.Fatal("Build mutated caller-owned paired artifact")
	}
	reverseFrames(a.AlignedEnvelopeFrames)
	reverseEvents(a.InputEventCandidates)
	p2 := Build(Input{Mode: ModePairedIO, Paired: &a, CreatedAt: "2027-01-01T00:00:00Z"})
	if p1.ProjectionID != p2.ProjectionID {
		t.Fatalf("paired projection is order/time unstable: %s != %s", p1.ProjectionID, p2.ProjectionID)
	}
	context := ContextProjection(p1)
	raw := strings.ToLower(string(mustJSON(t, context)))
	for _, forbidden := range []string{"aligned_envelope_frames", "input_event_candidates", "frame_trace", "gain_trace", "\"threshold_db\":", "\"ratio\":", "\"attack_ms\":", "\"release_ms\":"} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("context leaked %q", forbidden)
		}
	}
	if !p1.LLMContext.DoNotIncludeRawPackage || len(p1.LLMContext.CompactFacts) > 24 || !p1.TrustQuality.CanSupportSemanticPlanning {
		t.Fatalf("paired compact context contract failed: %+v", p1.LLMContext)
	}
}

func TestSelectPrimaryEventsUsesBoundedNonMaximumSuppression(t *testing.T) {
	const sampleRate = 48000.0
	events := []PairedInputEventCandidate{
		{Sample: 0, InputPeakDBFS: -12, Kind: "input_onset_candidate"},
		{Sample: 4800, InputPeakDBFS: -3, Kind: "input_onset_candidate"},
		{Sample: 9600, InputPeakDBFS: -9, Kind: "input_onset_candidate"},
		{Sample: 14400, InputPeakDBFS: -6, Kind: "input_onset_candidate"},
		{Sample: 24000, InputPeakDBFS: -4, Kind: "input_onset_candidate"},
	}
	selected := selectPrimaryEvents(events, sampleRate)
	if len(selected) != 3 {
		t.Fatalf("dense candidate chain collapsed incorrectly: %+v", selected)
	}
	if selected[0].Sample != 4800 || selected[0].InputPeakDBFS != -3 {
		t.Fatalf("local maximum was not retained: %+v", selected)
	}
	minimumGap := int64(.150 * sampleRate)
	for i := 1; i < len(selected); i++ {
		if selected[i].Sample-selected[i-1].Sample < minimumGap {
			t.Fatalf("selected events violate refractory interval: %+v", selected)
		}
	}
	reverseEvents(events)
	reversed := selectPrimaryEvents(events, sampleRate)
	if string(mustJSON(t, selected)) != string(mustJSON(t, reversed)) {
		t.Fatalf("event selection depends on input order: %+v != %+v", selected, reversed)
	}
}

func TestDecodeAndDeriveRealCOM2Artifact(t *testing.T) {
	path := filepath.Join("..", "..", "..", "VitApp", "Workspace", "Artifacts", "com_evidence", "com2_b22af9ec7b5d41e18043db06", "com2_b22af9ec7b5d41e18043db06.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("real COM-2 artifact unavailable: %v", err)
	}
	a, err := DecodePairedEvidenceArtifact(raw)
	if err != nil {
		t.Fatal(err)
	}
	p := Build(Input{Mode: ModePairedIO, Paired: &a, CreatedAt: "2026-08-04T00:00:00Z"})
	if (p.Status != StatusReady && p.Status != StatusPartial) || p.GainAction == nil || p.TransientResponse == nil || p.RecoveryMotion == nil || p.LevelEffect == nil || p.StereoBehavior == nil {
		t.Fatalf("real artifact derivation incomplete: status=%s gain=%+v transient=%+v recovery=%+v", p.Status, p.GainAction, p.TransientResponse, p.RecoveryMotion)
	}
	if p.SourceDynamics.Events.EventCount <= 0 || p.SourceDynamics.Events.TransientContrastDB == nil {
		t.Fatalf("real source event facts missing: %+v", p.SourceDynamics.Events)
	}
	if p.ProjectionID == "" || p.EvidenceRefs[0] != a.EvidenceRef {
		t.Fatalf("identity/evidence lost: %+v", p)
	}
	assertNoParameterInference(t, p)
}

func pairedFixture(options fixtureOptions) PairedEvidenceArtifact {
	if options.channelCount <= 0 {
		options.channelCount = 2
	}
	if options.hopSamples <= 0 {
		options.hopSamples = 64
	}
	const sampleRate = 48000.0
	const durationSamples = int64(4 * sampleRate)
	frameSize := options.hopSamples * 2
	eventSamples := []int64{0, 48000, 96000, 144000}
	frames := []PairedEnvelopeFrame{}
	for start := int64(0); start+int64(frameSize) <= durationSamples; start += int64(options.hopSamples) {
		phase := start % 48000
		inputRMS := -42.0
		inputPeak := -36.0
		if phase < 480 {
			inputRMS, inputPeak = -20, -5
		} else if phase < 9600 {
			inputRMS, inputPeak = -24, -15
		}
		inRMS, inPeak, outRMS, outPeak := []float64{}, []float64{}, []float64{}, []float64{}
		for channel := 0; channel < options.channelCount; channel++ {
			action := 0.0
			if options.action != nil {
				action = options.action(start, channel)
			}
			peakAction := action
			if options.peakAction != nil {
				peakAction = options.peakAction(start, channel)
			}
			outR := inputRMS + options.makeupDB - action
			outP := inputPeak + options.makeupDB - peakAction
			if options.parallel {
				outR = mixDB(inputRMS, outR, .5)
				outP = mixDB(inputPeak, outP, .5)
			}
			inRMS = append(inRMS, inputRMS)
			inPeak = append(inPeak, inputPeak)
			outRMS = append(outRMS, outR)
			outPeak = append(outPeak, outP)
		}
		frames = append(frames, PairedEnvelopeFrame{StartSample: start, EndSample: start + int64(frameSize), InputPeakDBFS: inPeak, InputRMSDBFS: inRMS, OutputPeakDBFS: outPeak, OutputRMSDBFS: outRMS})
	}
	events := make([]PairedInputEventCandidate, len(eventSamples))
	for i, sample := range eventSamples {
		events[i] = PairedInputEventCandidate{Sample: sample, InputPeakDBFS: -5, Kind: "input_onset_candidate"}
	}
	return PairedEvidenceArtifact{
		SchemaVersion: pairedArtifactSchema, PairID: "fixture_pair", EvidenceRef: "dad.compressor_dual_tap:fixture_pair", AnalyzerVersion: "dad.compressor_dual_tap_analyzer.v1",
		ProcessorScope:        PairedProcessorScope{TrackID: "1", PluginInstanceID: "2", PluginPosition: "track_slot:0", TopologyClass: "threshold_driven", TopologyGeneration: "topology_1", SupportClass: "single_band_broadband", ChainHash: "chain", ProcessorStateHash: "state", ScopeRevision: "scope"},
		Conditions:            PairedConditions{SourceRevision: "source", ClipRevision: "clip", RenderRevision: "render", StartSample: 0, EndSample: durationSamples, SampleRate: sampleRate, ChannelCount: options.channelCount, ChannelLayout: map[bool]string{true: "mono", false: "stereo"}[options.channelCount == 1], RenderMode: "offline_probe", Deterministic: true, InputTap: "compressor_input", OutputTap: "compressor_output", TailPolicy: "exact_window_no_tail", FrameSizeSamples: frameSize, HopSizeSamples: options.hopSamples},
		LatencyAlignment:      EvidenceLatencyAlignment{Status: StatusReady, Method: "offline_pdc_plus_integer_cross_correlation_v1", Correlation: .98, AlignedSampleFrames: durationSamples},
		DeterminismProof:      PairedDeterminismProof{Status: StatusReady, Method: "same_scope_repeat_render_envelope_tolerance_v1", RepeatCount: 2, Correlation: 1, CorrelationMinimum: .999, LevelDeltaToleranceDB: .1},
		QualityEvidence:       PairedQualityEvidence{Input: TapQualityEvidence{SampleFrames: durationSamples, NonzeroSamples: durationSamples, Coverage: 1, PeakDBFS: -5, RMSDBFS: -28}, Output: TapQualityEvidence{SampleFrames: durationSamples, NonzeroSamples: durationSamples, Coverage: 1, PeakDBFS: -3, RMSDBFS: -24}},
		AlignedEnvelopeFrames: frames, InputEventCandidates: events,
	}
}

func programAction(sample int64, channel int) float64 { return decayingAction(.25)(sample, channel) }

func decayingAction(seconds float64) func(int64, int) float64 {
	return func(sample int64, channel int) float64 {
		phase := float64(sample%48000) / 48000
		if phase > seconds {
			return 0
		}
		return 6 * (1 - phase/seconds)
	}
}

func leadingEdgeAction(db float64) func(int64, int) float64 {
	return func(sample int64, channel int) float64 {
		if sample%48000 < 480 {
			return db
		}
		return programAction(sample, channel)
	}
}

func alternatingLeadingEdgeAction() func(int64, int) float64 {
	return func(sample int64, channel int) float64 {
		event := (sample / 48000) % 2
		if sample%48000 < 480 {
			if event == 0 {
				return 10
			}
			return 0
		}
		return programAction(sample, channel)
	}
}

func mixDB(dry, wet, mix float64) float64 {
	return 20 * math.Log10((1-mix)*math.Pow(10, dry/20)+mix*math.Pow(10, wet/20))
}
func repeat(value float64, count int) []float64 {
	out := make([]float64, count)
	for i := range out {
		out[i] = value
	}
	return out
}
func factString(p *BehaviorProjection, key string) string {
	if p == nil {
		return ""
	}
	value, _ := p.Facts[key].(string)
	return value
}
func factFloat(p *BehaviorProjection, key string) float64 {
	if p == nil {
		return 0
	}
	value, _ := p.Facts[key].(float64)
	return value
}
func dimensionStatus(p Projection, name string) string {
	for _, d := range p.Identifiability.Dimensions {
		if d.Dimension == name {
			return d.Status
		}
	}
	return ""
}
func reverseFrames(values []PairedEnvelopeFrame) {
	for i, j := 0, len(values)-1; i < j; i, j = i+1, j-1 {
		values[i], values[j] = values[j], values[i]
	}
}
func reverseEvents(values []PairedInputEventCandidate) {
	for i, j := 0, len(values)-1; i < j; i, j = i+1, j-1 {
		values[i], values[j] = values[j], values[i]
	}
}
func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func containsJSONKey(raw []byte, key string) bool {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return false
	}
	_, ok := findJSONKey(value, key)
	return ok
}
func assertNoParameterInference(t *testing.T, p Projection) {
	t.Helper()
	raw := strings.ToLower(string(mustJSON(t, p)))
	for _, forbidden := range []string{"\"threshold_db\"", "\"ratio\"", "\"knee\"", "\"attack_ms\"", "\"release_ms\"", "\"lookahead_ms\"", "\"makeup_gain_db\"", "\"channel_link\""} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("projection inferred forbidden parameter %s", forbidden)
		}
	}
}
