package rlm

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"vit-daw-agent/internal/levelsafety"
)

func TestBuildReferenceLevelModelUsesActiveRMSAsStrictMetric(t *testing.T) {
	proj := Build(Input{
		GeneratedAt: "2026-07-10T00:00:00Z",
		ProjectState: map[string]any{
			"tracks": []map[string]any{
				testTrack("track_a", "clip_a", 0),
				testTrack("track_b", "clip_b", 0),
				testTrack("track_c", "clip_c", 0),
			},
		},
		AudioAnalysisStatus: map[string]any{
			"analysis_job": map[string]any{
				"track_waveform_envelopes": []map[string]any{
					{"track_id": "track_a", "active_rms_dbfs": -24.0, "rms_dbfs": -35.0},
					{"track_id": "track_b", "active_rms_dbfs": -20.0, "rms_dbfs": -30.0},
					{"track_id": "track_c", "active_rms_dbfs": -20.0, "rms_dbfs": -30.0},
				},
			},
		},
	})

	if proj.Status != StatusReady || proj.Mode != ModeStrict || proj.SelectedMetric != "active_rms_dbfs" {
		t.Fatalf("projection status/mode/metric = %s/%s/%s", proj.Status, proj.Mode, proj.SelectedMetric)
	}
	if proj.ReferenceLevel == nil || *proj.ReferenceLevel != -20 {
		t.Fatalf("reference = %+v", proj.ReferenceLevel)
	}
	if len(proj.Calibration) != 1 || proj.Calibration[0].ClipID != "clip_a" {
		t.Fatalf("calibration rows = %+v", proj.Calibration)
	}
	if proj.Calibration[0].TargetClipGainDB != 4 {
		t.Fatalf("target gain = %+v", proj.Calibration[0])
	}
	if !proj.Actionable {
		t.Fatalf("projection should be actionable: %+v", proj)
	}
}

func TestMergeSourceRowsKeepsTrackAggregateWhenDADUsesAnotherClip(t *testing.T) {
	merged := mergeSourceRows(
		sourceRows([]map[string]any{{
			"track_id": "track_a",
			"clips":    []map[string]any{{"clip_id": "clip_primary", "clip_gain_db": 2.0, "duration_seconds": 10.0}, {"clip_id": "clip_other", "clip_gain_db": -3.0, "duration_seconds": 4.0}},
		}}, "project.state:tracks"),
		sourceRows([]map[string]any{{
			"track_id":                  "track_a",
			"primary_clip":              map[string]any{"clip_id": "clip_primary"},
			"rms_dbfs":                  -20.0,
			"effective_static_rms_dbfs": -18.0,
		}}, "mix.observe:project_package.tracks"),
		sourceRows([]map[string]any{{
			"track_id":  "track_a",
			"clip_id":   "clip_other",
			"rms_dbfs":  -40.0,
			"peak_dbfs": -20.0,
		}}, "project.audio_analysis_status:track_waveform_envelopes"),
	)
	if len(merged) != 1 {
		t.Fatalf("merged rows = %+v", merged)
	}
	row := referenceRowFromMap(merged[0].Data, merged[0].EvidenceRefs)
	if row.PrimaryClipID != "clip_primary" {
		t.Fatalf("primary clip identity = %q, want clip_primary; data=%+v", row.PrimaryClipID, merged[0].Data)
	}
	if row.RMSDBFS == nil || math.Abs(*row.RMSDBFS-(-20.0)) > 0.001 {
		t.Fatalf("different DAD clip overwrote track aggregate: row=%+v data=%+v", row, merged[0].Data)
	}
}

