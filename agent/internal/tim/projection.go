package tim

import (
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	StatusReady         = "ready"
	StatusPartial       = "partial"
	StatusMissing       = "missing"
	StatusSuspect       = "suspect"
	StatusNotApplicable = "not_applicable"

	SeverityError   = "error"
	SeverityWarning = "warning"
	SeverityNotice  = "notice"

	maxIssues     = 80
	maxTrackFacts = 128
)

func Build(input Input) Projection {
	tracks := rowsFromAny(input.ProjectPackage["tracks"])
	tracks = reconcileAuthoritativeSourceState(tracks, input.ProjectPackage, input.AuthoritativeState)
	declaredTrackCount := intFromAny(input.ProjectPackage["track_count"])
	if declaredTrackCount < len(tracks) {
		declaredTrackCount = len(tracks)
	}
	summary := TechnicalSummary{
		TrackCount:         declaredTrackCount,
		FormatFamilyCounts: map[string]int{},
		SampleRateCounts:   map[string]int{},
		BitDepthCounts:     map[string]int{},
		ChannelCountCounts: map[string]int{},
	}
	issues := []Issue{}
	trackFacts := []TrackFact{}
	for _, track := range tracks {
		fact, factIssues := buildTrackFact(track)
		applyTrackFactToSummary(&summary, fact)
		issues = append(issues, factIssues...)
		if len(trackFacts) < maxTrackFacts {
			trackFacts = append(trackFacts, fact)
		}
	}
	if declaredTrackCount == 0 {
		issues = append(issues, Issue{
			Code:         "no_project_tracks",
			Severity:     SeverityError,
			Detail:       "Project track summary is empty, so TIM cannot verify imported audio integrity.",
			EvidenceRefs: []string{"mix.read:project.tracks.summary"},
		})
	}
	issuesCapped := false
	if len(issues) > maxIssues {
		issues = issues[:maxIssues]
		issuesCapped = true
	}
	coverage := buildCoverage(summary)
	risk := buildRiskSummary(issues)
	limitations := buildLimitations(summary, coverage, len(tracks), issuesCapped, len(trackFacts) < len(tracks))
	for _, limitation := range authoritativeDADLimitations(summary, input.AuthoritativeState) {
		limitations = appendUniqueString(limitations, limitation)
	}
	status := projectionStatus(summary, coverage, risk)
	proj := Projection{
		SchemaVersion:    SchemaVersion,
		TIMVersion:       Version,
		ObservationID:    strings.TrimSpace(input.ObservationID),
		MixSessionID:     strings.TrimSpace(input.MixSessionID),
		Status:           status,
		TechnicalSummary: summary,
		Coverage:         coverage,
		RiskSummary:      risk,
		Issues:           issues,
		TrackFacts:       trackFacts,
		EvidenceRefs:     evidenceRefs("mix.read:project.tracks.summary", "mix.read:project.acoustic.tracks", "mix.read:project.limitations", "dad:track_waveform_envelopes"),
		Limitations:      limitations,
		GeneratedAt:      strings.TrimSpace(input.CreatedAt),
	}
	proj.LLMContext = BuildLLMContext(proj)
	return proj
}

