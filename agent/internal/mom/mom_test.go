package mom

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGeneralProjectionPrefersL3AndKeepsL2StatusOnly(t *testing.T) {
	proj := Build(testInput(map[string]any{}))

	if proj.Intent != IntentGeneralBandStereoObservation {
		t.Fatalf("intent = %q", proj.Intent)
	}
	if got := proj.Layers.TimbreFrequency.Facts["primary_layer"]; got != "l3_deep" {
		t.Fatalf("primary layer = %v", got)
	}
	statusOnly := mapValue(proj.Layers.TimbreFrequency.Facts["l2_realtime_status"])
	if text(statusOnly["note"]) != "L2 realtime available but not requested" {
		t.Fatalf("L2 status-only note missing: %#v", proj.Layers.TimbreFrequency.Facts)
	}
	if _, ok := statusOnly["bands"]; ok {
		t.Fatalf("general projection expanded L2 band values: %#v", statusOnly)
	}
	if proj.TrustQuality.OverallStatus != StatusReady {
		t.Fatalf("overall status = %q, want ready for required L3 band/stereo layers", proj.TrustQuality.OverallStatus)
	}
	data, _ := json.Marshal(proj.LLMContext)
	llmText := string(data)
	for _, forbidden := range []string{"unit_energy", "balance_db", "correlation_estimate", "live_level_meter_spectrum", "live_level_meter_stereo"} {
		if strings.Contains(llmText, forbidden) {
			t.Fatalf("general LLM context leaked L2 value/source %s in %s", forbidden, llmText)
		}
	}
	if !proj.LLMContext.DoNotIncludeRawPackage {
		t.Fatalf("raw package flag not set")
	}
}

func TestRealtimeProjectionUsesL2AndReportsTapPoint(t *testing.T) {
	input := testInput(map[string]any{"requested_layer": "l2_realtime", "prefer_realtime": true})
	proj := Build(input)

	if proj.Intent != IntentRealtimeBandStereoObservation {
		t.Fatalf("intent = %q", proj.Intent)
	}
	if got := proj.Layers.SpaceStereo.Facts["primary_layer"]; got != "l2_realtime" {
		t.Fatalf("primary layer = %v", got)
	}
	if proj.TrustQuality.L2TapPoint != "unknown_live_meter" {
		t.Fatalf("tap point = %q", proj.TrustQuality.L2TapPoint)
	}
	if len(proj.TrustQuality.L2Limitations) == 0 {
		t.Fatalf("expected L2 limitation")
	}
	data, _ := json.Marshal(proj)
	text := string(data)
	for _, want := range []string{"live_level_meter_spectrum", "correlation_estimate", "l2_tap_point_not_fully_closed_loop"} {
		if !strings.Contains(text, want) {
			t.Fatalf("realtime projection missing %s in %s", want, text)
		}
	}
}

