package epm

import (
	"encoding/json"
	"fmt"
	"math"
	"path"
	"sort"
	"strconv"
	"strings"
)

const (
	statusReady   = "ready"
	statusPartial = "partial"
	statusLimited = "limited"
)

type clipProfile struct {
	TrackID       string
	TrackName     string
	ClipID        string
	ClipName      string
	SourcePath    string
	StartSeconds  float64
	HasStart      bool
	Duration      float64
	HasDuration   bool
	ClipCount     int
	ChannelCount  int
	FormatFamily  string
	SourceExt     string
	AcousticReady bool
	RMSDBFS       *float64
	PeakDBFS      *float64
	HeadroomDB    *float64
	TimeSegments  []timeEnergySegment
}

type timeEnergySegment struct {
	StartSeconds float64
	EndSeconds   float64
	RMS          float64
	RMSDBFS      float64
	PeakAbs      float64
	PeakDBFS     float64
	EnergyState  string
}

type sectionBoundary struct {
	TimeSeconds      float64
	Score            float64
	ActivityBefore   float64
	ActivityAfter    float64
	EnergyBefore     float64
	EnergyAfter      float64
	ActiveCountAfter int
}

type timelineBucket struct {
	StartSeconds float64
	EndSeconds   float64
	ActiveCount  int
	EnergySum    float64
	EnergyAvg    float64
}

func BuildFromImportRows(input ImportInput) Projection {
	profiles := buildClipProfiles(input)
	maxDuration := maxProfileDuration(profiles, input.Summary)
	cleanup := buildClipCleanupProjection(profiles, maxDuration)
	sectionMap := buildSectionMapProjection(profiles, input.TOMProjection)
	limitations := buildLimitations(input, cleanup)
	status := buildProjectionStatus(cleanup, sectionMap)
	proj := Projection{
		SchemaVersion: SchemaVersion,
		EPMVersion:    Version,
		ObservationID: strings.TrimSpace(input.ObservationID),
		MixSessionID:  strings.TrimSpace(input.MixSessionID),
		Status:        status,
		ClipCleanup:   cleanup,
		SectionMap:    sectionMap,
		DeepReasonPack: DeepReasonPack{
			Available:          true,
			DefaultMode:        "compact_projection",
			ExpandableLayers:   []string{"clip_timing_manifest", "dad_lightweight_edges", "tom_group_context", "section_reference_candidates"},
			RequiresUserIntent: true,
		},
		EvidenceRefs: evidenceRefs("import:track_refs", "tim:projection", "tom:projection", "dad:track_waveform_envelopes"),
		Limitations:  limitations,
		GeneratedAt:  strings.TrimSpace(input.CreatedAt),
	}
	proj.LLMContext = BuildLLMContext(proj)
	return proj
}

func buildClipProfiles(input ImportInput) []clipProfile {
	dadByTrackClip, dadBySource := indexDADRows(input.DADWaveformRows)
	out := make([]clipProfile, 0, len(input.Rows))
	for i, row := range input.Rows {
		if len(row) == 0 {
			continue
		}
		start, hasStart := firstNumber(row, "start_time_seconds", "start_time", "start_seconds", "position_seconds", "timeline_start_seconds")
		duration, hasDuration := firstNumber(row, "length_seconds", "duration_seconds", "duration", "edit_length_seconds", "clip_length_seconds")
		profile := clipProfile{
			TrackID:      firstNonEmpty(fieldString(row, "track_id", "id"), fmt.Sprintf("track_%03d", i+1)),
			TrackName:    firstNonEmpty(fieldString(row, "track_name", "name"), fieldString(row, "clip_name", "file_name")),
			ClipID:       firstNonEmpty(fieldString(row, "clip_id", "primary_clip_id", "item_id"), fmt.Sprintf("clip_%03d", i+1)),
			ClipName:     firstNonEmpty(fieldString(row, "clip_name", "file_name", "name")),
			SourcePath:   firstNonEmpty(fieldString(row, "source_file_path", "current_source_path", "source_path", "file_path", "imported_file_path", "copied_file_path", "path")),
			StartSeconds: start,
			HasStart:     hasStart,
			Duration:     duration,
			HasDuration:  hasDuration && duration > 0,
			ClipCount:    firstPositiveInt(row, "clip_count"),
			ChannelCount: firstPositiveInt(row, "channel_count", "channels", "num_channels", "source_channel_count"),
			FormatFamily: firstNonEmpty(fieldString(row, "format_family"), formatFamilyFromExt(sourceExtension(firstNonEmpty(fieldString(row, "source_file_path", "source_path", "file_path"), fieldString(row, "clip_name", "file_name"))))),
		}
		if profile.ClipCount <= 0 {
			profile.ClipCount = 1
		}
		profile.SourceExt = sourceExtension(firstNonEmpty(profile.SourcePath, profile.ClipName))
		if dad := dadForProfile(profile, dadByTrackClip, dadBySource); len(dad) > 0 {
			mergeDADFacts(&profile, dad)
		}
		out = append(out, profile)
	}
	return out
}