func TestBuildReferenceLevelModelUsesDADAggregateWhenRepresentativeClipDiffers(t *testing.T) {
	tracks := []map[string]any{}
	waveforms := []map[string]any{}
	for i, level := range []float64{-30, -20, -20} {
		trackID := fmt.Sprintf("track_%d", i)
		tracks = append(tracks, map[string]any{
			"track_id": trackID,
			"clips": []map[string]any{
				{"clip_id": "dad_clip_" + trackID, "clip_gain_db": 0.0, "duration_seconds": 2.0, "type": "audio"},
				{"clip_id": "primary_clip_" + trackID, "clip_gain_db": 0.0, "duration_seconds": 12.0, "type": "audio"},
			},
		})
		waveforms = append(waveforms, map[string]any{
			"track_id":  trackID,
			"clip_id":   "dad_clip_" + trackID,
			"rms_dbfs":  level,
			"peak_dbfs": -20.0,
		})
	}
	proj := Build(Input{
		ProjectState:        map[string]any{"tracks": tracks},
		AudioAnalysisStatus: map[string]any{"analysis_job": map[string]any{"track_waveform_envelopes": waveforms}},
	})
	if proj.Status != StatusReady || proj.Summary.CandidateCount != 3 {
		t.Fatalf("DAD aggregate was lost on representative clip mismatch: status=%s summary=%+v rows=%+v", proj.Status, proj.Summary, proj.Rows)
	}
	if len(proj.Calibration) != 1 || proj.Calibration[0].ClipID != "primary_clip_track_0" || proj.Calibration[0].TargetClipGainDB != 10 {
		t.Fatalf("calibration should target writable primary clip: %+v", proj.Calibration)
	}
}

func TestBuildReferenceLevelModelUsesFullRMSAsCoarseWhenActiveMissing(t *testing.T) {
	proj := Build(Input{
		GeneratedAt: "2026-07-10T00:00:00Z",
		ProjectState: map[string]any{
			"tracks": []map[string]any{
				testTrack("track_a", "clip_a", 0),
				testTrack("track_b", "clip_b", 0),
				testTrack("track_c", "clip_c", 0),
			},
		},
		AudioAnalysisStatus: map[string]any{
			"track_waveform_envelopes": []map[string]any{
				{"track_id": "track_a", "rms_dbfs": -24.5, "peak_dbfs": -10.0},
				{"track_id": "track_b", "rms_dbfs": -20.0, "peak_dbfs": -8.0},
				{"track_id": "track_c", "rms_dbfs": -20.5, "peak_dbfs": -8.5},
			},
		},
	})

	if proj.Status != StatusReady || proj.Mode != ModeCoarse || proj.SelectedMetric != "rms_dbfs" {
		t.Fatalf("projection status/mode/metric = %s/%s/%s", proj.Status, proj.Mode, proj.SelectedMetric)
	}
	if len(proj.Calibration) != 2 || proj.Calibration[0].TargetClipGainDB != 4 || proj.Calibration[1].TargetClipGainDB != -0.5 {
		t.Fatalf("coarse calibration rows = %+v", proj.Calibration)
	}
	if len(proj.Limitations) == 0 || proj.Limitations[0] != "rlm_strict_active_or_lufs_missing_coarse_reference" {
		t.Fatalf("limitations = %+v", proj.Limitations)
	}
}

func TestBuildReferenceLevelModelBlocksPartialCoverage(t *testing.T) {
	proj := Build(Input{
		GeneratedAt: "2026-07-10T00:00:00Z",
		ProjectState: map[string]any{
			"tracks": []map[string]any{
				testTrack("track_a", "clip_a", 0),
				testTrack("track_b", "clip_b", 0),
				testTrack("track_c", "clip_c", 0),
			},
		},
		AudioAnalysisStatus: map[string]any{
			"track_waveform_envelopes": []map[string]any{
				{"track_id": "track_a", "rms_dbfs": -24.0},
				{"track_id": "track_b", "rms_dbfs": -20.0},
			},
		},
	})

	if proj.Status != StatusPartial || proj.Actionable {
		t.Fatalf("partial projection should not be actionable: status=%s actionable=%v rows=%+v", proj.Status, proj.Actionable, proj.Calibration)
	}
	if len(proj.Calibration) != 0 {
		t.Fatalf("partial coverage should not create calibration rows: %+v", proj.Calibration)
	}
	if proj.Summary.MissingMetricCount != 1 {
		t.Fatalf("missing count = %+v", proj.Summary)
	}
}

