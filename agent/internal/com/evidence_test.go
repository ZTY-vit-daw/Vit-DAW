package com

import (
	"encoding/json"
	"strings"
	"testing"
)

func readyPairedEvidenceReceipt() PairedEvidenceReceipt {
	return PairedEvidenceReceipt{
		Command: "compressor_dual_tap_probe_ready", FeatureFamily: "audio_feature",
		FeatureType: "compressor_dual_tap_probe", SchemaVersion: PairedEvidenceSchemaVersion,
		Status: StatusReady, QualityStatus: StatusReady, Reason: "ok", JobID: "job_1", PairID: "com2_pair_1",
		TrackID: "track_1", ClipID: "clip_1", PluginInstanceID: "plugin_1", PluginPosition: "track_slot:0/rack:r1/node:1,1",
		TopologyClass: "threshold_driven", TopologyGeneration: "topology_1", SupportClass: "single_band_broadband",
		ChainHash: "chain_1", ProcessorStateHash: "state_1", ScopeRevision: "scope_1",
		SourceRevision: "source_1", ClipRevision: "clip_revision_1", RenderRevision: "render_2",
		StartSample: 48000, EndSample: 240000, SampleRate: 48000, ChannelCount: 2, ChannelLayout: "stereo",
		RenderMode: "offline_probe", Deterministic: true, InputTap: "compressor_input", OutputTap: "compressor_output",
		TailPolicy: "exact_window_no_tail", AnalyzerVersion: "dad.compressor_dual_tap_analyzer.v1",
		EvidenceRef: "dad.compressor_dual_tap:com2_pair_1", AlignedSampleFrames: 191963,
		EnvelopeFrameCount: 2998, EventCandidateCount: 8, ArtifactSHA256: strings.Repeat("a", 64), ArtifactBytes: 12000,
		DeterminismProofStatus: StatusReady, DeterminismMaxAbsDelta: 0, DeterminismRMSDelta: 0,
		DeterminismCorrelation: 1, DeterminismPeakDBDelta: 0, DeterminismRMSDBDelta: 0,
		LatencyAlignment: EvidenceLatencyAlignment{Status: StatusReady, Method: "offline_pdc_plus_integer_cross_correlation_v1",
			PluginReportedLatencySamples: 64, MeasuredOffsetSamples: 37, AppliedOffsetSamples: 37, ResidualErrorSamples: 0, Correlation: 0.91},
		QualityEvidence: PairedQualityEvidence{
			Input:  TapQualityEvidence{SampleFrames: 192000, NonzeroSamples: 300000, Coverage: 1, PeakDBFS: -3, RMSDBFS: -18},
			Output: TapQualityEvidence{SampleFrames: 192000, NonzeroSamples: 300000, Coverage: 1, PeakDBFS: -6, RMSDBFS: -20},
		},
	}
}

func TestValidatePairedEvidenceReceiptHappyPath(t *testing.T) {
	receipt := readyPairedEvidenceReceipt()
	validation := ValidatePairedEvidenceReceipt(receipt, PairedEvidenceExpectation{
		TrackID: "track_1", ClipID: "clip_1", PluginInstanceID: "plugin_1", TopologyGeneration: "topology_1",
		ScopeRevision: "scope_1", SourceRevision: "source_1", ClipRevision: "clip_revision_1",
		StartSample: 48000, EndSample: 240000, SampleRate: 48000, ChannelCount: 2,
		PriorPairID: "com2_pair_0", PriorJobID: "job_0",
	})
	if !validation.Ready || len(validation.Reasons) != 0 {
		t.Fatalf("validation = %#v", validation)
	}
}

func TestValidatePairedEvidenceReceiptRejectsIndependentGateMutations(t *testing.T) {
	tests := map[string]func(*PairedEvidenceReceipt){
		"source revision":  func(r *PairedEvidenceReceipt) { r.SourceRevision = "source_other" },
		"sample window":    func(r *PairedEvidenceReceipt) { r.EndSample++ },
		"format":           func(r *PairedEvidenceReceipt) { r.ChannelCount = 1 },
		"tap order":        func(r *PairedEvidenceReceipt) { r.InputTap, r.OutputTap = r.OutputTap, r.InputTap },
		"scope":            func(r *PairedEvidenceReceipt) { r.ScopeRevision = "scope_other" },
		"unsupported":      func(r *PairedEvidenceReceipt) { r.SupportClass = "multiband" },
		"nondeterministic": func(r *PairedEvidenceReceipt) { r.Deterministic = false },
		"alignment":        func(r *PairedEvidenceReceipt) { r.LatencyAlignment.ResidualErrorSamples = 4 },
		"quality":          func(r *PairedEvidenceReceipt) { r.QualityEvidence.Input.NaNInfSamples = 1 },
		"artifact":         func(r *PairedEvidenceReceipt) { r.ArtifactSHA256 = "bad" },
		"stale pair": func(r *PairedEvidenceReceipt) {
			r.PairID = "com2_pair_0"
			r.EvidenceRef = "dad.compressor_dual_tap:com2_pair_0"
		},
		"stale job": func(r *PairedEvidenceReceipt) { r.JobID = "job_0" },
	}
	expected := PairedEvidenceExpectation{TrackID: "track_1", ClipID: "clip_1", PluginInstanceID: "plugin_1",
		TopologyGeneration: "topology_1", ScopeRevision: "scope_1", SourceRevision: "source_1", ClipRevision: "clip_revision_1",
		StartSample: 48000, EndSample: 240000, SampleRate: 48000, ChannelCount: 2,
		PriorPairID: "com2_pair_0", PriorJobID: "job_0"}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			receipt := readyPairedEvidenceReceipt()
			mutate(&receipt)
			if validation := ValidatePairedEvidenceReceipt(receipt, expected); validation.Ready || len(validation.Reasons) == 0 {
				t.Fatalf("mutation promoted receipt: %#v", validation)
			}
		})
	}
}

func TestValidatePairedEvidenceReceiptAllowsStableRenderRevisionAcrossCaptures(t *testing.T) {
	receipt := readyPairedEvidenceReceipt()
	validation := ValidatePairedEvidenceReceipt(receipt, PairedEvidenceExpectation{
		PriorPairID: "com2_pair_0", PriorJobID: "job_0",
	})
	if !validation.Ready {
		t.Fatalf("stable render identity rejected for a fresh capture: %#v", validation)
	}
}

func TestDecodePairedEvidenceReceiptRejectsRawEvidenceAndPaths(t *testing.T) {
	receipt := readyPairedEvidenceReceipt()
	raw, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodePairedEvidenceReceipt(raw); err != nil {
		t.Fatalf("clean receipt rejected: %v", err)
	}
	for _, fragment := range []string{
		`{"schema_version":"dad.compressor_dual_tap_receipt.v1","aligned_envelope_frames":[{}]}`,
		`{"schema_version":"dad.compressor_dual_tap_receipt.v1","note":"D:\\tmp\\input.wav"}`,
		`{"schema_version":"dad.compressor_dual_tap_receipt.v1","render_file_path":"/tmp/input.wav"}`,
	} {
		if _, err := DecodePairedEvidenceReceipt([]byte(fragment)); err == nil || !strings.Contains(err.Error(), "raw_evidence_leak") {
			t.Fatalf("raw payload was not rejected: %s err=%v", fragment, err)
		}
	}
}