func buildClipCleanupProjection(profiles []clipProfile, maxDuration float64) ClipCleanupProjection {
	cleanup := ClipCleanupProjection{
		Status:                  statusLimited,
		TrackCount:              len(profiles),
		ClipCount:               totalClipCount(profiles),
		MaxDurationSeconds:      round3(maxDuration),
		StemAlignmentConfidence: "unknown",
		Recommendation:          "limited_missing_facts",
		TrimRecommendation:      "not_enough_data",
		FadeRecommendation:      "not_recommended_by_default",
		CrossfadeRecommendation: "not_recommended_by_default",
		AutoEditAllowed:         false,
		TechniqueNotes: []string{
			"Full-length stems should preserve timeline alignment unless the user explicitly asks for destructive cleanup.",
			"Fade is optional; EPM only raises candidates when there is boundary or acoustic risk evidence.",
		},
	}
	if len(profiles) == 0 {
		cleanup.TechniqueNotes = append(cleanup.TechniqueNotes, "No clip rows were available for edit preparation.")
		return cleanup
	}

	alignedStart := 0
	nearFullLength := 0
	withTiming := 0
	withClipTiming := 0
	candidates := []EditCandidate{}
	for _, profile := range profiles {
		if profile.HasStart || profile.HasDuration {
			withTiming++
			withClipTiming += maxInt(profile.ClipCount, 1)
		}
		if profile.HasStart && math.Abs(profile.StartSeconds) <= 0.02 {
			alignedStart++
		}
		if profile.HasDuration && maxDuration > 0 && profile.Duration >= maxDuration*0.95 {
			nearFullLength++
		}
		if profile.HasDuration && profile.Duration > 0 && profile.Duration < 1.0 {
			cleanup.CandidateSummary.ShortClipReviewCount++
			candidates = append(candidates, candidateFromProfile(profile, "short_clip_review", "medium", "人工复核短片段是否为误导入或转场素材", fmt.Sprintf("片段长度 %.2fs", profile.Duration), true))
		}
		if profileLooksSilent(profile) {
			cleanup.CandidateSummary.SilenceReviewCount++
			cleanup.CandidateSummary.TrimCandidateCount++
			candidates = append(candidates, candidateFromProfile(profile, "silence_review", "medium", "复核是否需要裁掉明显静音素材", "DAD 显示接近数字静音", true))
		}
		if profileLooksHot(profile) {
			cleanup.CandidateSummary.HotPeakReviewCount++
			cleanup.CandidateSummary.FadeCandidateCount++
			candidates = append(candidates, candidateFromProfile(profile, "fade_boundary_review", "low", "复核边界是否需要短 fade，避免爆点或切口", "峰值/余量接近 0 dBFS", true))
		}
	}
	cleanup.TrackWithTimingCount = withTiming
	cleanup.ClipWithTimingCount = withClipTiming
	cleanup.AlignedStartCount = alignedStart
	cleanup.NearFullLengthCount = nearFullLength
	if len(profiles) > 0 {
		cleanup.AlignmentCoverage = round3(float64(minInt(alignedStart, nearFullLength)) / float64(len(profiles)))
	}

	fullLengthRatio := 0.0
	alignedRatio := 0.0
	if len(profiles) > 0 {
		fullLengthRatio = float64(nearFullLength) / float64(len(profiles))
		alignedRatio = float64(alignedStart) / float64(len(profiles))
	}
	switch {
	case withTiming == 0:
		cleanup.Status = statusLimited
		cleanup.Recommendation = "limited_missing_facts"
		cleanup.TrimRecommendation = "not_enough_data"
	case alignedRatio >= 0.8 && fullLengthRatio >= 0.8:
		cleanup.Status = statusReady
		cleanup.FullLengthStemDetected = true
		cleanup.PreserveStemAlignment = true
		cleanup.StemAlignmentConfidence = "high"
		cleanup.Recommendation = "no_cleanup_needed"
		cleanup.TrimRecommendation = "not_recommended_for_stems"
		cleanup.FadeRecommendation = "not_recommended_by_default"
		cleanup.CrossfadeRecommendation = "not_recommended_for_single_full_length_stems"
	case alignedRatio >= 0.6 || fullLengthRatio >= 0.6:
		cleanup.Status = statusPartial
		cleanup.FullLengthStemDetected = true
		cleanup.PreserveStemAlignment = true
		cleanup.StemAlignmentConfidence = "medium"
		cleanup.Recommendation = "review_candidates"
		cleanup.TrimRecommendation = "candidate_review_required"
	default:
		cleanup.Status = statusPartial
		cleanup.StemAlignmentConfidence = "low"
		cleanup.Recommendation = "review_candidates"
		cleanup.TrimRecommendation = "candidate_review_required"
	}
	if cleanup.CandidateSummary.FadeCandidateCount > 0 {
		cleanup.FadeRecommendation = "review_boundary_risk_only"
	}
	if cleanup.CandidateSummary.TrimCandidateCount > 0 && !cleanup.FullLengthStemDetected {
		cleanup.TrimRecommendation = "candidate_review_required"
	}
	cleanup.Candidates = capCandidates(candidates, 16)
	return cleanup
}