func TestRealtimeProjectionPrefersL2RenderProbeAndEvidenceRef(t *testing.T) {
	input := testInput(map[string]any{"requested_layer": "post_fader"})
	realtime := mapValue(input.MixPackage["realtime_metrics"])
	realtime["render_probe"] = map[string]any{
		"status":               "ready",
		"source":               "l2_render_probe",
		"layer":                "l2_realtime",
		"tap_point":            "track_post_fader",
		"render_mode":          "offline_probe",
		"track_id":             "track_1",
		"clip_id":              "clip_1",
		"source_revision":      "rev_1",
		"clip_revision":        "cliprev_1",
		"render_revision":      "render_probe_1",
		"evidence_ref":         "dad.l2_render_probe:render_probe_1",
		"duration_seconds":     12.0,
		"sample_rate":          48000.0,
		"channel_count":        2,
		"quality_evidence":     map[string]any{"nonzero": true, "sum_abs": 42.0, "max_abs": 0.5, "nan_inf_count": 0, "coverage": 1.0},
		"bands":                map[string]any{"bass": map[string]any{"energy_db": -6.0, "unit_energy": 0.5}},
		"balance_db":           0.1,
		"correlation_estimate": 0.96,
		"correlation_state":    "highly_correlated",
		"shared_memory":        "must_not_leak",
		"raw_samples":          []any{0.1, 0.2},
		"spectral_tiles":       []any{map[string]any{"bin": 1}},
		"render_file_path":     "D:\\tmp\\probe.wav",
	}
	input.MixPackage["realtime_metrics"] = realtime

	proj := Build(input)
	if proj.Intent != IntentRealtimeBandStereoObservation {
		t.Fatalf("intent = %q", proj.Intent)
	}
	if proj.Layers.TimbreFrequency.Source != "l2_realtime.render_probe" {
		t.Fatalf("timbre source = %q", proj.Layers.TimbreFrequency.Source)
	}
	if proj.Layers.SpaceStereo.Source != "l2_realtime.render_probe" {
		t.Fatalf("space source = %q", proj.Layers.SpaceStereo.Source)
	}
	band := mapValue(proj.Layers.TimbreFrequency.Facts["band_energy_summary"])
	if band["tap_point"] != "track_post_fader" || band["render_mode"] != "offline_probe" || band["render_revision"] != "render_probe_1" {
		t.Fatalf("render probe band facts missing identity: %#v", band)
	}
	if !containsString(proj.Layers.TimbreFrequency.EvidenceRefs, "dad.l2_render_probe:render_probe_1") {
		t.Fatalf("missing render probe evidence refs: %#v", proj.Layers.TimbreFrequency.EvidenceRefs)
	}
	if proj.TrustQuality.L2TapPoint != "track_post_fader" {
		t.Fatalf("tap point = %q", proj.TrustQuality.L2TapPoint)
	}
	data, _ := json.Marshal(proj.LLMContext)
	text := string(data)
	for _, forbidden := range []string{"must_not_leak", "raw_samples", "spectral_tiles", "render_file_path", "probe.wav"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("LLM context leaked raw render probe field %s in %s", forbidden, text)
		}
	}
}

func TestGeneralProjectionKeepsL2RenderProbeStatusOnly(t *testing.T) {
	input := testInput(map[string]any{})
	realtime := mapValue(input.MixPackage["realtime_metrics"])
	realtime["render_probe"] = map[string]any{
		"status":               "ready",
		"source":               "l2_render_probe",
		"layer":                "l2_realtime",
		"tap_point":            "track_post_fader",
		"render_mode":          "offline_probe",
		"track_id":             "track_1",
		"clip_id":              "clip_1",
		"render_revision":      "render_probe_1",
		"evidence_ref":         "dad.l2_render_probe:render_probe_1",
		"bands":                map[string]any{"bass": map[string]any{"energy_db": -6.0, "unit_energy": 0.5}},
		"correlation_estimate": 0.96,
		"shared_memory":        "must_not_leak",
	}
	input.MixPackage["realtime_metrics"] = realtime

	proj := Build(input)
	if proj.Intent != IntentGeneralBandStereoObservation {
		t.Fatalf("intent = %q", proj.Intent)
	}
	statusOnly := mapValue(proj.Layers.TimbreFrequency.Facts["l2_render_probe_status"])
	if statusOnly["status"] != "ready" || statusOnly["render_revision"] != "render_probe_1" {
		t.Fatalf("render probe status-only facts = %#v", statusOnly)
	}
	for _, forbiddenKey := range []string{"bands", "quality_evidence", "correlation_estimate", "shared_memory"} {
		if _, ok := statusOnly[forbiddenKey]; ok {
			t.Fatalf("general observation expanded L2 render probe key %s in %#v", forbiddenKey, statusOnly)
		}
	}
	data, _ := json.Marshal(proj.LLMContext)
	if strings.Contains(string(data), "must_not_leak") {
		t.Fatalf("general LLM context leaked raw render probe field: %s", string(data))
	}
}

