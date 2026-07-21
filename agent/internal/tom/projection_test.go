package tom

import (
	"fmt"
	"strings"
	"testing"
)

func TestBuildFromImportRowsPrioritizesNamingAndAbbreviations(t *testing.T) {
	proj := BuildFromImportRows(ImportInput{
		Summary: map[string]any{"tracks_created": 6, "edit_length_seconds": 228.57},
		TIMProjection: map[string]any{
			"status": "ready",
			"technical_summary": map[string]any{
				"track_count":                6,
				"clip_count":                 6,
				"source_present_count":       6,
				"acoustic_ready_track_count": 6,
			},
			"risk_summary": map[string]any{"overall_risk": "none", "issue_count": 0},
		},
		Rows: []map[string]any{
			stemRow("track_lv", "Lead Vox", "clip_lv", "Lead Vox.wav", 228.57, 2),
			stemRow("track_bgv_1", "BGV 01", "clip_bgv_1", "BGV 01.wav", 228.57, 2),
			stemRow("track_str_1", "STR High", "clip_str_1", "STR High.wav", 228.57, 2),
			stemRow("track_kick", "Kick In", "clip_kick", "Kick In.wav", 228.57, 1),
			stemRow("track_bass", "Sub Bass", "clip_bass", "Sub Bass.wav", 228.57, 2),
			stemRow("track_fx", "FX Riser", "clip_fx", "FX Riser.wav", 7.5, 2),
		},
		DADWaveformRows: []map[string]any{
			{"status": "ready", "track_id": "track_lv", "clip_id": "clip_lv", "rms_dbfs": -18.0, "peak_dbfs": -5.0},
		},
	})

	if proj.SchemaVersion != SchemaVersion || proj.TOMVersion != Version {
		t.Fatalf("version mismatch: %#v", proj)
	}
	for _, want := range []string{"vocals", "backing_vocals", "strings", "drums", "bass", "fx"} {
		if group := findGroup(proj, want); group == nil {
			t.Fatalf("missing group %q in %#v", want, proj.GroupProposals)
		}
	}
	if got := findGroup(proj, "strings"); got == nil || got.Confidence != confidenceHigh {
		t.Fatalf("STR should be high-confidence strings, got %#v", got)
	}
	if got := findGroup(proj, "backing_vocals"); got == nil || got.Confidence != confidenceHigh {
		t.Fatalf("BGV should be high-confidence backing vocals, got %#v", got)
	}
	if proj.OrganizationSummary.NamingMatchedTrackCount != 6 {
		t.Fatalf("naming matched count = %d summary=%#v", proj.OrganizationSummary.NamingMatchedTrackCount, proj.OrganizationSummary)
	}
	if proj.OrganizationSummary.NeedsReviewTrackCount != 0 {
		t.Fatalf("strong naming should not require review: %#v", proj.OrganizationSummary)
	}
	if fact := findLLMFact(proj, "tim_projection"); fact == nil || fact["status"] != "ready" || fact["overall_risk"] != "none" {
		t.Fatalf("TOM should carry compact TIM status/risk fact, got %#v in %#v", fact, proj.LLMContext.CompactFacts)
	}
	if proj.DisclosurePlan.SelectedStage != disclosureStageNamingID {
		t.Fatalf("strong naming should stop at naming/id stage: %#v", proj.DisclosurePlan)
	}
	if proj.FullManifest.CoverageStatus != manifestCoverageComplete || proj.FullManifest.AssignmentCoverageCount != 6 {
		t.Fatalf("full manifest coverage mismatch: %#v", proj.FullManifest)
	}
}

func TestBuildFromImportRowsUsesTechnicalFallbackForWeakNames(t *testing.T) {
	proj := BuildFromImportRows(ImportInput{
		Summary: map[string]any{"tracks_created": 3, "edit_length_seconds": 120.0},
		Rows: []map[string]any{
			stemRow("audio_001", "Audio 001", "clip_001", "Audio 001.wav", 120.0, 2),
			stemRow("audio_002", "Audio 002", "clip_002", "Audio 002.wav", 120.0, 2),
			stemRow("audio_003", "Audio 003", "clip_003", "Audio 003.wav", 120.0, 2),
		},
	})

	group := findGroup(proj, "long_stereo_stems")
	if group == nil {
		t.Fatalf("weak long stereo tracks should be grouped by technical fallback: %#v", proj.GroupProposals)
	}
	if group.RoleHypothesis == "drums_or_percussion" || group.RoleHypothesis == "lead_vocal" {
		t.Fatalf("technical fallback must not fabricate roles: %#v", group)
	}
	if group.Confidence != confidenceLow {
		t.Fatalf("technical fallback should stay low-confidence, got %#v", group)
	}
	if proj.OrganizationSummary.NamingMatchedTrackCount != 0 || proj.OrganizationSummary.TechnicalFallbackTrackCount != 3 {
		t.Fatalf("summary mismatch: %#v", proj.OrganizationSummary)
	}
	if proj.Status != statusPartial {
		t.Fatalf("weak naming projection should be partial: %#v", proj)
	}
	if proj.DisclosurePlan.SelectedStage != disclosureStageTechnical {
		t.Fatalf("weak names without DAD should disclose technical clustering: %#v", proj.DisclosurePlan)
	}
	if proj.NamingSignalReport.NamingSignalTrackCount != 0 || proj.NamingSignalReport.WeakNameTrackCount != 3 {
		t.Fatalf("generic Audio 001 names should not count as naming signal: %#v", proj.NamingSignalReport)
	}
}