func buildSectionMapProjection(profiles []clipProfile, tomProjection map[string]any) SectionMapProjection {
	trackCount := len(profiles)
	strategy := "single_track_proxy"
	switch {
	case trackCount >= 8:
		strategy = "multitrack_group_activity"
	case trackCount >= 2:
		strategy = "few_track_proxy_mix"
	}
	inputs := []string{"clip timing map", "DAD lightweight waveform envelopes"}
	if len(tomProjection) > 0 {
		inputs = append(inputs, "TOM group context")
	}
	duration := maxProfileDuration(profiles, nil)
	sections, evidence := buildSectionCandidatesFromProfiles(profiles, duration, strategy)
	status := "recommended"
	if len(sections) == 0 {
		status = statusLimited
	} else if evidence.TimeSegmentCount == 0 {
		status = "suggested_low_confidence"
	}
	confidence := sectionMapConfidence(evidence, len(sections))
	coverageSeconds := sectionCoverageSeconds(sections)
	coverageRatio := 0.0
	if duration > 0 {
		coverageRatio = round3(coverageSeconds / duration)
	}
	return SectionMapProjection{
		Status:            status,
		TrackCount:        trackCount,
		DurationSeconds:   round3(duration),
		ReferenceStrategy: strategy,
		Confidence:        confidence,
		CandidateCount:    len(sections),
		CoverageSeconds:   round3(coverageSeconds),
		CoverageRatio:     coverageRatio,
		EvidenceSummary:   evidence,
		RecommendedInputs: inputs,
		SupportedStrategies: []SectionStrategy{
			{ID: "single_track_proxy", Label: "Single-track proxy analysis", AppliesWhen: "single track or user-selected reference track", Status: "planned"},
			{ID: "few_track_proxy_mix", Label: "Few-track proxy mix analysis", AppliesWhen: "2-7 tracks or sparse arrangement", Status: "planned"},
			{ID: "multitrack_group_activity", Label: "Multitrack group activity map", AppliesWhen: "organized multitrack/stem project", Status: "planned"},
		},
		Sections:           sections,
		MarkerWriteSupport: "ready",
		LLMReasoningMode:   "compact_by_default_deep_on_request",
		Notes: []string{
			"A5 produces recommended section candidates only; confirmed marker writing is available through project.markers.apply_section_markers.",
			"Section labels are recommendations, not guaranteed musical truth.",
		},
	}
}

func BuildLLMContext(proj Projection) LLMContext {
	facts := []map[string]any{
		{
			"layer":                     "clip_cleanup",
			"status":                    proj.ClipCleanup.Status,
			"track_count":               proj.ClipCleanup.TrackCount,
			"clip_count":                proj.ClipCleanup.ClipCount,
			"full_length_stem_detected": proj.ClipCleanup.FullLengthStemDetected,
			"preserve_stem_alignment":   proj.ClipCleanup.PreserveStemAlignment,
			"alignment_confidence":      proj.ClipCleanup.StemAlignmentConfidence,
			"recommendation":            proj.ClipCleanup.Recommendation,
			"trim_recommendation":       proj.ClipCleanup.TrimRecommendation,
			"fade_recommendation":       proj.ClipCleanup.FadeRecommendation,
			"candidate_summary":         proj.ClipCleanup.CandidateSummary,
		},
		{
			"layer":              "section_map",
			"status":             proj.SectionMap.Status,
			"reference_strategy": proj.SectionMap.ReferenceStrategy,
			"confidence":         proj.SectionMap.Confidence,
			"candidate_count":    proj.SectionMap.CandidateCount,
			"coverage_ratio":     proj.SectionMap.CoverageRatio,
			"marker_write":       proj.SectionMap.MarkerWriteSupport,
			"reasoning_mode":     proj.SectionMap.LLMReasoningMode,
			"evidence_summary":   proj.SectionMap.EvidenceSummary,
		},
	}
	if len(proj.ClipCleanup.Candidates) > 0 {
		facts = append(facts, map[string]any{
			"layer":      "edit_candidate_excerpt",
			"candidates": compactCandidates(proj.ClipCleanup.Candidates, 8),
		})
	}
	if len(proj.SectionMap.Sections) > 0 {
		facts = append(facts, map[string]any{
			"layer":    "section_candidate_excerpt",
			"sections": compactSections(proj.SectionMap.Sections, 8),
		})
	}
	return LLMContext{
		SummaryMD:              summaryMD(proj),
		CompactFacts:           facts,
		DoNotIncludeRawPackage: true,
		EvidenceRefs:           proj.EvidenceRefs,
		LimitationNotes:        proj.Limitations,
		SuggestedNextStep:      "Ask the user before applying any clip trim, fade, crossfade, or marker write operation; section map candidates are recommendations only.",
	}
}