func TestChineseGoalTextSelectsRealtimeAndActionPreflight(t *testing.T) {
	general := Build(testInput(map[string]any{"goal_text": "观察当前工程的频段和声像状态，不要修改。"}))
	if general.Intent != IntentGeneralBandStereoObservation {
		t.Fatalf("Chinese general observation intent = %q", general.Intent)
	}

	realtime := Build(testInput(map[string]any{"goal_text": "播放后检查 L2 实时频谱、电平和声像，不要修改。"}))
	if realtime.Intent != IntentRealtimeBandStereoObservation {
		t.Fatalf("Chinese realtime intent = %q", realtime.Intent)
	}

	multitrack := Build(testInput(map[string]any{"goal_text": "比较一下各轨频段占用和声像关系，不要修改。"}))
	if multitrack.Intent != IntentProjectMultitrackObservation {
		t.Fatalf("Chinese multitrack intent = %q", multitrack.Intent)
	}

	action := Build(testInput(map[string]any{"goal_text": "帮我把低频稍微收一点，但先告诉我依据。"}))
	if action.Intent != IntentActionPreflightObservation {
		t.Fatalf("Chinese action intent = %q", action.Intent)
	}
	ab := Build(testInput(map[string]any{"goal_text": "确认后检查 AB result。"}))
	if ab.Intent != IntentABResultObservation {
		t.Fatalf("Chinese AB result intent = %q", ab.Intent)
	}
	properEQ := Build(testInput(map[string]any{"goal_text": "轻微低频 EQ，可以进行处理。"}))
	if properEQ.Intent != IntentActionPreflightObservation {
		t.Fatalf("proper Chinese low EQ intent = %q", properEQ.Intent)
	}
	readOnly := Build(testInput(map[string]any{"goal_text": "只观察低频和声像，不要修改。"}))
	if readOnly.Intent != IntentGeneralBandStereoObservation {
		t.Fatalf("read-only Chinese intent = %q", readOnly.Intent)
	}
}

func TestGeneralOverallStatusIgnoresOptionalBasicEnergy(t *testing.T) {
	input := testInput(map[string]any{})
	current := mapValue(input.MixPackage["current_metrics"])
	current["waveform"] = map[string]any{"status": "missing", "reason": "waveform_not_required_for_general_band_stereo"}
	input.MixPackage["current_metrics"] = current

	proj := Build(input)
	if proj.TrustQuality.OverallStatus != StatusReady {
		t.Fatalf("overall status = %q, want ready when required L3 band/stereo layers are ready", proj.TrustQuality.OverallStatus)
	}
}

func TestTrustGateIgnoresABMissingForNonABActionPreflight(t *testing.T) {
	proj := Build(testInput(map[string]any{"goal_text": "帮我把低频收一点，但先告诉我依据。"}))

	if proj.Intent != IntentActionPreflightObservation {
		t.Fatalf("intent = %q", proj.Intent)
	}
	if !proj.TrustQuality.CanSupportSuggestion || !proj.TrustQuality.CanSupportActionPreflight {
		t.Fatalf("trust should support action preflight without AB result: %+v", proj.TrustQuality)
	}
	if containsString(proj.TrustQuality.MissingFields, "ab_result_comparison") {
		t.Fatalf("AB missing should not block non-AB action: %+v", proj.TrustQuality)
	}
}

func TestTrustGateIgnoresSingleTrackMultitrackNotApplicableForActionPreflight(t *testing.T) {
	input := testInput(map[string]any{"goal_text": "轻微低频 EQ"})
	project := mapValue(input.ProjectPackage)
	tracks := anySlice(project["tracks"])
	if len(tracks) > 1 {
		project["tracks"] = tracks[:1]
	}
	project["relationship_inputs"] = map[string]any{"status": "not_applicable_single_track", "track_count": 1}
	input.ProjectPackage = project

	proj := Build(input)
	if proj.MultitrackRelation.Status != "not_applicable_single_track" {
		t.Fatalf("relation = %+v", proj.MultitrackRelation)
	}
	if !proj.TrustQuality.CanSupportActionPreflight {
		t.Fatalf("single-track relation should not block action preflight: %+v", proj.TrustQuality)
	}
	if containsString(proj.TrustQuality.MissingFields, "multitrack_relation") {
		t.Fatalf("single-track not_applicable should not be a missing field: %+v", proj.TrustQuality)
	}
}

func TestProjectionDoesNotCarryRawAcousticPayload(t *testing.T) {
	proj := Build(testInput(map[string]any{}))
	data, err := json.Marshal(proj)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{`"time_segments":`, `"spectrogram_tile_rows":`, "raw.time_energy.range"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("raw payload leaked %s in %s", forbidden, text)
		}
	}
	for _, required := range []string{"mom_version", "intent_policy", "project_structure", "project_mix_profile", "multitrack_relation", "basic_energy", "timbre_frequency", "space_stereo", "trust_quality", "do_not_include_raw_package"} {
		if !strings.Contains(text, required) {
			t.Fatalf("projection missing %s in %s", required, text)
		}
	}
}

