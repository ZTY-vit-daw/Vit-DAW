package harness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/com"
	"vit-daw-agent/internal/mixboard"
)

func TestPrepareMixObservationCOMEvidenceCapturesAndValidatesCurrentPair(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_DAW_DEV_ROOT", root)
	pairID := "com2_live_pair"
	artifactPath := filepath.Join(root, "VitApp", "Workspace", "Artifacts", "com_evidence", pairID, pairID+".json")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, []byte(`{"schema_version":"dad.compressor_dual_tap_evidence.v1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	receipt := validCOMReceipt(pairID)
	receiptMap := map[string]any{}
	data, _ := json.Marshal(receipt)
	_ = json.Unmarshal(data, &receiptMap)
	var sent map[string]any
	h := &Harness{comProbeCollect: func(_ context.Context, command map[string]any, _, _, _ string) (map[string]any, map[string]any, error) {
		sent = command
		return receiptMap, map[string]any{"status": "ok", "pair_id": pairID}, nil
	}}
	out, capture, err := h.prepareMixObservationCOMEvidence(context.Background(), map[string]any{
		"com_mode": com.ModePairedIO, "track_id": "1007", "plugin_id": "1021", "clip_id": "2001",
		"topology_class": "amount_driven", "topology_generation": "topology-1", "start_sample": 0, "end_sample": 480000,
	}, mixboard.TargetRef{Kind: "track", ID: "1007"}, map[string]any{"track_id": "1007", "clip_id": "2001"})
	if err != nil {
		t.Fatal(err)
	}
	if firstString(sent, "cmd") != "compressor_dual_tap_probe" || !boolValueDefault(sent["deterministic"], false) {
		t.Fatalf("kernel COM command = %+v", sent)
	}
	if firstString(out, "com_artifact_path") != artifactPath || firstString(capture, "pair_id") != pairID || firstString(capture, "evidence_ref") != receipt.EvidenceRef {
		t.Fatalf("COM capture result out=%+v capture=%+v", out, capture)
	}
	if _, leaked := capture["com_artifact_path"]; leaked {
		t.Fatalf("capture response leaked artifact path: %+v", capture)
	}
}

func TestPrepareMixObservationCOMEvidenceRejectsIncompleteLiveScope(t *testing.T) {
	h := &Harness{}
	_, _, err := h.prepareMixObservationCOMEvidence(context.Background(), map[string]any{
		"com_mode": com.ModePairedIO, "track_id": "1007", "plugin_id": "1021",
	}, mixboard.TargetRef{}, nil)
	if err == nil {
		t.Fatal("incomplete live COM scope was accepted")
	}
}

func TestReadyPairedCOMCaptureCanSurviveGeneralObservationGateLag(t *testing.T) {
	cmd := map[string]any{"com_mode": com.ModePairedIO}
	capture := map[string]any{
		"status": com.StatusReady, "pair_id": "pair-1",
		"artifact_sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	if !mixObservationHasReadyPairedCOMCapture(cmd, capture) {
		t.Fatal("validated paired COM capture was not admitted past general gate lag")
	}
	delete(capture, "artifact_sha256")
	if mixObservationHasReadyPairedCOMCapture(cmd, capture) {
		t.Fatal("incomplete paired COM capture bypassed the general gate")
	}
}

func TestPrepareMixObservationCOMEvidenceResolvesBoundedSemanticProductWindow(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_DAW_DEV_ROOT", root)
	pairID := "com2_product_window"
	artifactPath := filepath.Join(root, "VitApp", "Workspace", "Artifacts", "com_evidence", pairID, pairID+".json")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, []byte(`{"schema_version":"dad.compressor_dual_tap_evidence.v1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	receipt := validCOMReceipt(pairID)
	receipt.StartSample, receipt.EndSample = 1440000, 1920000
	receipt.QualityEvidence.Input.SampleFrames = 480000
	receipt.QualityEvidence.Output.SampleFrames = 480000
	receiptMap := map[string]any{}
	data, _ := json.Marshal(receipt)
	_ = json.Unmarshal(data, &receiptMap)
	var sent map[string]any
	h := &Harness{comProbeCollect: func(_ context.Context, command map[string]any, _, _, _ string) (map[string]any, map[string]any, error) {
		sent = command
		return receiptMap, map[string]any{"status": "ok", "pair_id": pairID}, nil
	}}
	out, capture, err := h.prepareMixObservationCOMEvidence(context.Background(), map[string]any{
		"com_mode": com.ModePairedIO, "track_id": "1007", "plugin_id": "1021",
		"topology_class": "amount_driven", "topology_generation": "topology-1",
		"resolve_semantic_compressor_window": true, "current_playhead_seconds": 30.0,
	}, mixboard.TargetRef{Kind: "track", ID: "1007"}, map[string]any{
		"track_id": "1007", "clip_id": "2001", "clip_start_seconds": 20.0, "duration_seconds": 60.0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, hasSamples := sent["start_sample"]; hasSamples || numberFromAny(sent["start_seconds"]) != 30 || numberFromAny(sent["end_seconds"]) != 40 {
		t.Fatalf("product window was not sent as bounded seconds: %+v", sent)
	}
	if int64(numberFromAny(out["start_sample"])) != receipt.StartSample || int64(numberFromAny(out["end_sample"])) != receipt.EndSample ||
		firstString(out, "clip_id") != receipt.ClipID {
		t.Fatalf("authoritative receipt window was not adopted: %+v", out)
	}
	if firstString(capture, "window_source") != "resolved_clip_default_10s" || numberFromAny(capture["sample_rate"]) != 48000 ||
		int(numberFromAny(capture["channel_count"])) != 2 {
		t.Fatalf("capture context = %+v", capture)
	}
}

func TestSemanticCompressorObservationSecondsPrefersExplicitSelection(t *testing.T) {
	start, end, source, err := semanticCompressorObservationSeconds(map[string]any{
		"selected_clip_time_range": map[string]any{"active": true, "start_seconds": 12.0, "end_seconds": 18.0},
		"current_playhead_seconds": 25.0,
	}, map[string]any{"clip_start_seconds": 10.0, "duration_seconds": 30.0})
	if err != nil || start != 12 || end != 18 || source != "selected_clip" {
		t.Fatalf("window=(%v,%v,%q) err=%v", start, end, source, err)
	}
}

func TestSemanticCompressorObservationSecondsDoesNotTreatSelectedClipBoundsAsExplicitSelection(t *testing.T) {
	start, end, source, err := semanticCompressorObservationSeconds(map[string]any{
		"selected_clip_time_range": map[string]any{
			"active": true, "start_seconds": 10.0, "end_seconds": 229.0, "source": "selected_clip",
		},
		"current_playhead_seconds": 25.0,
	}, map[string]any{"clip_start_seconds": 10.0, "duration_seconds": 219.0})
	if err != nil || start != 25 || end != 35 || source != "resolved_clip_default_10s" {
		t.Fatalf("window=(%v,%v,%q) err=%v", start, end, source, err)
	}
}

func TestPrepareMixObservationCOMEvidenceKeepsGeneralPairedBoundaryStrict(t *testing.T) {
	h := &Harness{}
	_, _, err := h.prepareMixObservationCOMEvidence(context.Background(), map[string]any{
		"com_mode": com.ModePairedIO, "track_id": "1007", "plugin_id": "1021", "clip_id": "2001",
		"topology_class": "amount_driven", "topology_generation": "topology-1",
	}, mixboard.TargetRef{}, map[string]any{"track_id": "1007", "clip_id": "2001", "duration_seconds": 10.0})
	if err == nil || !strings.Contains(err.Error(), "exact start_sample/end_sample") {
		t.Fatalf("general paired_io boundary was widened: %v", err)
	}
}

func TestMixObserveCOMSourceOnlyKeepsExplicitReadyEvidence(t *testing.T) {
	t.Setenv("VIT_MIXBOARD_ROOT", t.TempDir())
	h := New(nil, nil, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{Tool: "mix.observe", Args: map[string]any{
		"mix_session_id": "com-source-explicit", "scope": "selected_track", "track_id": "1007",
		"com_mode": com.ModeSourceOnly, "feature_snapshot": map[string]any{
			"schema_version": "mixboard_feature_snapshot.v1",
			"waveform_envelope": map[string]any{
				"schema_version": "dad.com.source_projection_input.v1", "feature_type": "waveform_envelope",
				"status": "ready", "freshness": "fresh", "track_id": "1007", "clip_id": "2001",
				"source_revision": "source-1", "clip_revision": "clip-1", "sample_rate": 48000.0,
				"channel_count": 2, "channel_layout": "stereo", "duration_seconds": 2.0,
				"analyzed_sample_count": 96000, "nonzero_count": 1000, "coverage_ratio": 1.0,
				"rms_dbfs": -24.0, "peak_dbfs": -8.0, "crest_db": 16.0, "quality_status": "ready",
				"evidence_ref": "dad:com-source:1007:source-1", "analyzer_version": "dad.com.source.v1",
				"time_segments": []any{map[string]any{"start_seconds": 0.0, "end_seconds": 2.0,
					"rms_dbfs": -24.0, "peak_dbfs": -8.0, "crest_db": 16.0, "energy_state": "active"}},
			},
		},
	}})
	if err != nil || resp.Status != "ok" {
		t.Fatalf("mix.observe = status=%q result=%+v err=%v", resp.Status, resp.Result, err)
	}
	projection, ok := resp.Result["com_projection"].(*com.Projection)
	if !ok || projection == nil || projection.Status != com.StatusReady || !projection.TrustQuality.CanSupportSourceDescription {
		t.Fatalf("explicit source projection was downgraded: %#v", resp.Result["com_projection"])
	}
	if _, ok := resp.Result["acoustic_package_status"]; ok {
		t.Fatalf("weaker acoustic package status escaped explicit COM source path: %+v", resp.Result)
	}
}

func validCOMReceipt(pairID string) com.PairedEvidenceReceipt {
	return com.PairedEvidenceReceipt{
		Command: "compressor_dual_tap_probe_ready", FeatureFamily: "audio_feature", FeatureType: "compressor_dual_tap_probe",
		SchemaVersion: com.PairedEvidenceSchemaVersion, Status: "ready", QualityStatus: "ready", Reason: "ok",
		JobID: "job-1", RequestID: "request-1", PairID: pairID, TrackID: "1007", ClipID: "2001", PluginInstanceID: "1021",
		PluginPosition: "track_slot:0", TopologyClass: "amount_driven", TopologyGeneration: "topology-1", SupportClass: "single_band_broadband",
		ChainHash: "chain-1", ProcessorStateHash: "state-1", ScopeRevision: "scope-1", SourceRevision: "source-1", ClipRevision: "clip-1", RenderRevision: "render-1",
		StartSample: 0, EndSample: 480000, SampleRate: 48000, ChannelCount: 2, ChannelLayout: "stereo", RenderMode: "offline_probe", Deterministic: true,
		InputTap: "compressor_input", OutputTap: "compressor_output", TailPolicy: "exact_window_no_tail", AnalyzerVersion: "dad.compressor_dual_tap_analyzer.v1",
		EvidenceRef: "dad.compressor_dual_tap:" + pairID, AlignedSampleFrames: 480000, EnvelopeFrameCount: 100, EventCandidateCount: 10,
		ArtifactSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ArtifactBytes: 128,
		DeterminismProofStatus: "ready", DeterminismCorrelation: 1, DeterminismPeakDBDelta: 0, DeterminismRMSDBDelta: 0,
		LatencyAlignment: com.EvidenceLatencyAlignment{Status: "ready", Method: "offline_pdc_plus_integer_cross_correlation_v1", Correlation: .99},
		QualityEvidence: com.PairedQualityEvidence{
			Input:  com.TapQualityEvidence{SampleFrames: 480000, NonzeroSamples: 100, Coverage: 1, PeakDBFS: -1, RMSDBFS: -20},
			Output: com.TapQualityEvidence{SampleFrames: 480000, NonzeroSamples: 100, Coverage: 1, PeakDBFS: -2, RMSDBFS: -21}},
	}
}