// reconcileAuthoritativeSourceState keeps compact project packets honest. A
// missing source_path in a compact row is not evidence of missing media; only
// a matching authoritative binding/topology with explicit missing/invalid
// source evidence may upgrade that row to missing. A stale authoritative
// summary is ignored rather than borrowed across project revisions.
func reconcileAuthoritativeSourceState(tracks []map[string]any, project, authoritative map[string]any) []map[string]any {
	if len(tracks) == 0 || len(authoritative) == 0 || !authoritativeMatchesProject(project, authoritative) {
		return tracks
	}
	authRows := rowsFromAny(authoritative["tracks"])
	byKey := map[string]map[string]any{}
	byTrack := map[string][]map[string]any{}
	for _, row := range authRows {
		key := sourceStateRowKey(row)
		if key != "" {
			byKey[key] = row
		}
		if trackID := firstNonEmptyText(row, "track_id", "id"); trackID != "" {
			byTrack[trackID] = append(byTrack[trackID], row)
		}
	}
	out := make([]map[string]any, 0, len(tracks))
	for _, track := range tracks {
		copyTrack := cloneAnyMap(track)
		key := sourceStateRowKey(track)
		auth := byKey[key]
		if len(auth) == 0 && sourceStateClipID(track) == "" {
			trackID := firstNonEmptyText(track, "track_id", "id")
			if rows := byTrack[trackID]; len(rows) == 1 {
				auth = rows[0]
			}
		}
		if len(auth) > 0 {
			primary := mapValue(copyTrack["primary_clip"])
			if len(primary) == 0 {
				primary = map[string]any{}
				copyTrack["primary_clip"] = primary
			}
			// Only explicit authoritative source states are propagated. DAD
			// readiness alone is intentionally insufficient to invent identity.
			if status := firstNonEmptyText(auth, "source_status", "source_state", "source_availability"); status != "" {
				primary["source_status"] = status
			} else if valid, ok := boolValue(auth["playback_source_valid"]); ok {
				primary["playback_source_valid"] = valid
			}
			for _, keyName := range []string{"source_path", "current_source_path", "file_path"} {
				if value := cleanText(auth[keyName]); value != "" {
					primary["current_source_path"] = value
					break
				}
			}
			// Persisted L1 waveform rows are authoritative acoustic evidence for
			// the matching project binding. Keep the readiness fact internal to
			// TIM; the model receives only the compact projection derived below.
			if authAcoustic := mapValue(auth["acoustic"]); len(authAcoustic) > 0 {
				acoustic := mapValue(copyTrack["acoustic"])
				if len(acoustic) == 0 {
					acoustic = map[string]any{}
					copyTrack["acoustic"] = acoustic
				}
				for _, keyName := range []string{"status", "rms_dbfs", "peak_dbfs", "headroom_db", "crest_db", "sample_rate", "sample_rate_hz", "channel_count", "channels", "bit_depth", "bits_per_sample", "duration_seconds", "length_seconds"} {
					if value, ok := authAcoustic[keyName]; ok && value != nil {
						acoustic[keyName] = value
					}
				}
			}
		}
		out = append(out, copyTrack)
	}
	return out
}

func authoritativeMatchesProject(project, authoritative map[string]any) bool {
	for _, key := range []string{"project_uuid", "project_epoch", "project_revision", "project_state_hash"} {
		want := cleanText(project[key])
		got := cleanText(authoritative[key])
		if want != "" && got != "" && want != got {
			return false
		}
		if want != "" && got == "" {
			return false
		}
		if got != "" && want == "" {
			return false
		}
	}
	for _, key := range []string{"track_count", "clip_count"} {
		want := cleanText(project[key])
		got := cleanText(authoritative[key])
		if want != "" && got != "" && want != got {
			return false
		}
	}
	return true
}

func sourceStateRowKey(row map[string]any) string {
	if len(row) == 0 {
		return ""
	}
	trackID := firstNonEmptyText(row, "track_id", "id")
	clipID := firstNonEmptyText(row, "clip_id")
	if primary := mapValue(row["primary_clip"]); len(primary) > 0 {
		clipID = firstNonEmpty(clipID, firstNonEmptyText(primary, "clip_id", "id"))
	}
	if trackID == "" && clipID == "" {
		return ""
	}
	return trackID + "::" + clipID
}

func sourceStateClipID(row map[string]any) string {
	clipID := firstNonEmptyText(row, "clip_id")
	if primary := mapValue(row["primary_clip"]); len(primary) > 0 {
		clipID = firstNonEmpty(clipID, firstNonEmptyText(primary, "clip_id", "id"))
	}
	return clipID
}

func authoritativeDADLimitations(summary TechnicalSummary, authoritative map[string]any) []string {
	if len(authoritative) == 0 {
		return nil
	}
	status := strings.ToLower(cleanText(authoritative["dad_fact_status"]))
	ready := intFromAny(authoritative["dad_fact_ready_count"])
	total := intFromAny(authoritative["dad_fact_total_count"])
	if status == "" && ready == 0 && total == 0 {
		return nil
	}
	limits := []string{}
	if total > 0 && total != summary.TrackCount {
		limits = append(limits, "authoritative_dad_topology_count_mismatch")
	}
	if status == "ready" && total > 0 && ready != total {
		limits = append(limits, "authoritative_dad_ready_count_inconsistent")
	}
	if status == "failed" || status == "missing" || status == "partial" {
		limits = append(limits, "authoritative_dad_status_not_complete")
	}
	return limits
}

func cloneAnyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cleanText(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func firstNonEmptyText(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := cleanText(row[key]); value != "" {
			return value
		}
	}
	return ""
}

func boolValue(value any) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "1", "yes":
			return true, true
		case "false", "0", "no":
			return false, true
		}
	}
	return false, false
}

func buildTrackFact(track map[string]any) (TrackFact, []Issue) {
	primary := mapValue(track["primary_clip"])
	acoustic := mapValue(track["acoustic"])
	trackID := firstNonEmpty(text(track["track_id"]), text(track["id"]))
	trackName := firstNonEmpty(text(track["track_name"]), text(track["name"]), trackID)
	clipID := firstNonEmpty(fieldString(primary, "clip_id", "id", "item_id"), fieldString(acoustic, "primary_clip_id", "clip_id"))
	clipName := firstNonEmpty(fieldString(primary, "clip_name", "name"), fieldString(acoustic, "primary_clip_name", "clip_name", "name"), clipID)
	sourcePath := firstNonEmpty(
		fieldString(primary, "current_source_path", "source_path", "file_path"),
		fieldString(acoustic, "source_path", "current_source_path", "file_path"),
		fieldString(track, "source_path", "current_source_path", "file_path"),
	)
	clipCount := intFromAny(track["clip_count"])
	if clipCount == 0 && len(primary) > 0 {
		clipCount = 1
	}
	length := firstPositiveNumber(primary, "length_seconds", "duration_seconds", "duration")
	if length <= 0 {
		length = firstPositiveNumber(acoustic, "total_duration", "duration_seconds", "length_seconds")
	}
	playbackStatus := "unknown"
	if valid, ok := firstBool(primary, "playback_source_valid", "source_valid"); ok {
		if valid {
			playbackStatus = "valid"
		} else {
			playbackStatus = "invalid"
		}
	}
	ext := sourceExtension(firstNonEmpty(sourcePath, clipName))
	formatFamily := formatFamilyFromExt(ext)
	sampleRate := firstPositiveNumber(primary, "sample_rate_hz", "sample_rate", "source_sample_rate", "source_sample_rate_hz")
	if sampleRate <= 0 {
		sampleRate = firstPositiveNumber(acoustic, "sample_rate_hz", "sample_rate", "source_sample_rate", "source_sample_rate_hz")
	}
	if sampleRate <= 0 {
		sampleRate = firstPositiveNumber(track, "sample_rate_hz", "sample_rate", "source_sample_rate", "source_sample_rate_hz")
	}
	bitDepth := firstPositiveInt(primary, "bit_depth", "bits_per_sample", "source_bit_depth")
	if bitDepth == 0 {
		bitDepth = firstPositiveInt(acoustic, "bit_depth", "bits_per_sample", "source_bit_depth")
	}
	if bitDepth == 0 {
		bitDepth = firstPositiveInt(track, "bit_depth", "bits_per_sample", "source_bit_depth")
	}
	channelCount := firstPositiveInt(primary, "channel_count", "channels", "num_channels", "source_channel_count")
	if channelCount == 0 {
		channelCount = firstPositiveInt(acoustic, "channel_count", "channels", "num_channels", "source_channel_count")
	}
	if channelCount == 0 {
		channelCount = firstPositiveInt(track, "channel_count", "channels", "num_channels", "source_channel_count")
	}
	acousticStatus := firstNonEmpty(fieldString(acoustic, "status"), StatusMissing)
	peakDBFS, hasPeak := firstNumber(acoustic, "peak_dbfs")
	rmsDBFS, hasRMS := firstNumber(acoustic, "rms_dbfs")
	headroomDB, hasHeadroom := firstNumber(acoustic, "headroom_db")
	fact := TrackFact{
		TrackID:              trackID,
		TrackName:            trackName,
		ClipID:               clipID,
		ClipName:             clipName,
		ClipCount:            clipCount,
		LengthSeconds:        round3(length),
		SourcePresent:        strings.TrimSpace(sourcePath) != "",
		SourceStatus:         sourceFactStatus(sourcePath, primary, acoustic, track),
		PlaybackSourceStatus: playbackStatus,
		SourceExtension:      ext,
		FormatFamily:         formatFamily,
		SampleRateHz:         sampleRate,
		BitDepth:             bitDepth,
		ChannelCount:         channelCount,
		AcousticStatus:       acousticStatus,
	}
	fact.SourcePresent = fact.SourceStatus == "present"
	if hasRMS {
		value := round3(rmsDBFS)
		fact.RMSDBFS = &value
	}
	if hasPeak {
		value := round3(peakDBFS)
		fact.PeakDBFS = &value
	}
	if hasHeadroom {
		value := round3(headroomDB)
		fact.HeadroomDB = &value
	}
	issues := issuesForTrack(fact, hasPeak, peakDBFS, hasRMS, rmsDBFS, hasHeadroom, headroomDB)
	for _, issue := range issues {
		fact.RiskCodes = appendUniqueString(fact.RiskCodes, issue.Code)
	}
	return fact, issues
}