func TestLLMContextOmitsRawPackageWaveformAndTilePayloads(t *testing.T) {
	input := testInput(map[string]any{"goal_text": "观察当前工程的频段和声像状态，不要修改。"})
	current := mapValue(input.MixPackage["current_metrics"])
	current["raw_package"] = map[string]any{"secret": "raw_package_secret"}
	current["waveform"] = map[string]any{
		"status":          "ready",
		"rms_dbfs":        -18.0,
		"peak_dbfs":       -3.0,
		"raw_waveform":    []any{0.1, 0.2},
		"waveform_arrays": []any{0.3, 0.4},
		"time_segments":   []any{map[string]any{"start": 0, "rms": -18.0}},
	}
	current["band_energy"] = map[string]any{
		"status":                "ready",
		"source":                "spectral_tile_derived",
		"tile_payload":          "tile_payload_secret",
		"spectrogram_tiles":     []any{map[string]any{"bin": 1}},
		"spectrogram_tile_rows": []any{map[string]any{"tile": 1}},
		"bands": map[string]any{
			"bass": map[string]any{"energy_db": -12.0, "unit_energy": 0.25},
		},
	}
	current["stereo_relation"] = map[string]any{
		"status":               "ready",
		"source":               "spectral_tile_derived",
		"balance_db":           0.2,
		"correlation_estimate": 0.82,
		"shared_memory":        "shared_memory_secret",
		"render_file_path":     "D:/tmp/raw_probe.wav",
	}
	input.MixPackage["current_metrics"] = current
	proj := Build(input)

	for label, value := range map[string]any{
		"llm_context":        proj.LLMContext,
		"context_projection": ContextProjection(proj),
	} {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, forbidden := range []string{
			`"raw_package":`, "raw_package_secret", "raw_waveform", "waveform_arrays",
			`"time_segments":`, "tile_payload", "tile_payload_secret", "spectrogram_tiles",
			"spectrogram_tile_rows", "shared_memory_secret", "raw_probe.wav", "D:/tmp",
		} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s leaked raw payload marker %s in %s", label, forbidden, text)
			}
		}
	}
}

func TestActionPreflightUsesEvidenceAndRequiresConfirmation(t *testing.T) {
	proj := Build(testInput(map[string]any{"goal_text": "please lower the low end a little"}))

	if proj.Intent != IntentActionPreflightObservation {
		t.Fatalf("intent = %q", proj.Intent)
	}
	data, _ := json.Marshal(proj.LLMContext)
	text := string(data)
	for _, want := range []string{"Action preflight", "wait for confirmation", "evidence_refs"} {
		if !strings.Contains(text, want) {
			t.Fatalf("action preflight context missing %s in %s", want, text)
		}
	}
	if len(proj.TrustQuality.EvidenceRefs) == 0 {
		t.Fatalf("action preflight missing evidence refs")
	}
}

func TestProjectMultitrackProjectionBuildsCompactRelation(t *testing.T) {
	proj := Build(testInput(map[string]any{
		"goal_text": "比较一下各轨频段占用和声像关系，不要修改。",
	}))

	if proj.MOMVersion != Version {
		t.Fatalf("mom version = %q", proj.MOMVersion)
	}
	if proj.Intent != IntentProjectMultitrackObservation {
		t.Fatalf("intent = %q", proj.Intent)
	}
	if proj.IntentPolicy.Name != IntentProjectMultitrackObservation {
		t.Fatalf("intent policy = %#v", proj.IntentPolicy)
	}
	if proj.ProjectMixProfile.Status != StatusReady {
		t.Fatalf("project mix profile = %#v", proj.ProjectMixProfile)
	}
	if proj.MultitrackRelation.Status != StatusReady {
		t.Fatalf("multitrack relation = %#v", proj.MultitrackRelation)
	}
	if len(proj.MultitrackRelation.BandOccupancy) == 0 {
		t.Fatalf("band occupancy missing: %#v", proj.MultitrackRelation)
	}
	if len(proj.MultitrackRelation.ComparedTracks) != 2 {
		t.Fatalf("compared tracks = %#v", proj.MultitrackRelation.ComparedTracks)
	}
	if proj.TrustQuality.Coverage["track_count"] != 2 {
		t.Fatalf("coverage = %#v", proj.TrustQuality.Coverage)
	}
	data, _ := json.Marshal(proj.LLMContext)
	text := string(data)
	for _, forbidden := range []string{`"time_segments":`, `"spectrogram_tile_rows":`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("multitrack llm context leaked raw field %s in %s", forbidden, text)
		}
	}
}