func ContextProjection(proj Projection) map[string]any {
	return map[string]any{
		"schema_version":   proj.SchemaVersion,
		"epm_version":      proj.EPMVersion,
		"observation_id":   proj.ObservationID,
		"mix_session_id":   proj.MixSessionID,
		"status":           proj.Status,
		"clip_cleanup":     proj.ClipCleanup,
		"section_map":      proj.SectionMap,
		"deep_reason_pack": proj.DeepReasonPack,
		"limitations":      proj.Limitations,
		"llm_context":      proj.LLMContext,
	}
}

func ContextProjectionMap(proj Projection) map[string]any {
	data, err := json.Marshal(ContextProjection(proj))
	if err != nil {
		return ContextProjection(proj)
	}
	out := map[string]any{}
	if err := json.Unmarshal(data, &out); err != nil {
		return ContextProjection(proj)
	}
	return out
}

func buildProjectionStatus(cleanup ClipCleanupProjection, section SectionMapProjection) string {
	if cleanup.Status == statusLimited {
		return statusLimited
	}
	if cleanup.Status == statusReady && section.Status != statusLimited {
		return statusReady
	}
	return statusPartial
}

func buildLimitations(input ImportInput, cleanup ClipCleanupProjection) []string {
	limitations := []string{}
	if len(input.Rows) == 0 {
		limitations = append(limitations, "epm_import_rows_missing")
	}
	if cleanup.TrackWithTimingCount < cleanup.TrackCount {
		limitations = append(limitations, "clip_timing_incomplete")
	}
	if len(input.DADWaveformRows) == 0 {
		limitations = append(limitations, "dad_lightweight_edges_unavailable")
	}
	return evidenceRefs(limitations...)
}

func buildSectionCandidatesFromProfiles(profiles []clipProfile, duration float64, strategy string) ([]SectionCandidate, SectionEvidence) {
	evidence := SectionEvidence{
		BoundarySource:          "duration_template",
		ActivityChangeThreshold: 0.35,
		EnergyChangeThreshold:   0.35,
	}
	for _, profile := range profiles {
		if profile.HasStart || profile.HasDuration {
			evidence.TimingTrackCount++
		}
		if len(profile.TimeSegments) > 0 {
			evidence.TimeSegmentTrackCount++
			evidence.TimeSegmentCount += len(profile.TimeSegments)
		}
	}
	if duration <= 0 {
		evidence.Limitations = evidenceRefs("section_duration_missing")
		return nil, evidence
	}

	if evidence.TimeSegmentCount > 0 {
		buckets := buildTimelineBuckets(profiles, duration)
		boundaries := chooseSectionBoundaries(buckets, duration)
		evidence.BoundaryCandidateCount = len(boundaries)
		if len(boundaries) > 0 {
			evidence.BoundarySource = "dad_time_segment_activity"
			sections := sectionsFromBoundaries(duration, boundaries, strategy)
			if len(sections) > 0 {
				return sections, evidence
			}
		}
		evidence.Limitations = evidenceRefs("dad_time_segments_available_but_no_clear_boundaries")
	}

	evidence.Limitations = evidenceRefs(append(evidence.Limitations, "section_map_template_fallback")...)
	return templateSectionCandidates(duration, strategy), evidence
}

func buildTimelineBuckets(profiles []clipProfile, duration float64) []timelineBucket {
	bucketSize := sectionBucketSize(duration)
	if bucketSize <= 0 {
		return nil
	}
	count := int(math.Ceil(duration / bucketSize))
	if count <= 0 {
		return nil
	}
	if count > 64 {
		count = 64
		bucketSize = duration / float64(count)
	}
	buckets := make([]timelineBucket, count)
	for i := range buckets {
		start := float64(i) * bucketSize
		end := start + bucketSize
		if i == len(buckets)-1 || end > duration {
			end = duration
		}
		buckets[i] = timelineBucket{StartSeconds: round3(start), EndSeconds: round3(end)}
	}
	for _, profile := range profiles {
		for _, segment := range profile.TimeSegments {
			if segment.EndSeconds <= segment.StartSeconds {
				continue
			}
			energy := segmentEnergy(segment)
			if energy <= 0 {
				continue
			}
			for i := range buckets {
				if segment.EndSeconds <= buckets[i].StartSeconds || segment.StartSeconds >= buckets[i].EndSeconds {
					continue
				}
				buckets[i].ActiveCount++
				buckets[i].EnergySum += energy
			}
		}
	}
	for i := range buckets {
		if buckets[i].ActiveCount > 0 {
			buckets[i].EnergyAvg = buckets[i].EnergySum / float64(buckets[i].ActiveCount)
		}
		buckets[i].EnergySum = round3(buckets[i].EnergySum)
		buckets[i].EnergyAvg = round3(buckets[i].EnergyAvg)
	}
	return buckets
}