func issuesForTrack(fact TrackFact, hasPeak bool, peakDBFS float64, hasRMS bool, rmsDBFS float64, hasHeadroom bool, headroomDB float64) []Issue {
	issues := []Issue{}
	add := func(code, severity, detail string) {
		issues = append(issues, Issue{
			Code:         code,
			Severity:     severity,
			TrackID:      fact.TrackID,
			TrackName:    fact.TrackName,
			ClipID:       fact.ClipID,
			ClipName:     fact.ClipName,
			Detail:       detail,
			EvidenceRefs: []string{"mix.read:project.tracks.summary", "mix.read:project.acoustic.tracks"},
		})
	}
	if fact.ClipCount == 0 {
		add("empty_track", SeverityWarning, "Track has no clips in the project summary.")
		return issues
	}
	if fact.SourceStatus == "missing" {
		add("source_path_missing", SeverityError, "Primary clip has no readable source path in the project summary.")
	}
	if fact.PlaybackSourceStatus == "invalid" {
		add("playback_source_invalid", SeverityError, "Primary clip source is marked invalid for playback.")
	}
	if fact.LengthSeconds > 0 && fact.LengthSeconds < 0.05 {
		add("abnormally_short_clip", SeverityWarning, "Primary clip is shorter than 50 ms and may be an import or trim artifact.")
	}
	if fact.FormatFamily == "compressed" {
		add("compressed_source_format", SeverityNotice, "Primary source is a compressed audio format; bit depth is not treated as an applicable source fact.")
	}
	if fact.AcousticStatus != StatusReady {
		add("acoustic_package_missing_or_partial", SeverityWarning, "DAD waveform/acoustic package is not ready for this track.")
	}
	if hasPeak && peakDBFS >= -0.1 {
		add("possible_clipping_or_no_headroom", SeverityWarning, "Peak is at or above -0.1 dBFS in the acoustic summary.")
	}
	if hasHeadroom && headroomDB < 0.1 {
		add("possible_clipping_or_no_headroom", SeverityWarning, "Headroom is below 0.1 dB in the acoustic summary.")
	}
	if (hasPeak && peakDBFS <= -90) || (hasRMS && rmsDBFS <= -90) {
		add("possible_silence", SeverityWarning, "Acoustic level is near digital silence.")
	}
	return issues
}