func TestABResultLayerUsesCompactRenderProbeResult(t *testing.T) {
	input := testInput(map[string]any{"goal_text": "确认后复查这次音量修改"})
	current := mapValue(input.MixPackage["current_metrics"])
	current["ab_result"] = map[string]any{
		"schema_version":         "mom_ab_result.v1",
		"status":                 "ready",
		"tap_point":              "track_post_fader",
		"render_mode":            "offline_probe",
		"before_render_revision": "render_before",
		"after_render_revision":  "render_after",
		"before_evidence_ref":    "dad.l2_render_probe:render_before",
		"after_evidence_ref":     "dad.l2_render_probe:render_after",
		"summary":                "同一 tap point 的 L2 Render Probe AB 对比显示已产生可测变化。",
		"summary_tags":           []any{"rms_changed"},
		"quality_gates":          map[string]any{"same_tap_point": true, "render_revision_changed": true},
		"delta":                  map[string]any{"levels": map[string]any{"rms_dbfs": map[string]any{"before": -12.0, "after": -10.5, "delta": 1.5}}},
		"raw_samples":            []any{1, 2, 3},
		"spectral_tiles":         []any{map[string]any{"bin": 1}},
		"render_file_path":       "D:/tmp/probe.wav",
		"quality_evidence":       map[string]any{"sum_abs": 10.0},
	}
	proj := Build(input)
	layer := proj.Layers.ABResultComparison
	if proj.MOMVersion != Version || layer.Status != StatusReady || layer.Source != "mixboard.ab_result" {
		t.Fatalf("ab layer = %#v version=%s", layer, proj.MOMVersion)
	}
	ab := mapValue(layer.Facts["ab_result"])
	if ab["tap_point"] != "track_post_fader" || ab["before_render_revision"] != "render_before" || ab["after_render_revision"] != "render_after" {
		t.Fatalf("compact ab facts = %#v", ab)
	}
	data, _ := json.Marshal(proj.LLMContext)
	text := string(data)
	for _, forbidden := range []string{"raw_samples", "spectral_tiles", "render_file_path", "probe.wav", "quality_evidence"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("AB LLM context leaked raw field %s in %s", forbidden, text)
		}
	}
	for _, want := range []string{"dad.l2_render_probe:render_before", "dad.l2_render_probe:render_after", "track_post_fader", "render_after"} {
		if !strings.Contains(text, want) {
			t.Fatalf("AB LLM context missing %s in %s", want, text)
		}
	}
}

func TestSingleTrackMultitrackProjectionReturnsNotApplicable(t *testing.T) {
	input := testInput(map[string]any{
		"goal_text": "观察一下当前工程整体混音和多轨关系，不要修改。",
	})
	input.ProjectPackage["tracks"] = []any{
		map[string]any{
			"track_id":        "track_1",
			"name":            "Lead Vocal",
			"track_name":      "Lead Vocal",
			"user_label":      "Lead Vocal",
			"role_guess":      "vocal",
			"active_state":    "active",
			"level_db":        -18.0,
			"rms_dbfs":        -18.0,
			"peak_dbfs":       -3.0,
			"headroom_db":     3.0,
			"band_energy":     map[string]any{"status": "ready", "bands": map[string]any{"presence": map[string]any{"unit_energy": 0.3, "energy_db": -10.458}}},
			"stereo_relation": map[string]any{"status": "ready", "balance_db": 0.1, "correlation_estimate": 0.84, "correlation_state": "stable"},
		},
	}
	proj := Build(input)

	if proj.Intent != IntentProjectMultitrackObservation {
		t.Fatalf("intent = %q", proj.Intent)
	}
	if proj.MultitrackRelation.Status != "not_applicable_single_track" {
		t.Fatalf("multitrack relation = %#v", proj.MultitrackRelation)
	}
	if proj.TrustQuality.OverallStatus != StatusPartial {
		t.Fatalf("overall status = %q", proj.TrustQuality.OverallStatus)
	}
	if len(proj.ProjectMixProfile.ComparedTracks) != 1 {
		t.Fatalf("project mix profile = %#v", proj.ProjectMixProfile)
	}
	data, _ := json.Marshal(proj.TrustQuality.QualityGates)
	if !strings.Contains(string(data), "not_applicable_single_track") {
		t.Fatalf("quality gates = %#v", proj.TrustQuality.QualityGates)
	}
}