func chooseSectionBoundaries(buckets []timelineBucket, duration float64) []sectionBoundary {
	if len(buckets) < 4 || duration <= 0 {
		return nil
	}
	candidates := []sectionBoundary{}
	for i := 1; i < len(buckets); i++ {
		t := buckets[i].StartSeconds
		if t < duration*0.08 || t > duration*0.92 {
			continue
		}
		before := averagedBucketWindow(buckets, i-2, i)
		after := averagedBucketWindow(buckets, i, i+2)
		activityDelta := normalizedDelta(before.ActiveCount, after.ActiveCount)
		energyDelta := normalizedDelta(before.EnergyAvg, after.EnergyAvg)
		score := activityDelta*0.6 + energyDelta*0.4
		if score < 0.28 {
			continue
		}
		candidates = append(candidates, sectionBoundary{
			TimeSeconds:      t,
			Score:            round3(score),
			ActivityBefore:   round3(float64(before.ActiveCount)),
			ActivityAfter:    round3(float64(after.ActiveCount)),
			EnergyBefore:     round3(before.EnergyAvg),
			EnergyAfter:      round3(after.EnergyAvg),
			ActiveCountAfter: after.ActiveCount,
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].TimeSeconds < candidates[j].TimeSeconds
		}
		return candidates[i].Score > candidates[j].Score
	})
	selected := []sectionBoundary{}
	minSpacing := math.Max(8, duration*0.08)
	for _, candidate := range candidates {
		tooClose := false
		for _, existing := range selected {
			if math.Abs(candidate.TimeSeconds-existing.TimeSeconds) < minSpacing {
				tooClose = true
				break
			}
		}
		if tooClose {
			continue
		}
		selected = append(selected, candidate)
		if len(selected) >= maxSectionBoundaryCount(duration) {
			break
		}
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].TimeSeconds < selected[j].TimeSeconds })
	return selected
}

func sectionsFromBoundaries(duration float64, boundaries []sectionBoundary, strategy string) []SectionCandidate {
	points := []float64{0}
	basisByStart := map[float64][]string{}
	scoreByStart := map[float64]float64{}
	for _, boundary := range boundaries {
		if boundary.TimeSeconds <= 0 || boundary.TimeSeconds >= duration {
			continue
		}
		t := round3(boundary.TimeSeconds)
		points = append(points, t)
		basisByStart[t] = []string{
			fmt.Sprintf("activity %.1f -> %.1f", boundary.ActivityBefore, boundary.ActivityAfter),
			fmt.Sprintf("energy %.3f -> %.3f", boundary.EnergyBefore, boundary.EnergyAfter),
		}
		scoreByStart[t] = boundary.Score
	}
	points = append(points, round3(duration))
	points = uniqueSortedFloats(points)
	if len(points) < 2 {
		return nil
	}
	sections := []SectionCandidate{}
	totalSections := len(points) - 1
	for i := 0; i < totalSections; i++ {
		start := points[i]
		end := points[i+1]
		if end <= start {
			continue
		}
		score := 0.42
		basis := []string{strategy + " reference"}
		if i > 0 {
			if items := basisByStart[start]; len(items) > 0 {
				basis = append(basis, items...)
			}
			if boundaryScore := scoreByStart[start]; boundaryScore > 0 {
				score = clampFloat(0.45+boundaryScore*0.45, 0.45, 0.78)
			}
		} else {
			score = 0.48
			basis = append(basis, "project start")
		}
		labelHint := sectionLabelHint(i, totalSections)
		sections = append(sections, SectionCandidate{
			SectionID:       fmt.Sprintf("section_%02d", i+1),
			Label:           sectionDisplayLabel(labelHint, i),
			LabelHint:       labelHint,
			StartSeconds:    round3(start),
			EndSeconds:      round3(end),
			DurationSeconds: round3(end - start),
			Confidence:      sectionConfidenceLabel(score),
			ConfidenceScore: round3(score),
			Basis:           evidenceRefs(basis...),
			RequiresConfirm: true,
		})
	}
	return sections
}

func templateSectionCandidates(duration float64, strategy string) []SectionCandidate {
	if duration <= 0 {
		return nil
	}
	if duration <= 16 {
		return []SectionCandidate{{
			SectionID:       "section_01",
			Label:           "完整段落",
			LabelHint:       "full",
			StartSeconds:    0,
			EndSeconds:      round3(duration),
			DurationSeconds: round3(duration),
			Confidence:      "low",
			ConfidenceScore: 0.24,
			Basis:           []string{"duration_template", strategy + " reference"},
			RequiresConfirm: true,
		}}
	}
	sectionCount := templateSectionCount(duration)
	points := []float64{0}
	for i := 1; i < sectionCount; i++ {
		points = append(points, round3(duration*float64(i)/float64(sectionCount)))
	}
	points = append(points, round3(duration))
	sections := []SectionCandidate{}
	for i := 0; i < len(points)-1; i++ {
		start := points[i]
		end := points[i+1]
		labelHint := sectionLabelHint(i, len(points)-1)
		sections = append(sections, SectionCandidate{
			SectionID:       fmt.Sprintf("section_%02d", i+1),
			Label:           sectionDisplayLabel(labelHint, i),
			LabelHint:       labelHint,
			StartSeconds:    round3(start),
			EndSeconds:      round3(end),
			DurationSeconds: round3(end - start),
			Confidence:      "low",
			ConfidenceScore: 0.22,
			Basis:           []string{"duration_template", "DAD time-segment evidence unavailable or unclear"},
			RequiresConfirm: true,
		})
	}
	return sections
}