func applyTrackFactToSummary(summary *TechnicalSummary, fact TrackFact) {
	if summary == nil {
		return
	}
	summary.ClipCount += fact.ClipCount
	if fact.ClipCount == 0 {
		summary.EmptyTrackCount++
		return
	}
	if fact.SourcePresent {
		summary.SourcePresentCount++
	} else if fact.SourceStatus == "missing" && fact.ClipCount > 0 {
		summary.SourceMissingCount++
	} else if fact.ClipCount > 0 {
		summary.SourceUnknownCount++
	}
	switch fact.PlaybackSourceStatus {
	case "valid":
		summary.PlaybackValidCount++
	case "invalid":
		summary.PlaybackInvalidCount++
	default:
		if fact.ClipCount > 0 {
			summary.PlaybackUnknownCount++
		}
	}
	if fact.FormatFamily == "" {
		fact.FormatFamily = "unknown"
	}
	summary.FormatFamilyCounts[fact.FormatFamily]++
	switch fact.FormatFamily {
	case "pcm_like_or_lossless":
		summary.PCMSourceCount++
	case "compressed":
		summary.CompressedSourceCount++
	default:
		summary.UnknownFormatCount++
	}
	if fact.SampleRateHz > 0 {
		summary.SampleRateCounts[fmt.Sprintf("%g_hz", fact.SampleRateHz)]++
	}
	if fact.BitDepth > 0 {
		summary.BitDepthCounts[fmt.Sprintf("%d_bit", fact.BitDepth)]++
	}
	if fact.ChannelCount > 0 {
		summary.ChannelCountCounts[fmt.Sprintf("%d_ch", fact.ChannelCount)]++
	}
	if fact.AcousticStatus == StatusReady {
		summary.AcousticReadyTrackCount++
	} else {
		summary.AcousticMissingCount++
	}
}

func buildCoverage(summary TechnicalSummary) TechnicalCoverage {
	sourceTotal := summary.TrackCount - summary.EmptyTrackCount
	if sourceTotal < 0 {
		sourceTotal = 0
	}
	sampleKnown := sumCounts(summary.SampleRateCounts)
	bitKnown := minInt(sumCounts(summary.BitDepthCounts), summary.PCMSourceCount)
	channelKnown := sumCounts(summary.ChannelCountCounts)
	return TechnicalCoverage{
		SourcePath:       coverageItemWithUnknown(summary.SourcePresentCount, sourceTotal, summary.SourceMissingCount, summary.SourceUnknownCount, 0, 0),
		PlaybackValidity: coverageItem(summary.PlaybackValidCount, sourceTotal, summary.PlaybackUnknownCount, summary.PlaybackInvalidCount, 0),
		FormatFamily:     coverageItem(summary.PCMSourceCount+summary.CompressedSourceCount, sourceTotal, summary.UnknownFormatCount, 0, 0),
		SampleRate:       coverageItem(sampleKnown, sourceTotal, sourceTotal-sampleKnown, 0, 0),
		BitDepth:         coverageItem(bitKnown, summary.PCMSourceCount, summary.PCMSourceCount-bitKnown, 0, summary.CompressedSourceCount+summary.UnknownFormatCount),
		ChannelCount:     coverageItem(channelKnown, sourceTotal, sourceTotal-channelKnown, 0, 0),
		AcousticPackage:  coverageItem(summary.AcousticReadyTrackCount, sourceTotal, summary.AcousticMissingCount, 0, 0),
	}
}

func coverageItem(known, total, missing, invalid, notApplicable int) CoverageItem {
	if missing < 0 {
		missing = 0
	}
	status := StatusReady
	switch {
	case total <= 0 && notApplicable > 0:
		status = StatusNotApplicable
	case total <= 0:
		status = StatusMissing
	case invalid > 0:
		status = StatusSuspect
	case known == 0:
		status = StatusMissing
	case known < total || missing > 0:
		status = StatusPartial
	}
	return CoverageItem{
		Status:        status,
		KnownCount:    known,
		TotalCount:    total,
		MissingCount:  missing,
		InvalidCount:  invalid,
		NotApplicable: notApplicable,
	}
}

func coverageItemWithUnknown(known, total, missing, unknown, invalid, notApplicable int) CoverageItem {
	if missing < 0 {
		missing = 0
	}
	if unknown < 0 {
		unknown = 0
	}
	status := StatusReady
	switch {
	case total <= 0 && notApplicable > 0:
		status = StatusNotApplicable
	case total <= 0:
		status = StatusMissing
	case invalid > 0:
		status = StatusSuspect
	case known == 0 && unknown == 0:
		status = StatusMissing
	case known < total || missing > 0 || unknown > 0:
		status = StatusPartial
	}
	return CoverageItem{Status: status, KnownCount: known, TotalCount: total, MissingCount: missing, UnknownCount: unknown, InvalidCount: invalid, NotApplicable: notApplicable}
}