func TestMultitrackProjectionDoesNotFakeReadyOnMissingOrSuspectData(t *testing.T) {
	input := testInput(map[string]any{
		"goal_text": "比较一下整体混音的多轨关系，不要修改。",
	})
	tracks := rowsFromAny(input.ProjectPackage["tracks"])
	tracks[1]["band_energy"] = map[string]any{"status": "missing", "reason": "band_energy_summary_not_available"}
	tracks[1]["stereo_relation"] = map[string]any{"status": "stale", "reason": "stereo_relation_summary_stale"}
	input.ProjectPackage["tracks"] = []any{tracks[0], tracks[1]}
	proj := Build(input)

	if proj.MultitrackRelation.Status != StatusSuspect {
		t.Fatalf("multitrack relation = %#v", proj.MultitrackRelation)
	}
	if proj.TrustQuality.OverallStatus != StatusPartial && proj.TrustQuality.OverallStatus != StatusSuspect {
		t.Fatalf("overall status = %q", proj.TrustQuality.OverallStatus)
	}
	if len(proj.MultitrackRelation.MissingTracks) == 0 {
		t.Fatalf("missing tracks = %#v", proj.MultitrackRelation)
	}
	data, _ := json.Marshal(proj.TrustQuality.QualityGates)
	text := string(data)
	for _, want := range []string{"multitrack_relation:suspect", "missing_tracks:1"} {
		if !strings.Contains(text, want) {
			t.Fatalf("quality gates missing %s in %s", want, text)
		}
	}
}

func TestApproximateLoudnessIsMarkedAndNotTreatedAsFormalLUFS(t *testing.T) {
	input := testInput(map[string]any{})
	current := mapValue(input.MixPackage["current_metrics"])
	current["loudness"] = map[string]any{
		"status":          "ready",
		"algorithm":       "approximate_rms_lufs_v1",
		"approximate":     true,
		"integrated_lufs": -15.2,
		"rms_dbfs":        -15.2,
		"evidence_ref":    "dad.l3.loudness:obs_1",
	}
	input.MixPackage["current_metrics"] = current

	proj := Build(input)
	if proj.TrustQuality.SchemaVersion != "mom_trust_quality.v1" {
		t.Fatalf("trust schema = %q", proj.TrustQuality.SchemaVersion)
	}
	if len(proj.TrustQuality.ApproximateFields) == 0 {
		t.Fatalf("approximate fields missing: %#v", proj.TrustQuality)
	}
	loudness := mapValue(proj.Layers.TimeDynamicsStructure.Facts["loudness_summary"])
	if loudness["integrated_lufs"] != nil {
		t.Fatalf("approximate LUFS must not be exposed as formal integrated_lufs: %#v", loudness)
	}
	if loudness["approximate_lufs"] == nil || loudness["approximate"] != true {
		t.Fatalf("approximate loudness not marked: %#v", loudness)
	}
	data, _ := json.Marshal(proj.LLMContext)
	if strings.Contains(string(data), "integrated_lufs") {
		t.Fatalf("LLM context treated approximate loudness as formal LUFS: %s", string(data))
	}
	if !strings.Contains(string(data), "approximate_fields") {
		t.Fatalf("LLM context missing approximate field summary: %s", string(data))
	}
}

