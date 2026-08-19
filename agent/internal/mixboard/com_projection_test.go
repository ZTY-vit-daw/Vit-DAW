package mixboard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/com"
)

func TestObservationConnectsCOMSourceOnlyCatalogReadContextAndPersistence(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", root)
	store := NewStore("")
	result, err := store.RequestObservation(Request{
		MixSessionID: "mix-com-source", TargetRef: TargetRef{Kind: "track", ID: "1007"},
		Args: map[string]any{"com_mode": com.ModeSourceOnly, "feature_snapshot": comSourceFeatureSnapshot("ready")},
	})
	if err != nil {
		t.Fatal(err)
	}
	projection := result.Observation.COMProjection
	if projection == nil || projection.Mode != com.ModeSourceOnly || projection.Status != com.StatusReady {
		t.Fatalf("COM source projection = %+v", projection)
	}
	if projection.ObservationID != result.Observation.ObservationID || projection.MixSessionID != "mix-com-source" {
		t.Fatalf("COM observation identity = %+v", projection)
	}
	if !projection.LLMContext.DoNotIncludeRawPackage || len(projection.LLMContext.CompactFacts) > 24 {
		t.Fatalf("COM context contract = %+v", projection.LLMContext)
	}
	if !hasCatalogEntry(result.Observation.Catalog, "observation.com_projection", "fresh") {
		t.Fatalf("COM catalog entry missing: %+v", result.Observation.Catalog.Entries)
	}
	if result.ContextPack.LatestObservation["com_projection"] == nil || result.ContextPack.LatestObservation["compression_observation_context"] == nil {
		t.Fatalf("COM context pack missing: %+v", result.ContextPack.LatestObservation)
	}
	read, err := store.Read(ReadRequest{MixSessionID: "mix-com-source", ObservationID: result.Observation.ObservationID,
		Keys: []string{"observation.com_projection"}})
	if err != nil {
		t.Fatal(err)
	}
	item := mapValue(mapValue(read["items"])["observation.com_projection"])
	if cleanAnyString(item["projection_id"]) != projection.ProjectionID || cleanAnyString(item["mode"]) != com.ModeSourceOnly {
		t.Fatalf("persisted COM read = %+v", item)
	}
	assertNoCOMRawLeak(t, result.ObservationPath)
	assertNoCOMRawLeak(t, result.ContextPackPath)
}

func TestObservationConnectsCOMPairedAndChangeDeltaWithoutArtifactPathLeak(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", root)
	beforePath := writeCOMArtifact(t, root, comProductArtifact("pair-before", "state-before", "render-before", 0))
	afterPath := writeCOMArtifact(t, root, comProductArtifact("pair-after", "state-after", "render-after", .8))
	store := NewStore("")

	paired, err := store.RequestObservation(Request{MixSessionID: "mix-com-paired", TargetRef: TargetRef{Kind: "track", ID: "1007"},
		Args: map[string]any{"com_mode": com.ModePairedIO, "com_artifact_path": beforePath}})
	if err != nil {
		t.Fatal(err)
	}
	if paired.Observation.COMProjection == nil || paired.Observation.COMProjection.Mode != com.ModePairedIO || paired.Observation.COMProjection.Status == com.StatusMissing {
		t.Fatalf("paired COM projection = %+v", paired.Observation.COMProjection)
	}
	if !hasCatalogEntry(paired.Observation.Catalog, "observation.com_projection", comProjectionFreshness(paired.Observation)) {
		t.Fatal("paired COM catalog entry missing")
	}

	delta, err := store.RequestObservation(Request{MixSessionID: "mix-com-change", TargetRef: TargetRef{Kind: "track", ID: "1007"},
		Args: map[string]any{"com_mode": com.ModeChangeDelta, "com_before_artifact_path": beforePath, "com_after_artifact_path": afterPath}})
	if err != nil {
		t.Fatal(err)
	}
	projection := delta.Observation.COMProjection
	if projection == nil || projection.Mode != com.ModeChangeDelta || projection.Status == com.StatusMissing || projection.BehaviorChange == nil {
		t.Fatalf("change COM projection = %+v", projection)
	}
	if projection.BehaviorChange.BeforeProjectionID == "" || projection.BehaviorChange.AfterProjectionID == "" {
		t.Fatalf("change child IDs missing: %+v", projection.BehaviorChange)
	}
	for _, path := range []string{paired.ObservationPath, paired.ContextPackPath, delta.ObservationPath, delta.ContextPackPath} {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		text := string(data)
		if strings.Contains(text, beforePath) || strings.Contains(text, afterPath) {
			t.Fatalf("COM artifact path leaked into %s", path)
		}
		assertNoCOMRawLeak(t, path)
	}
}