func sourceFactStatus(sourcePath string, primary, acoustic, track map[string]any) string {
	if strings.TrimSpace(sourcePath) != "" {
		return "present"
	}
	for _, row := range []map[string]any{primary, acoustic, track} {
		for _, key := range []string{"source_status", "source_state", "source_availability"} {
			status := strings.ToLower(strings.TrimSpace(text(row[key])))
			switch status {
			case "missing", "absent", "unavailable", "invalid":
				return "missing"
			case "present", "available", "valid":
				return "present"
			}
		}
		if valid, ok := firstBool(row, "playback_source_valid", "source_valid"); ok && !valid {
			return "missing"
		}
	}
	return "unknown"
}

func buildRiskSummary(issues []Issue) RiskSummary {
	bySeverity := map[string]int{}
	codeCounts := map[string]int{}
	for _, issue := range issues {
		bySeverity[issue.Severity]++
		codeCounts[issue.Code]++
	}
	overall := "none"
	if bySeverity[SeverityError] > 0 {
		overall = "high"
	} else if bySeverity[SeverityWarning] > 0 {
		overall = "medium"
	} else if bySeverity[SeverityNotice] > 0 {
		overall = "low"
	}
	type codeCount struct {
		code  string
		count int
	}
	codes := make([]codeCount, 0, len(codeCounts))
	for code, count := range codeCounts {
		codes = append(codes, codeCount{code: code, count: count})
	}
	sort.Slice(codes, func(i, j int) bool {
		if codes[i].count == codes[j].count {
			return codes[i].code < codes[j].code
		}
		return codes[i].count > codes[j].count
	})
	primary := []string{}
	for _, item := range codes {
		primary = append(primary, item.code)
		if len(primary) >= 8 {
			break
		}
	}
	return RiskSummary{
		OverallRisk:  overall,
		IssueCount:   len(issues),
		BySeverity:   bySeverity,
		PrimaryCodes: primary,
	}
}

func projectionStatus(summary TechnicalSummary, coverage TechnicalCoverage, risk RiskSummary) string {
	if summary.TrackCount == 0 {
		return StatusMissing
	}
	if risk.BySeverity[SeverityError] > 0 || coverage.PlaybackValidity.Status == StatusSuspect {
		return StatusSuspect
	}
	for _, item := range []CoverageItem{coverage.SourcePath, coverage.FormatFamily, coverage.SampleRate, coverage.BitDepth, coverage.ChannelCount, coverage.AcousticPackage} {
		if item.Status == StatusMissing || item.Status == StatusPartial {
			return StatusPartial
		}
	}
	return StatusReady
}

func buildLimitations(summary TechnicalSummary, coverage TechnicalCoverage, observedTrackCount int, issuesCapped, tracksCapped bool) []string {
	limits := []string{}
	if summary.TrackCount > 0 && observedTrackCount == 0 {
		limits = append(limits, "project_track_summaries_unavailable")
	}
	if coverage.SampleRate.Status == StatusMissing || coverage.SampleRate.Status == StatusPartial {
		limits = append(limits, "source_sample_rate_not_fully_exposed_by_project_state")
	}
	if coverage.BitDepth.Status == StatusMissing || coverage.BitDepth.Status == StatusPartial {
		limits = append(limits, "pcm_bit_depth_not_fully_exposed_by_project_state")
	}
	if coverage.ChannelCount.Status == StatusMissing || coverage.ChannelCount.Status == StatusPartial {
		limits = append(limits, "source_channel_count_not_fully_exposed_by_project_state")
	}
	if coverage.AcousticPackage.Status == StatusMissing || coverage.AcousticPackage.Status == StatusPartial {
		limits = append(limits, "dad_acoustic_package_not_ready_for_all_tracks")
	}
	if coverage.SourcePath.UnknownCount > 0 {
		limits = append(limits, "source_path_not_exposed_by_compact_project_state")
	}
	if summary.CompressedSourceCount > 0 {
		limits = append(limits, "compressed_audio_formats_do_not_provide_pcm_bit_depth")
	}
	if issuesCapped {
		limits = append(limits, "tim_issue_list_capped")
	}
	if tracksCapped {
		limits = append(limits, "tim_track_fact_list_capped")
	}
	return evidenceRefs(limits...)
}