func TestActionPreflightBlocksSuspectEvidence(t *testing.T) {
	input := testInput(map[string]any{"goal_text": "please lower the low end a little"})
	current := mapValue(input.MixPackage["current_metrics"])
	current["band_energy"] = map[string]any{"status": "suspect", "reason": "coverage_too_low"}
	input.MixPackage["current_metrics"] = current

	proj := Build(input)
	if proj.Intent != IntentActionPreflightObservation {
		t.Fatalf("intent = %q", proj.Intent)
	}
	if proj.TrustQuality.CanSupportActionPreflight {
		t.Fatalf("suspect evidence should not support action preflight: %#v", proj.TrustQuality)
	}
	if !containsString(proj.TrustQuality.SuspectFields, "timbre_frequency") {
		t.Fatalf("suspect fields = %#v", proj.TrustQuality.SuspectFields)
	}
	if !containsString(proj.TrustQuality.BlockedReasons, "action_preflight_requires_ready_band_stereo_evidence") {
		t.Fatalf("blocked reasons = %#v", proj.TrustQuality.BlockedReasons)
	}
}

func TestProjectMultitrackConsumesDADV12ProjectProjection(t *testing.T) {
	input := testInput(map[string]any{"mom_intent": IntentProjectMultitrackObservation})
	input.ProjectPackage["project_band_occupancy"] = map[string]any{
		"status":      "ready",
		"track_count": 2,
		"bands": map[string]any{
			"bass": map[string]any{
				"status":              "ready",
				"average_unit_energy": 0.41,
				"dominant_tracks": []any{
					map[string]any{"track_id": "track_2", "name": "Bass", "unit_energy": 0.7, "energy_db": -3.1},
				},
			},
		},
	}
	input.ProjectPackage["project_stereo_spread"] = map[string]any{
		"status":                     "ready",
		"track_count":                2,
		"tracks_with_stereo_summary": 2,
		"widest_balance_tracks": []any{
			map[string]any{"track_id": "track_1", "name": "Lead Vocal", "balance_db": 0.1, "correlation_estimate": 0.9},
			map[string]any{"track_id": "track_2", "name": "Bass", "balance_db": -0.2, "correlation_estimate": 0.92},
		},
	}
	input.ProjectPackage["level_distribution"] = map[string]any{
		"status":  "ready",
		"highest": map[string]any{"track_id": "track_2", "name": "Bass", "metric": "rms_dbfs", "value": -16.0},
		"lowest":  map[string]any{"track_id": "track_1", "name": "Lead Vocal", "metric": "rms_dbfs", "value": -18.0},
	}
	input.ProjectPackage["conflict_candidates"] = map[string]any{
		"status": "ready",
		"candidates": []any{
			map[string]any{"type": "low_end_overlap", "status": "candidate", "band": "bass"},
		},
	}

	proj := Build(input)
	if proj.MultitrackRelation.Status != StatusReady {
		t.Fatalf("relation = %#v", proj.MultitrackRelation)
	}
	if len(proj.MultitrackRelation.BandOccupancy) != 1 || proj.MultitrackRelation.BandOccupancy[0]["band"] != "bass" {
		t.Fatalf("band occupancy did not use project projection: %#v", proj.MultitrackRelation.BandOccupancy)
	}
	if !containsString(proj.MultitrackRelation.Limitations, "project_relation_from_dad_v1_2_projection") {
		t.Fatalf("limitations = %#v", proj.MultitrackRelation.Limitations)
	}
	if !containsString(proj.MultitrackRelation.EvidenceRefs, "project_package.project_band_occupancy") {
		t.Fatalf("evidence refs = %#v", proj.MultitrackRelation.EvidenceRefs)
	}
}

func TestContextProjectionDoesNotExposeRawRevisionStrings(t *testing.T) {
	proj := Build(testInput(map[string]any{}))
	ctx := ContextProjection(proj)
	data, _ := json.Marshal(ctx)
	text := string(data)
	for _, forbidden := range []string{`"source_revision":`, `"clip_revision":`, `"render_revision":`, `rev_1`, `cliprev_1`, `render_1`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("context projection leaked revision detail %s in %s", forbidden, text)
		}
	}
	for _, want := range []string{"source_revision_status", "clip_revision_status", "render_revision_status", "mom_trust_quality.v1"} {
		if !strings.Contains(text, want) {
			t.Fatalf("context projection missing %s in %s", want, text)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func anySlice(value any) []any {
	switch v := value.(type) {
	case []any:
		return v
	case []map[string]any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, item)
		}
		return out
	default:
		return nil
	}
}