func TestBuildReferenceLevelModelExcludesExplicitSilentWaveformFromCoverage(t *testing.T) {
	proj := Build(Input{
		GeneratedAt: "2026-07-10T00:00:00Z",
		ProjectState: map[string]any{
			"tracks": []map[string]any{
				testTrack("track_a", "clip_a", 0),
				testTrack("track_b", "clip_b", 0),
				testTrack("track_silent", "clip_silent", 0),
			},
		},
		AudioAnalysisStatus: map[string]any{
			"analysis_job": map[string]any{
				"track_waveform_envelopes": []map[string]any{
					{"track_id": "track_a", "rms_dbfs": -24.0, "peak_dbfs": -10.0},
					{"track_id": "track_b", "rms_dbfs": -20.0, "peak_dbfs": -8.0},
					{"track_id": "track_silent", "clip_id": "clip_silent", "rms": 0.0, "peak_abs": 0.0, "status": "ready"},
				},
			},
		},
	})

	if proj.Status != StatusReady || proj.Summary.EligibleTrackCount != 2 || proj.Summary.CandidateCount != 2 {
		t.Fatalf("silent source should not block strict coverage: status=%s summary=%+v limitations=%+v", proj.Status, proj.Summary, proj.Limitations)
	}
	foundSilent := false
	for _, row := range proj.Rows {
		if row.TrackID == "track_silent" {
			foundSilent = row.SilentSource && !row.Eligible && row.EligibilityReason == "silent_source_no_level_evidence"
		}
	}
	if !foundSilent {
		t.Fatalf("silent row was not marked/excluded: %+v", proj.Rows)
	}
}

func TestBuildReferenceLevelModelUsesLinearDADRowsAndExcludesOneSilentSource(t *testing.T) {
	tracks := make([]map[string]any, 0, 61)
	waveforms := make([]map[string]any, 0, 61)
	for i := 0; i < 60; i++ {
		trackID := fmt.Sprintf("track_%02d", i)
		clipID := fmt.Sprintf("clip_%02d", i)
		tracks = append(tracks, testTrack(trackID, clipID, 0))
		levelDB := -21.0
		if i%2 == 0 {
			levelDB = -20.0
		}
		waveforms = append(waveforms, map[string]any{
			"track_id": trackID,
			"clip_id":  clipID,
			"rms":      ampFromDB(levelDB),
			"peak_abs": ampFromDB(-8.0),
			"status":   "ready",
		})
	}
	tracks = append(tracks, testTrack("track_silent", "clip_silent", 0))
	waveforms = append(waveforms, map[string]any{
		"track_id": "track_silent",
		"clip_id":  "clip_silent",
		"rms":      0.0,
		"peak_abs": 0.0,
		"status":   "ready",
	})

	proj := Build(Input{
		GeneratedAt: "2026-07-10T00:00:00Z",
		ProjectState: map[string]any{
			"tracks": tracks,
		},
		AudioAnalysisStatus: map[string]any{
			"analysis_job": map[string]any{
				"track_waveform_envelopes": waveforms,
			},
		},
	})

	if proj.Status != StatusReady || proj.SelectedMetric != "rms_dbfs" || proj.Summary.CandidateCount != 60 || proj.Summary.EligibleTrackCount != 60 {
		t.Fatalf("realistic DAD rows should produce full non-silent coverage: status=%s metric=%s summary=%+v limitations=%+v", proj.Status, proj.SelectedMetric, proj.Summary, proj.Limitations)
	}
	if len(proj.Calibration) != 60 {
		t.Fatalf("calibration rows = %d, want 60", len(proj.Calibration))
	}
}

func TestBuildReferenceLevelModelCreatesManyRowsForBatchUse(t *testing.T) {
	tracks := make([]map[string]any, 0, 64)
	waveforms := make([]map[string]any, 0, 64)
	for i := 0; i < 64; i++ {
		trackID := fmt.Sprintf("track_%02d", i)
		clipID := fmt.Sprintf("clip_%02d", i)
		tracks = append(tracks, testTrack(trackID, clipID, 0))
		level := -20.0
		if i%2 == 0 {
			level = -21.0
		}
		waveforms = append(waveforms, map[string]any{"track_id": trackID, "active_rms_dbfs": level})
	}
	proj := Build(Input{
		GeneratedAt:         "2026-07-10T00:00:00Z",
		ProjectState:        map[string]any{"tracks": tracks},
		AudioAnalysisStatus: map[string]any{"track_waveform_envelopes": waveforms},
	})

	if proj.Status != StatusReady || proj.Summary.CandidateCount != 64 {
		t.Fatalf("projection summary = %+v status=%s", proj.Summary, proj.Status)
	}
	if len(proj.Calibration) != 64 {
		t.Fatalf("strict reference should keep all actionable rows, got %d: %+v", len(proj.Calibration), proj.Calibration)
	}
}