func TestCOMFreshnessFailsClosedForStaleSourceAndUnchangedChangeState(t *testing.T) {
	stale := BuildObservation(Request{MixSessionID: "mix-com-stale", TargetRef: TargetRef{Kind: "track", ID: "1007"},
		Args: map[string]any{"com_mode": com.ModeSourceOnly, "feature_snapshot": comSourceFeatureSnapshot("stale")}}, "2026-08-04T00:00:00Z")
	if stale.COMProjection == nil || stale.COMProjection.Status != com.StatusStale || !hasCatalogEntry(stale.Catalog, "observation.com_projection", "stale") {
		t.Fatalf("stale COM identity = %+v catalog=%+v", stale.COMProjection, stale.Catalog.Entries)
	}

	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", root)
	artifact := comProductArtifact("pair-same", "state-same", "render-same", 0)
	before := writeCOMArtifact(t, root, artifact)
	artifact.PairID = "pair-repeat"
	artifact.EvidenceRef = "dad.compressor_dual_tap:pair-repeat"
	after := writeCOMArtifact(t, root, artifact)
	unchanged := BuildObservation(Request{MixSessionID: "mix-com-unchanged", TargetRef: TargetRef{Kind: "track", ID: "1007"},
		Args: map[string]any{"com_mode": com.ModeChangeDelta, "com_before_artifact_path": before, "com_after_artifact_path": after}}, "2026-08-04T00:00:00Z")
	if unchanged.COMProjection == nil || unchanged.COMProjection.Status != com.StatusStale || comProjectionFreshness(unchanged) != "stale" {
		t.Fatalf("unchanged change state did not fail stale: %+v", unchanged.COMProjection)
	}
	for _, dimension := range unchanged.COMProjection.BehaviorChange.Dimensions {
		if dimension.Classification != com.ChangeNonComparable {
			t.Fatalf("stale dimension became comparable: %+v", dimension)
		}
	}
}

func comSourceFeatureSnapshot(status string) map[string]any {
	segments := []any{}
	for i, rms := range []float64{.05, .08, .04, .07} {
		segments = append(segments, map[string]any{"start_seconds": float64(i), "end_seconds": float64(i + 1),
			"rms": rms, "peak_abs": rms * 3, "rms_dbfs": -26.0 + float64(i), "peak_dbfs": -16.0 + float64(i), "crest_db": 10.0, "energy_state": "active"})
	}
	return map[string]any{"schema_version": "mixboard_feature_snapshot.v1", "waveform_envelope": map[string]any{
		"schema_version": "dad.waveform_envelope.v1", "status": status, "freshness": status,
		"track_id": "1007", "clip_id": "2001", "source_revision": "source-1", "clip_revision": "clip-1",
		"sample_rate": 48000.0, "channel_count": 2, "channel_layout": "stereo", "duration_seconds": 4.0,
		"analyzed_sample_count": 192000, "nonzero_count": 1000, "coverage_ratio": 1.0,
		"rms": .06, "peak_abs": .24, "rms_dbfs": -24.0, "peak_dbfs": -12.0, "crest_db": 12.0,
		"quality_status": status, "evidence_ref": "dad:waveform:1007:source-1", "analyzer_version": "dad.waveform.v1",
		"time_segments": segments,
	}}
}

func comSourceFeatureSnapshotWithTransients(status string) map[string]any {
	snapshot := comSourceFeatureSnapshot(status)
	snapshot["transient_events"] = map[string]any{
		"status": "ready", "coverage": 1.0, "window_ms": 85.33, "hop_ms": 85.33,
		"evidence_refs": []any{"dad.l3.transient_events"},
		"events": []any{
			map[string]any{"onset_seconds": 0.5, "body_end_seconds": 0.58, "sustain_end_seconds": 0.85, "onset_dbfs": -8.0, "body_dbfs": -15.0, "sustain_dbfs": -20.0, "attack_body_contrast_db": 7.0, "sustain_decay_db": 5.0},
			map[string]any{"onset_seconds": 1.5, "body_end_seconds": 1.58, "sustain_end_seconds": 1.85, "onset_dbfs": -10.0, "body_dbfs": -17.0, "sustain_dbfs": -22.0, "attack_body_contrast_db": 7.0, "sustain_decay_db": 5.0},
			map[string]any{"onset_seconds": 2.5, "body_end_seconds": 2.58, "sustain_end_seconds": 2.85, "onset_dbfs": -6.0, "body_dbfs": -14.0, "sustain_dbfs": -19.0, "attack_body_contrast_db": 8.0, "sustain_decay_db": 5.0},
		},
	}
	return snapshot
}

func TestCOMSourceOnlyConsumesDADTransientEvents(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", root)
	store := NewStore("")
	result, err := store.RequestObservation(Request{
		MixSessionID: "mix-com-transient", TargetRef: TargetRef{Kind: "track", ID: "1007"},
		Args: map[string]any{"com_mode": com.ModeSourceOnly, "feature_snapshot": comSourceFeatureSnapshotWithTransients("ready")},
	})
	if err != nil {
		t.Fatal(err)
	}
	projection := result.Observation.COMProjection
	if projection == nil || projection.SourceDynamics == nil {
		t.Fatalf("COM projection = %+v", projection)
	}
	events := projection.SourceDynamics.Events
	if events.EventCount != 3 {
		t.Fatalf("micro transient event count = %d, want 3", events.EventCount)
	}
	scaleFound := false
	for _, scale := range projection.SourceDynamics.TimeScaleCoverage {
		if scale.Scale != com.ScaleMicroTransient {
			continue
		}
		scaleFound = true
		if scale.Status != com.StatusPartial || scale.WindowMS != 85.33 || scale.HopMS != 85.33 || scale.SegmentCount != 3 {
			t.Fatalf("micro transient scale = %+v", scale)
		}
	}
	if !scaleFound {
		t.Fatal("micro transient scale missing from coverage")
	}
	assertNoCOMRawLeak(t, result.ObservationPath)
}