func BuildLLMContext(proj Projection) LLMContext {
	facts := []map[string]any{
		{
			"layer":              "technical_summary",
			"status":             proj.Status,
			"track_count":        proj.TechnicalSummary.TrackCount,
			"clip_count":         proj.TechnicalSummary.ClipCount,
			"source_missing":     proj.TechnicalSummary.SourceMissingCount,
			"source_unknown":     proj.TechnicalSummary.SourceUnknownCount,
			"playback_invalid":   proj.TechnicalSummary.PlaybackInvalidCount,
			"compressed_sources": proj.TechnicalSummary.CompressedSourceCount,
			"format_families":    proj.TechnicalSummary.FormatFamilyCounts,
		},
		{
			"layer":         "technical_coverage",
			"status":        proj.Status,
			"sample_rate":   proj.Coverage.SampleRate,
			"bit_depth":     proj.Coverage.BitDepth,
			"channel_count": proj.Coverage.ChannelCount,
			"acoustic":      proj.Coverage.AcousticPackage,
		},
		{
			"layer":         "technical_risk",
			"status":        proj.Status,
			"overall_risk":  proj.RiskSummary.OverallRisk,
			"issue_count":   proj.RiskSummary.IssueCount,
			"primary_codes": proj.RiskSummary.PrimaryCodes,
		},
	}
	if len(proj.Issues) > 0 {
		facts = append(facts, map[string]any{
			"layer":  "technical_issue_excerpt",
			"status": proj.Status,
			"issues": compactIssues(proj.Issues, 8),
		})
	}
	return LLMContext{
		SummaryMD:              summaryMD(proj),
		CompactFacts:           facts,
		DoNotIncludeRawPackage: true,
		EvidenceRefs:           proj.EvidenceRefs,
		QualitySummary: map[string]any{
			"schema_version": proj.SchemaVersion,
			"status":         proj.Status,
			"overall_risk":   proj.RiskSummary.OverallRisk,
			"issue_count":    proj.RiskSummary.IssueCount,
			"coverage":       proj.Coverage,
		},
		LimitationNotes:   proj.Limitations,
		SuggestedNextStep: suggestedNextStep(proj),
	}
}

func summaryMD(proj Projection) string {
	return fmt.Sprintf(
		"TIM %s status=%s risk=%s tracks=%d clips=%d issues=%d. Source/sample/channel facts are evidence-only; unknown fields must be reported as limitations, not inferred.",
		proj.TIMVersion,
		proj.Status,
		proj.RiskSummary.OverallRisk,
		proj.TechnicalSummary.TrackCount,
		proj.TechnicalSummary.ClipCount,
		proj.RiskSummary.IssueCount,
	)
}

func suggestedNextStep(proj Projection) string {
	if proj.RiskSummary.BySeverity[SeverityError] > 0 {
		return "Resolve missing or invalid imported source clips before using the project for mix actions."
	}
	if proj.TechnicalSummary.SourceUnknownCount > 0 {
		return "Confirm source identity through the authoritative project binding before treating compact metadata gaps as missing media."
	}
	if proj.Status == StatusPartial {
		return "Report missing technical facts explicitly, then proceed with conservative organization or listening tasks."
	}
	return "TIM evidence is sufficient for the next project organization or listening step."
}

func ContextProjection(proj Projection) map[string]any {
	return map[string]any{
		"schema_version":     proj.SchemaVersion,
		"tim_version":        proj.TIMVersion,
		"observation_id":     proj.ObservationID,
		"mix_session_id":     proj.MixSessionID,
		"status":             proj.Status,
		"technical_summary":  proj.TechnicalSummary,
		"coverage":           proj.Coverage,
		"risk_summary":       proj.RiskSummary,
		"issue_excerpt":      compactIssues(proj.Issues, 12),
		"track_fact_excerpt": compactTrackFacts(proj.TrackFacts, 24),
		"limitations":        proj.Limitations,
		"llm_context":        proj.LLMContext,
	}
}