func averagedBucketWindow(buckets []timelineBucket, start, end int) timelineBucket {
	if start < 0 {
		start = 0
	}
	if end > len(buckets) {
		end = len(buckets)
	}
	if start >= end {
		return timelineBucket{}
	}
	out := timelineBucket{}
	for i := start; i < end; i++ {
		out.ActiveCount += buckets[i].ActiveCount
		out.EnergyAvg += buckets[i].EnergyAvg
	}
	count := float64(end - start)
	out.ActiveCount = int(math.Round(float64(out.ActiveCount) / count))
	out.EnergyAvg = out.EnergyAvg / count
	return out
}

func normalizedDelta(before, after any) float64 {
	b := floatFromAny(before)
	a := floatFromAny(after)
	denom := math.Max(math.Abs(b), math.Abs(a))
	if denom < 0.0001 {
		return 0
	}
	return math.Abs(a-b) / denom
}

func sectionBucketSize(duration float64) float64 {
	switch {
	case duration <= 0:
		return 0
	case duration <= 60:
		return 4
	case duration <= 180:
		return 8
	case duration <= 360:
		return 12
	default:
		return 16
	}
}

func maxSectionBoundaryCount(duration float64) int {
	switch {
	case duration <= 90:
		return 3
	case duration <= 240:
		return 5
	default:
		return 7
	}
}

func templateSectionCount(duration float64) int {
	switch {
	case duration <= 45:
		return 2
	case duration <= 120:
		return 3
	case duration <= 240:
		return 5
	default:
		return 6
	}
}

func sectionLabelHint(index, total int) string {
	if total <= 1 {
		return "full"
	}
	if index == 0 {
		return "intro"
	}
	if index == total-1 {
		return "outro"
	}
	if total >= 5 {
		switch index {
		case 1:
			return "section_a"
		case 2:
			return "section_b"
		case 3:
			return "section_c"
		}
	}
	return fmt.Sprintf("section_%c", 'A'+rune(index-1))
}

func sectionDisplayLabel(hint string, index int) string {
	switch hint {
	case "full":
		return "完整段落"
	case "intro":
		return "Intro"
	case "outro":
		return "Outro"
	case "section_a":
		return "Section A"
	case "section_b":
		return "Section B"
	case "section_c":
		return "Section C"
	default:
		if strings.HasPrefix(hint, "section_") && len(hint) > len("section_") {
			return "Section " + strings.ToUpper(strings.TrimPrefix(hint, "section_"))
		}
		return fmt.Sprintf("Section %d", index+1)
	}
}

func sectionMapConfidence(evidence SectionEvidence, sectionCount int) string {
	if sectionCount == 0 {
		return "none"
	}
	if evidence.TimeSegmentTrackCount >= 3 && evidence.BoundaryCandidateCount >= 2 {
		return "medium"
	}
	if evidence.TimeSegmentTrackCount > 0 && evidence.BoundaryCandidateCount > 0 {
		return "low_medium"
	}
	return "low"
}

func sectionConfidenceLabel(score float64) string {
	switch {
	case score >= 0.72:
		return "high"
	case score >= 0.5:
		return "medium"
	default:
		return "low"
	}
}

func sectionCoverageSeconds(sections []SectionCandidate) float64 {
	total := 0.0
	for _, section := range sections {
		if section.EndSeconds > section.StartSeconds {
			total += section.EndSeconds - section.StartSeconds
		}
	}
	return total
}

func uniqueSortedFloats(values []float64) []float64 {
	sort.Float64s(values)
	out := []float64{}
	for _, value := range values {
		value = round3(value)
		if len(out) == 0 || math.Abs(out[len(out)-1]-value) > 0.001 {
			out = append(out, value)
		}
	}
	return out
}

func segmentEnergy(segment timeEnergySegment) float64 {
	if segment.RMS > 0 {
		return segment.RMS
	}
	if segment.RMSDBFS != 0 {
		return math.Pow(10, segment.RMSDBFS/20)
	}
	switch strings.ToLower(strings.TrimSpace(segment.EnergyState)) {
	case "high":
		return 0.2
	case "medium":
		return 0.08
	case "low":
		return 0.015
	case "silent":
		return 0.0001
	default:
		return 0
	}
}

func compactSections(sections []SectionCandidate, maxItems int) []map[string]any {
	out := []map[string]any{}
	for _, section := range sections {
		out = append(out, map[string]any{
			"section_id":       section.SectionID,
			"label":            section.Label,
			"label_hint":       section.LabelHint,
			"start_seconds":    section.StartSeconds,
			"end_seconds":      section.EndSeconds,
			"duration_seconds": section.DurationSeconds,
			"confidence":       section.Confidence,
			"basis":            section.Basis,
		})
		if maxItems > 0 && len(out) >= maxItems {
			break
		}
	}
	return out
}

func totalClipCount(profiles []clipProfile) int {
	total := 0
	for _, profile := range profiles {
		total += maxInt(profile.ClipCount, 1)
	}
	return total
}