func TestBuildReferenceLevelModelClampsCalibrationToSourcePeakSafety(t *testing.T) {
	proj := Build(Input{
		GeneratedAt: "2026-08-02T00:00:00Z",
		ProjectState: map[string]any{"tracks": []map[string]any{
			testTrack("1062", "1066", 0),
			testTrack("ref_a", "ref_clip_a", 0),
			testTrack("ref_b", "ref_clip_b", 0),
		}},
		AudioAnalysisStatus: map[string]any{"track_waveform_envelopes": []map[string]any{
			{"track_id": "1062", "clip_id": "1066", "rms_dbfs": -72.905, "peak_dbfs": -23.718},
			{"track_id": "ref_a", "clip_id": "ref_clip_a", "rms_dbfs": -48.905, "peak_dbfs": -10.0},
			{"track_id": "ref_b", "clip_id": "ref_clip_b", "rms_dbfs": -48.905, "peak_dbfs": -10.0},
		}},
	})

	if len(proj.Calibration) != 1 {
		t.Fatalf("calibration = %+v", proj.Calibration)
	}
	row := proj.Calibration[0]
	if math.Abs(row.TargetClipGainDB-22.718) > 0.001 || math.Abs(row.AppliedDeltaDB-22.718) > 0.001 {
		t.Fatalf("calibration row = %+v", row)
	}
	if !row.PeakSafetyClipped || row.ProjectedPeakDBFS == nil || math.Abs(*row.ProjectedPeakDBFS-(-1.0)) > 0.001 || row.PeakSafetyAchieved == nil || !*row.PeakSafetyAchieved {
		t.Fatalf("peak safety = %+v", row)
	}
}

func TestBuildReferenceLevelModelPreservesExistingFaderWhenClampingClipGain(t *testing.T) {
	quiet := testTrack("1062", "1066", 0)
	quiet["volume_db"] = 1.5
	proj := Build(Input{
		GeneratedAt: "2026-08-03T00:00:00Z",
		ProjectState: map[string]any{"tracks": []map[string]any{
			quiet,
			testTrack("ref_a", "ref_clip_a", 0),
			testTrack("ref_b", "ref_clip_b", 0),
		}},
		AudioAnalysisStatus: map[string]any{"track_waveform_envelopes": []map[string]any{
			{"track_id": "1062", "clip_id": "1066", "rms_dbfs": -72.905, "peak_dbfs": -23.718},
			{"track_id": "ref_a", "clip_id": "ref_clip_a", "rms_dbfs": -48.905, "peak_dbfs": -10.0},
			{"track_id": "ref_b", "clip_id": "ref_clip_b", "rms_dbfs": -48.905, "peak_dbfs": -10.0},
		}},
	})

	if len(proj.Calibration) != 1 {
		t.Fatalf("calibration = %+v", proj.Calibration)
	}
	row := proj.Calibration[0]
	if math.Abs(row.TargetClipGainDB-21.218) > 0.001 || row.TrackFaderDB == nil || math.Abs(*row.TrackFaderDB-1.5) > 0.001 {
		t.Fatalf("calibration row = %+v", row)
	}
	if row.ProjectedPeakDBFS == nil || math.Abs(*row.ProjectedPeakDBFS-levelsafety.StaticPeakCeilingDBFS) > 0.001 {
		t.Fatalf("projected peak = %+v", row.ProjectedPeakDBFS)
	}
}

func testTrack(trackID string, clipID string, gainDB float64) map[string]any {
	return map[string]any{
		"track_id":       trackID,
		"track_name":     trackID,
		"is_audio_track": true,
		"track_type":     "audio",
		"volume_db":      0.0,
		"clips": []map[string]any{{
			"clip_id":          clipID,
			"clip_name":        clipID + ".wav",
			"type":             "audio",
			"clip_gain_db":     gainDB,
			"duration_seconds": 12.0,
		}},
	}
}

func ampFromDB(db float64) float64 {
	return math.Pow(10, db/20)
}