func testInput(args map[string]any) Input {
	return Input{
		ObservationID: "obs_1",
		MixSessionID:  "mix_1",
		Args:          args,
		TargetRef:     map[string]any{"kind": "track", "id": "track_1", "label": "Track 1"},
		ListenScope:   map[string]any{"time": map[string]any{"mode": "full_song"}, "source": map[string]any{"mode": "selected_track"}},
		TimeRuler:     map[string]any{"duration_seconds": 12.0},
		ProjectPackage: map[string]any{
			"status":           "ready",
			"duration_seconds": 12.0,
			"relationship_inputs": map[string]any{
				"status": "ready", "track_count": 2, "tracks_with_acoustic": 2,
			},
			"tracks": []any{
				map[string]any{
					"track_id":     "track_1",
					"name":         "Lead Vocal",
					"track_name":   "Lead Vocal",
					"user_label":   "Lead Vocal",
					"role_guess":   "vocal",
					"active_state": "active",
					"selected":     true,
					"focused":      true,
					"level_db":     -18.0,
					"rms_dbfs":     -18.0,
					"peak_dbfs":    -3.0,
					"headroom_db":  3.0,
					"pan":          0.0,
					"band_energy": map[string]any{"status": "ready", "bands": map[string]any{
						"presence": map[string]any{"energy_db": -10.0, "unit_energy": 0.316},
						"mid":      map[string]any{"energy_db": -12.0, "unit_energy": 0.251},
					}},
					"stereo_relation": map[string]any{"status": "ready", "balance_db": 0.2, "correlation_estimate": 0.82, "correlation_state": "stable", "phase_negative_ratio": 0.02},
				},
				map[string]any{
					"track_id":     "track_2",
					"name":         "Bass",
					"track_name":   "Bass",
					"user_label":   "Bass",
					"role_guess":   "bass",
					"active_state": "active",
					"selected":     false,
					"focused":      false,
					"level_db":     -16.0,
					"rms_dbfs":     -16.0,
					"peak_dbfs":    -2.0,
					"headroom_db":  2.0,
					"pan":          -0.1,
					"band_energy": map[string]any{"status": "ready", "bands": map[string]any{
						"bass": map[string]any{"energy_db": -8.0, "unit_energy": 0.398},
						"sub":  map[string]any{"energy_db": -10.0, "unit_energy": 0.316},
					}},
					"stereo_relation": map[string]any{"status": "ready", "balance_db": -0.1, "correlation_estimate": 0.9, "correlation_state": "stable", "phase_negative_ratio": 0.01},
				},
			},
		},
		MixPackage: map[string]any{
			"current_metrics": map[string]any{
				"waveform": map[string]any{"status": "ready", "rms_dbfs": -18.0, "peak_dbfs": -3.0, "headroom_db": 3.0, "crest_db": 15.0},
				"band_energy": map[string]any{"status": "ready", "source": "spectral_tile_derived", "bands": map[string]any{
					"bass": map[string]any{"energy_db": -12.0, "unit_energy": 0.25},
				}},
				"stereo_relation": map[string]any{"status": "ready", "source": "spectral_tile_derived", "balance_db": 0.2, "correlation_estimate": 0.82, "correlation_state": "stable"},
			},
			"realtime_metrics": map[string]any{
				"band_energy":     map[string]any{"status": "ready", "source": "live_level_meter_spectrum", "tap_point": "unknown_live_meter", "bands": map[string]any{"bass": map[string]any{"energy_db": -3.0}}},
				"stereo_relation": map[string]any{"status": "ready", "source": "live_level_meter_stereo", "tap_point": "unknown_live_meter", "balance_db": -0.3, "correlation_estimate": 0.68, "correlation_state": "stable"},
			},
		},
		DeepPackage: map[string]any{
			"feature_snapshot": map[string]any{
				"spectrogram_tiles": map[string]any{"status": "ready", "tile_count_seen": 8, "tile_count_expected": 8, "coverage_ratio": 1.0},
			},
		},
		AcousticPackageStatus: map[string]any{
			"schema_version":  "acoustic_package_status.v0",
			"status":          "ready",
			"source_revision": "rev_1",
			"clip_revision":   "cliprev_1",
			"render_revision": "render_1",
			"source_identity": map[string]any{"project_id": "current", "session_id": "mix_1", "track_id": "track_1", "clip_id": "clip_1", "source_revision": "rev_1", "clip_revision": "cliprev_1", "render_revision": "render_1", "duration_seconds": 12.0},
		},
	}
}