func TestBuildFromImportRowsUsesMonoAndShortFallbackWithoutRoleClaims(t *testing.T) {
	proj := BuildFromImportRows(ImportInput{
		Summary: map[string]any{"tracks_created": 2, "edit_length_seconds": 90.0},
		Rows: []map[string]any{
			stemRow("mic_01", "Take 01", "clip_mic", "Take 01.wav", 90.0, 1),
			stemRow("clip_02", "Audio Chop", "clip_chop", "Audio Chop.wav", 2.0, 2),
		},
	})

	if group := findGroup(proj, "mono_sources"); group == nil || group.RoleHypothesis != "mono_source_unknown" {
		t.Fatalf("mono fallback missing or too assertive: %#v", group)
	}
	if group := findGroup(proj, "short_clips"); group == nil || group.RoleHypothesis != "short_clip_unknown" {
		t.Fatalf("short fallback missing or too assertive: %#v", group)
	}
	if proj.OrganizationSummary.NeedsReviewTrackCount != 2 {
		t.Fatalf("fallback tracks should require review: %#v", proj.OrganizationSummary)
	}
}

func TestBuildFromImportRowsUsesDADAcousticReviewForWeakNames(t *testing.T) {
	proj := BuildFromImportRows(ImportInput{
		Summary: map[string]any{"tracks_created": 2, "edit_length_seconds": 120.0},
		Rows: []map[string]any{
			stemRow("audio_001", "Audio 001", "clip_001", "Audio 001.wav", 120.0, 2),
			stemRow("audio_002", "Audio 002", "clip_002", "Audio 002.wav", 120.0, 2),
		},
		DADWaveformRows: []map[string]any{
			{"status": "ready", "track_id": "audio_001", "clip_id": "clip_001", "rms_dbfs": -84.2, "peak_dbfs": -96.0},
			{"status": "ready", "track_id": "audio_002", "clip_id": "clip_002", "rms_dbfs": -14.0, "peak_dbfs": -0.05, "headroom_db": 0.05},
		},
	})

	silent := findGroup(proj, "silent_candidates")
	if silent == nil || silent.RoleHypothesis != "silence_or_empty_candidate" || !silent.NeedsConfirmation {
		t.Fatalf("near-silent weak names should become review candidates: %#v", silent)
	}
	hot := findGroup(proj, "hot_clipping_review")
	if hot == nil || hot.RoleHypothesis != "hot_or_clipping_candidate" || !hot.NeedsConfirmation {
		t.Fatalf("hot weak names should become clipping review candidates: %#v", hot)
	}
	for _, group := range []*GroupProposal{silent, hot} {
		if group.RoleHypothesis == "drums_or_percussion" || group.RoleHypothesis == "lead_vocal" {
			t.Fatalf("acoustic review must not fabricate instrument roles: %#v", group)
		}
	}
	if proj.OrganizationSummary.NeedsReviewTrackCount != 2 {
		t.Fatalf("acoustic review groups should require confirmation: %#v", proj.OrganizationSummary)
	}
	if proj.DisclosurePlan.SelectedStage != disclosureStageDADLightweight {
		t.Fatalf("weak names with DAD facts should disclose dad_lightweight: %#v", proj.DisclosurePlan)
	}
}

func TestBuildFromImportRowsKeepsNamingPriorityOverAcousticReview(t *testing.T) {
	proj := BuildFromImportRows(ImportInput{
		Summary: map[string]any{"tracks_created": 1, "edit_length_seconds": 90.0},
		Rows: []map[string]any{
			stemRow("lead_vox", "Lead Vox", "clip_vox", "Lead Vox.wav", 90.0, 2),
		},
		DADWaveformRows: []map[string]any{
			{"status": "ready", "track_id": "lead_vox", "clip_id": "clip_vox", "peak_dbfs": -0.05, "headroom_db": 0.05},
		},
	})

	if group := findGroup(proj, "vocals"); group == nil || group.TrackCount != 1 {
		t.Fatalf("strong naming should remain in the named role group: %#v", proj.GroupProposals)
	}
	if group := findGroup(proj, "hot_clipping_review"); group != nil {
		t.Fatalf("acoustic review should not override clear naming: %#v", group)
	}
}