// TestRLMThreeSourceMergeRevisionMismatchIsVisible — E8 红测（现状锁定，非修复）。
//
// 规格权威：docs/MATERIALIZATION_V1_DESIGN.md §5.4（G2-C 素材，RECON §2/§5.3
// 指认 E8）：mergeSourceRows 按 track 键做字段级覆盖合并，无 revision 比较——
// "mix.observe 后工程再变更"时序下，r1 旧观察值可静默覆盖 r2 新工程态。
//
// 红测纪律（AGENTS §10）：本测试锁定**现状行为**并显式标注"已知隐患，修复卡
// （rlm 三源 revision 比较，OQ-4）合入后升级断言：异 revision 源触发 stale/
// mismatch 注记"。它的存在使隐患可见、修复可验，不是豁免。
func TestRLMThreeSourceMergeRevisionMismatchIsVisible(t *testing.T) {
	proj := Build(Input{
		GeneratedAt: "2026-09-28T00:00:00Z",
		// 源 1（工程态，r2 最新）：T3 推子在观察之后被改到 -6。
		ProjectState: map[string]any{
			"project_revision": "r2",
			"tracks": []map[string]any{{
				"track_id": "T3", "track_name": "lead", "is_audio_track": true,
				"track_type": "audio", "volume_db": -6.0,
				"clips": []map[string]any{{"clip_id": "C3", "clip_name": "lead.wav", "type": "audio", "clip_gain_db": 0.0, "duration_seconds": 12.0}},
			}},
		},
		// 源 2（mix.observe 产物，r1 旧观察）：推子还是改前 -3.0，rms=-20.5。
		MixObservation: map[string]any{"observation": map[string]any{"project_package": map[string]any{"tracks": []map[string]any{
			{"track_id": "T3", "track_name": "lead", "volume_db": -3.0, "rms_dbfs": -20.5},
		}}}},
		// 源 3（DAD 烘焙，r1 旧分析）：rms=-18.0。
		AudioAnalysisStatus: map[string]any{"track_waveform_envelopes": []map[string]any{
			{"track_id": "T3", "clip_id": "C3", "rms_dbfs": -18.0, "peak_dbfs": -3.0},
		}},
	})

	// 现状锁定①：Build 成功——revision 不一致不阻断、不报错。
	if proj.SchemaVersion == "" {
		t.Fatalf("Build 未产出投影")
	}
	var row *ReferenceLevelRow
	for i := range proj.Rows {
		if proj.Rows[i].TrackID == "T3" {
			row = &proj.Rows[i]
			break
		}
	}
	if row == nil {
		t.Fatalf("T3 行缺失：rows=%+v", proj.Rows)
	}
	// 现状锁定②（E8 本体）：r1 旧观察的推子值（-3.0）静默覆盖 r2 新工程态
	// （-6.0）——后源字段级覆盖、无 revision 门。修复卡合入后此断言升级为：
	// 异 revision 源不得覆盖新工程态（触发 stale/mismatch 注记）。
	if row.TrackFaderDB == nil || math.Abs(*row.TrackFaderDB-(-3.0)) > 0.001 {
		t.Fatalf("E8 现状锁定失败：TrackFaderDB=%+v（期望后源旧观察值 -3.0 覆盖工程态 -6.0）", row.TrackFaderDB)
	}
	// 现状锁定③：声学值取最后源（DAD r1 烘焙）。
	if row.RMSDBFS == nil || math.Abs(*row.RMSDBFS-(-18.0)) > 0.001 {
		t.Fatalf("RMSDBFS=%+v（期望最后源 -18.0）", row.RMSDBFS)
	}
	// 现状锁定④：隐患在产物里不可见——无任何 revision/mismatch/stale 注记
	//（行上三源 EvidenceRefs 并存是唯一痕迹）。修复卡合入后升级为：产物带
	// revision_mismatch 可见性。
	for _, limitation := range proj.Limitations {
		if strings.Contains(strings.ToLower(limitation), "revision") || strings.Contains(strings.ToLower(limitation), "mismatch") || strings.Contains(strings.ToLower(limitation), "stale") {
			t.Fatalf("现状不应有 revision/mismatch/stale 注记（修复卡后升级断言）：%q", limitation)
		}
	}
	if len(row.EvidenceRefs) < 3 {
		t.Fatalf("三源合并痕迹（EvidenceRefs）不足：%v", row.EvidenceRefs)
	}
}