func maxProfileDuration(profiles []clipProfile, summary map[string]any) float64 {
	maxDuration := firstPositiveNumber(summary, "edit_length_seconds", "duration_seconds", "max_duration_seconds", "longest_duration_seconds", "timeline_length_seconds")
	for _, profile := range profiles {
		if profile.Duration > maxDuration {
			maxDuration = profile.Duration
		}
	}
	return maxDuration
}

func candidateFromProfile(profile clipProfile, kind, severity, recommendation, reason string, requiresConfirm bool) EditCandidate {
	evidence := []string{}
	if profile.HasStart {
		evidence = append(evidence, fmt.Sprintf("start %.2fs", profile.StartSeconds))
	}
	if profile.HasDuration {
		evidence = append(evidence, fmt.Sprintf("duration %.2fs", profile.Duration))
	}
	if profile.RMSDBFS != nil {
		evidence = append(evidence, fmt.Sprintf("rms %.1f dBFS", *profile.RMSDBFS))
	}
	if profile.PeakDBFS != nil {
		evidence = append(evidence, fmt.Sprintf("peak %.1f dBFS", *profile.PeakDBFS))
	}
	evidence = evidenceRefs(evidence...)
	var pending *PendingActionPlan
	if kind == "fade_boundary_review" && strings.TrimSpace(profile.ClipID) != "" {
		pending = pendingClipFadeAction(profile, evidence)
	}
	return EditCandidate{
		TrackID:         profile.TrackID,
		TrackName:       profile.TrackName,
		ClipID:          profile.ClipID,
		ClipName:        profile.ClipName,
		Kind:            kind,
		Severity:        severity,
		Recommendation:  recommendation,
		Reason:          reason,
		Evidence:        evidence,
		RequiresConfirm: requiresConfirm,
		PendingAction:   pending,
	}
}

func pendingClipFadeAction(profile clipProfile, evidence []string) *PendingActionPlan {
	fadeSeconds := 0.005
	if profile.HasDuration && profile.Duration > 0 {
		fadeSeconds = math.Min(0.005, math.Max(0.001, profile.Duration*0.002))
	}
	after := map[string]any{
		"fade_in_seconds":  round3(fadeSeconds),
		"fade_out_seconds": round3(fadeSeconds),
		"fade_in_curve":    "linear",
		"fade_out_curve":   "linear",
	}
	return &PendingActionPlan{
		ToolName: "clip.fade.set",
		TargetIDs: map[string]any{
			"track_id": profile.TrackID,
			"clip_id":  profile.ClipID,
		},
		Args: map[string]any{
			"clip_id":          profile.ClipID,
			"fade_in_seconds":  after["fade_in_seconds"],
			"fade_out_seconds": after["fade_out_seconds"],
			"fade_in_curve":    after["fade_in_curve"],
			"fade_out_curve":   after["fade_out_curve"],
		},
		Before: map[string]any{
			"read_tool": "clip.fade.read",
			"clip_id":   profile.ClipID,
		},
		After: after,
		TimeRange: map[string]any{
			"clip_start_seconds": profile.StartSeconds,
			"clip_duration_seconds": func() any {
				if profile.HasDuration {
					return round3(profile.Duration)
				}
				return nil
			}(),
		},
		EvidenceRefs: evidenceRefs(evidence...),
		Risk:         "confirm",
		RollbackHint: "Read clip.fade before applying; rollback by restoring the previous fade_in_seconds/fade_out_seconds and fade metadata.",
	}
}

func compactCandidates(candidates []EditCandidate, maxItems int) []map[string]any {
	out := []map[string]any{}
	for _, candidate := range candidates {
		out = append(out, map[string]any{
			"track_id":         candidate.TrackID,
			"track_name":       candidate.TrackName,
			"clip_id":          candidate.ClipID,
			"kind":             candidate.Kind,
			"severity":         candidate.Severity,
			"recommendation":   candidate.Recommendation,
			"requires_confirm": candidate.RequiresConfirm,
			"pending_action":    candidate.PendingAction,
		})
		if maxItems > 0 && len(out) >= maxItems {
			break
		}
	}
	return out
}

func capCandidates(candidates []EditCandidate, maxItems int) []EditCandidate {
	if maxItems <= 0 || len(candidates) <= maxItems {
		return candidates
	}
	return append([]EditCandidate(nil), candidates[:maxItems]...)
}

func summaryMD(proj Projection) string {
	return fmt.Sprintf(
		"EPM %s status=%s tracks=%d clips=%d cleanup=%s preserve_alignment=%t fade=%s section_map=%s/%s. Do not edit clips or markers without user confirmation.",
		proj.EPMVersion,
		proj.Status,
		proj.ClipCleanup.TrackCount,
		proj.ClipCleanup.ClipCount,
		proj.ClipCleanup.Recommendation,
		proj.ClipCleanup.PreserveStemAlignment,
		proj.ClipCleanup.FadeRecommendation,
		proj.SectionMap.Status,
		proj.SectionMap.ReferenceStrategy,
	)
}