func TestFullManifestKeepsAllAssignmentsWhenContextExcerptOmitted(t *testing.T) {
	rows := make([]map[string]any, 0, 25)
	for i := 1; i <= 25; i++ {
		rows = append(rows, stemRow(
			fmt.Sprintf("track_kick_%02d", i),
			fmt.Sprintf("Kick %02d", i),
			fmt.Sprintf("clip_kick_%02d", i),
			fmt.Sprintf("Kick %02d.wav", i),
			180,
			1,
		))
	}
	proj := BuildFromImportRows(ImportInput{
		Summary: map[string]any{"tracks_created": 25, "edit_length_seconds": 180.0},
		Rows:    rows,
	})

	group := findGroup(proj, "drums")
	if group == nil || group.TrackCount != 25 {
		t.Fatalf("expected one 25-track drum group, got %#v", proj.GroupProposals)
	}
	manifestGroup := findManifestGroup(proj.FullManifest, "drums")
	if manifestGroup == nil || len(manifestGroup.TrackIDs) != 25 {
		t.Fatalf("full manifest truncated group IDs: %#v", proj.FullManifest)
	}
	if proj.FullManifest.AssignmentCoverageCount != 25 || proj.FullManifest.CoverageStatus != manifestCoverageComplete {
		t.Fatalf("manifest coverage mismatch: %#v", proj.FullManifest)
	}
	ctx := ContextProjectionMap(proj)
	ctxGroups := rowsFromAny(ctx["group_proposals"])
	if len(ctxGroups) == 0 {
		t.Fatalf("context projection missing groups: %#v", ctx)
	}
	if omitted := intFromAny(ctxGroups[0]["assignment_omitted_count"]); omitted != 13 {
		t.Fatalf("readable context should still omit the 13-item excerpt tail, got %d in %#v", omitted, ctxGroups[0])
	}
	ctxManifest := messageMap(ctx["full_assignment_manifest"])
	ctxManifestGroups := rowsFromAny(ctxManifest["groups"])
	if len(ctxManifestGroups) == 0 || len(messageStringSlice(ctxManifestGroups[0]["track_ids"])) != 25 {
		t.Fatalf("full manifest in context must keep all track IDs: %#v", ctxManifest)
	}
	fact := findLLMFact(proj, "full_assignment_manifest")
	if fact == nil || !strings.Contains(fmt.Sprint(fact), "track_kick_25") {
		t.Fatalf("LLM context should expose full manifest IDs, got %#v", fact)
	}
}

func TestNamingSignalReportDetectsSimilarIDClusters(t *testing.T) {
	proj := BuildFromImportRows(ImportInput{
		Summary: map[string]any{"tracks_created": 4, "edit_length_seconds": 100.0},
		Rows: []map[string]any{
			stemRow("bgv_01", "BGV 01", "clip_bgv_01", "BGV 01.wav", 100.0, 2),
			stemRow("bgv_02", "BGV 02", "clip_bgv_02", "BGV 02.wav", 100.0, 2),
			stemRow("bgv_03", "BGV 03", "clip_bgv_03", "BGV 03.wav", 100.0, 2),
			stemRow("audio_004", "Audio 004", "clip_004", "Audio 004.wav", 100.0, 2),
		},
	})

	if proj.NamingSignalReport.NamingMatchedTrackCount != 3 {
		t.Fatalf("BGV tracks should be role-name matched: %#v", proj.NamingSignalReport)
	}
	if proj.NamingSignalReport.NamingSignalTrackCount != 3 || proj.NamingSignalReport.WeakNameTrackCount != 1 {
		t.Fatalf("naming signal coverage mismatch: %#v", proj.NamingSignalReport)
	}
	foundBGVCluster := false
	for _, cluster := range proj.NamingSignalReport.SimilarityClusters {
		if cluster.Key == "bgv" && cluster.Count == 3 {
			foundBGVCluster = true
		}
	}
	if !foundBGVCluster {
		t.Fatalf("expected similar BGV id/name cluster: %#v", proj.NamingSignalReport.SimilarityClusters)
	}
}

func stemRow(trackID, trackName, clipID, fileName string, duration float64, channels int) map[string]any {
	return map[string]any{
		"track_id":         trackID,
		"track_name":       trackName,
		"clip_id":          clipID,
		"clip_name":        fileName,
		"source_file_path": "E:/stems/" + fileName,
		"duration_seconds": duration,
		"sample_rate_hz":   48000,
		"bit_depth":        24,
		"pcm_format":       "int24",
		"channel_count":    channels,
	}
}

func findGroup(proj Projection, groupID string) *GroupProposal {
	for i := range proj.GroupProposals {
		if proj.GroupProposals[i].GroupID == groupID {
			return &proj.GroupProposals[i]
		}
	}
	return nil
}

func findLLMFact(proj Projection, layer string) map[string]any {
	for _, fact := range proj.LLMContext.CompactFacts {
		if fact["layer"] == layer {
			return fact
		}
	}
	return nil
}

func findManifestGroup(manifest FullAssignmentManifest, groupID string) *ManifestGroup {
	for i := range manifest.Groups {
		if manifest.Groups[i].GroupID == groupID {
			return &manifest.Groups[i]
		}
	}
	return nil
}

func messageMap(value any) map[string]any {
	if row, ok := value.(map[string]any); ok {
		return row
	}
	return nil
}

func messageStringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			out = append(out, fmt.Sprint(item))
		}
		return out
	default:
		return nil
	}
}