func compactIssues(issues []Issue, maxItems int) []map[string]any {
	out := []map[string]any{}
	for _, issue := range issues {
		out = append(out, map[string]any{
			"code":       issue.Code,
			"severity":   issue.Severity,
			"track_id":   issue.TrackID,
			"track_name": issue.TrackName,
			"clip_id":    issue.ClipID,
			"clip_name":  issue.ClipName,
			"detail":     issue.Detail,
		})
		if len(out) >= maxItems {
			break
		}
	}
	return out
}

func compactTrackFacts(facts []TrackFact, maxItems int) []map[string]any {
	out := []map[string]any{}
	for _, fact := range facts {
		row := map[string]any{
			"track_id":               fact.TrackID,
			"track_name":             fact.TrackName,
			"clip_id":                fact.ClipID,
			"clip_count":             fact.ClipCount,
			"source_present":         fact.SourcePresent,
			"source_status":          fact.SourceStatus,
			"playback_source_status": fact.PlaybackSourceStatus,
			"format_family":          fact.FormatFamily,
			"source_extension":       fact.SourceExtension,
			"acoustic_status":        fact.AcousticStatus,
			"risk_codes":             fact.RiskCodes,
		}
		if fact.LengthSeconds > 0 {
			row["length_seconds"] = fact.LengthSeconds
		}
		if fact.SampleRateHz > 0 {
			row["sample_rate_hz"] = fact.SampleRateHz
		}
		if fact.BitDepth > 0 {
			row["bit_depth"] = fact.BitDepth
		}
		if fact.ChannelCount > 0 {
			row["channel_count"] = fact.ChannelCount
		}
		out = append(out, row)
		if len(out) >= maxItems {
			break
		}
	}
	return out
}

func sourceExtension(pathOrName string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(strings.TrimSpace(pathOrName))), ".")
	if ext == "wave" {
		return "wav"
	}
	return ext
}

func formatFamilyFromExt(ext string) string {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case "wav", "wave", "bwf", "aif", "aiff", "flac", "caf":
		return "pcm_like_or_lossless"
	case "mp3", "aac", "m4a", "ogg", "opus", "wma":
		return "compressed"
	case "":
		return "unknown"
	default:
		return "unknown"
	}
}

func rowsFromAny(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return typed
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if row := mapValue(item); len(row) > 0 {
				out = append(out, row)
			}
		}
		return out
	default:
		return nil
	}
}

func mapValue(value any) map[string]any {
	if row, ok := value.(map[string]any); ok {
		return row
	}
	return nil
}

func fieldString(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := text(row[key]); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func text(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return ""
	}
}

func firstPositiveNumber(row map[string]any, keys ...string) float64 {
	for _, key := range keys {
		if value, ok := numberFromAny(row[key]); ok && value > 0 {
			return value
		}
	}
	return 0
}

func firstNumber(row map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if value, ok := numberFromAny(row[key]); ok {
			return value, true
		}
	}
	return 0, false
}

func numberFromAny(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		if !math.IsNaN(typed) && !math.IsInf(typed, 0) {
			return typed, true
		}
	case float32:
		value := float64(typed)
		if !math.IsNaN(value) && !math.IsInf(value, 0) {
			return value, true
		}
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case json.Number:
		if value, err := strconv.ParseFloat(typed.String(), 64); err == nil {
			return value, true
		}
	case string:
		if value, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
			return value, true
		}
	}
	return 0, false
}

func firstPositiveInt(row map[string]any, keys ...string) int {
	for _, key := range keys {
		if value := intFromAny(row[key]); value > 0 {
			return value
		}
	}
	return 0
}

func intFromAny(value any) int {
	if number, ok := numberFromAny(value); ok {
		return int(math.Round(number))
	}
	return 0
}

func firstBool(row map[string]any, keys ...string) (bool, bool) {
	for _, key := range keys {
		switch typed := row[key].(type) {
		case bool:
			return typed, true
		case string:
			switch strings.ToLower(strings.TrimSpace(typed)) {
			case "true", "yes", "1", "valid":
				return true, true
			case "false", "no", "0", "invalid":
				return false, true
			}
		}
	}
	return false, false
}

func sumCounts(counts map[string]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func round3(value float64) float64 {
	if value == 0 {
		return 0
	}
	return math.Round(value*1000) / 1000
}

func appendUniqueString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func evidenceRefs(values ...string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