func TestCOMSourceOnlyWithoutTransientEvidenceKeepsMacroOnly(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", root)
	store := NewStore("")
	result, err := store.RequestObservation(Request{
		MixSessionID: "mix-com-no-transient", TargetRef: TargetRef{Kind: "track", ID: "1007"},
		Args: map[string]any{"com_mode": com.ModeSourceOnly, "feature_snapshot": comSourceFeatureSnapshot("ready")},
	})
	if err != nil {
		t.Fatal(err)
	}
	projection := result.Observation.COMProjection
	if projection == nil || projection.SourceDynamics == nil {
		t.Fatalf("COM projection = %+v", projection)
	}
	if projection.SourceDynamics.Events.EventCount != 0 {
		t.Fatalf("transient events fabricated without evidence: %+v", projection.SourceDynamics.Events)
	}
	for _, scale := range projection.SourceDynamics.TimeScaleCoverage {
		if scale.Scale == com.ScaleMicroTransient && scale.Status != com.StatusMissing {
			t.Fatalf("micro transient promoted without evidence: %+v", scale)
		}
	}
}

func comProductArtifact(pairID, stateHash, renderRevision string, outputOffset float64) com.PairedEvidenceArtifact {
	frames := make([]com.PairedEnvelopeFrame, 80)
	for i := range frames {
		input := -30.0
		if i%10 < 4 {
			input = -9.0
		}
		output := input - outputOffset
		frames[i] = com.PairedEnvelopeFrame{StartSample: int64(i * 64), EndSample: int64(i*64 + 128),
			InputPeakDBFS: []float64{input, input}, InputRMSDBFS: []float64{input - 3, input - 3},
			OutputPeakDBFS: []float64{output, output}, OutputRMSDBFS: []float64{output - 3, output - 3}}
	}
	return com.PairedEvidenceArtifact{
		SchemaVersion: "dad.compressor_dual_tap_evidence.v1", PairID: pairID,
		EvidenceRef: "dad.compressor_dual_tap:" + pairID, AnalyzerVersion: "dad.compressor_dual_tap_analyzer.v1",
		ProcessorScope: com.PairedProcessorScope{TrackID: "1007", PluginInstanceID: "1021", PluginPosition: "track_slot:0",
			TopologyClass: "amount_driven", TopologyGeneration: "topology-1", SupportClass: "single_band_broadband",
			ChainHash: "chain-1", ProcessorStateHash: stateHash, ScopeRevision: "scope-" + stateHash},
		Conditions: com.PairedConditions{SourceRevision: "source-1", ClipRevision: "clip-1", RenderRevision: renderRevision,
			StartSample: 0, EndSample: 5184, SampleRate: 48000, ChannelCount: 2, ChannelLayout: "stereo",
			RenderMode: "offline_probe", Deterministic: true, InputTap: "compressor_input", OutputTap: "compressor_output",
			TailPolicy: "exact_window_no_tail", FrameSizeSamples: 128, HopSizeSamples: 64},
		LatencyAlignment: com.EvidenceLatencyAlignment{Status: "ready", Method: "offline_pdc_plus_integer_cross_correlation_v1", Correlation: .99},
		DeterminismProof: com.PairedDeterminismProof{Status: "ready", Correlation: 1, PeakDBDelta: 0, RMSDBDelta: 0},
		QualityEvidence: com.PairedQualityEvidence{
			Input:  com.TapQualityEvidence{SampleFrames: 5184, NonzeroSamples: 1000, Coverage: 1, PeakDBFS: -9, RMSDBFS: -20},
			Output: com.TapQualityEvidence{SampleFrames: 5184, NonzeroSamples: 1000, Coverage: 1, PeakDBFS: -9 - outputOffset, RMSDBFS: -20 - outputOffset}},
		AlignedEnvelopeFrames: frames,
	}
}

func writeCOMArtifact(t *testing.T, root string, artifact com.PairedEvidenceArtifact) string {
	t.Helper()
	data, err := json.Marshal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, artifact.PairID+".json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func hasCatalogEntry(catalog Catalog, key, freshness string) bool {
	for _, entry := range catalog.Entries {
		if entry.Key == key && entry.Freshness == freshness {
			return true
		}
	}
	return false
}

func assertNoCOMRawLeak(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(data))
	for _, forbidden := range []string{"aligned_envelope_frames", "input_event_candidates", "raw_samples", "render_file_path", "shared_memory"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("%s leaked into %s", forbidden, path)
		}
	}
}