func indexDADRows(rows []map[string]any) (map[string]map[string]any, map[string]map[string]any) {
	byTrackClip := map[string]map[string]any{}
	bySource := map[string]map[string]any{}
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		if key := trackClipKey(fieldString(row, "track_id", "id"), fieldString(row, "clip_id", "primary_clip_id", "item_id")); key != "" {
			byTrackClip[key] = row
		}
		if source := normalizePath(firstNonEmpty(fieldString(row, "source_path", "file_path", "source_file_path", "current_source_path"))); source != "" {
			bySource[source] = row
		}
	}
	return byTrackClip, bySource
}

func dadForProfile(profile clipProfile, byTrackClip, bySource map[string]map[string]any) map[string]any {
	if key := trackClipKey(profile.TrackID, profile.ClipID); key != "" {
		if row := byTrackClip[key]; len(row) > 0 {
			return row
		}
	}
	if source := normalizePath(profile.SourcePath); source != "" {
		return bySource[source]
	}
	return nil
}

func mergeDADFacts(profile *clipProfile, row map[string]any) {
	if profile == nil || len(row) == 0 {
		return
	}
	status := strings.ToLower(fieldString(row, "status"))
	profile.AcousticReady = status == "ready"
	if !profile.HasDuration {
		if value, ok := firstNumber(row, "duration_seconds", "total_duration", "length_seconds"); ok && value > 0 {
			profile.Duration = value
			profile.HasDuration = true
		}
	}
	for _, target := range []struct {
		keys []string
		set  func(float64)
	}{
		{[]string{"rms_dbfs"}, func(v float64) { profile.RMSDBFS = &v }},
		{[]string{"peak_dbfs"}, func(v float64) { profile.PeakDBFS = &v }},
		{[]string{"headroom_db"}, func(v float64) { profile.HeadroomDB = &v }},
	} {
		if value, ok := firstNumber(row, target.keys...); ok {
			rounded := round3(value)
			target.set(rounded)
		}
	}
	profile.TimeSegments = timeSegmentsFromAny(row["time_segments"])
}

func timeSegmentsFromAny(value any) []timeEnergySegment {
	rows := rowsFromAny(value)
	out := make([]timeEnergySegment, 0, len(rows))
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		start, hasStart := firstNumber(row, "start_seconds", "start", "time_start_seconds")
		end, hasEnd := firstNumber(row, "end_seconds", "end", "time_end_seconds")
		if !hasEnd {
			if length, ok := firstNumber(row, "duration_seconds", "length_seconds"); ok {
				end = start + length
				hasEnd = true
			}
		}
		if !hasStart || !hasEnd || end <= start {
			continue
		}
		segment := timeEnergySegment{
			StartSeconds: round3(start),
			EndSeconds:   round3(end),
			EnergyState:  fieldString(row, "energy_state", "activity_state"),
		}
		if value, ok := firstNumber(row, "rms"); ok {
			segment.RMS = value
		}
		if value, ok := firstNumber(row, "rms_dbfs"); ok {
			segment.RMSDBFS = value
		}
		if value, ok := firstNumber(row, "peak_abs", "peak"); ok {
			segment.PeakAbs = value
		}
		if value, ok := firstNumber(row, "peak_dbfs"); ok {
			segment.PeakDBFS = value
		}
		out = append(out, segment)
	}
	return out
}

func profileLooksSilent(profile clipProfile) bool {
	if profile.RMSDBFS != nil && *profile.RMSDBFS <= -80 {
		return true
	}
	return profile.PeakDBFS != nil && *profile.PeakDBFS <= -90
}

func profileLooksHot(profile clipProfile) bool {
	if profile.PeakDBFS != nil && *profile.PeakDBFS >= -0.1 {
		return true
	}
	return profile.HeadroomDB != nil && *profile.HeadroomDB <= 0.1
}

func sourceExtension(pathOrName string) string {
	name := strings.ReplaceAll(strings.TrimSpace(pathOrName), "\\", "/")
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")
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

func trackClipKey(trackID, clipID string) string {
	trackID = strings.TrimSpace(trackID)
	clipID = strings.TrimSpace(clipID)
	if trackID == "" && clipID == "" {
		return ""
	}
	return trackID + "::" + clipID
}

func normalizePath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.ToLower(strings.ReplaceAll(value, "\\", "/"))
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
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func text(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "<nil>" {
			return ""
		}
		return text
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

func floatFromAny(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case int32:
		return float64(typed)
	case uint:
		return float64(typed)
	case uint64:
		return float64(typed)
	default:
		if parsed, ok := numberFromAny(value); ok {
			return parsed
		}
	}
	return 0
}

func rowsFromAny(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return typed
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if row, ok := item.(map[string]any); ok && len(row) > 0 {
				out = append(out, row)
			}
		}
		return out
	default:
		return nil
	}
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

func round3(value float64) float64 {
	if value == 0 {
		return 0
	}
	return math.Round(value*1000) / 1000
}

func clampFloat(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func evidenceRefs(values ...string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
