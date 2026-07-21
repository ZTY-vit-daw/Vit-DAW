package tom

import (
	"encoding/json"
	"fmt"
	"math"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const (
	statusReady   = "ready"
	statusPartial = "partial"
	statusMissing = "missing"

	confidenceHigh   = "high"
	confidenceMedium = "medium"
	confidenceLow    = "low"

	maxContextAssignmentsPerGroup = 12

	disclosureStageNamingID       = "naming_id"
	disclosureStageTechnical      = "technical_cluster"
	disclosureStageDADLightweight = "dad_lightweight"
	disclosureStatusSelected      = "selected"
	disclosureStatusAvailable     = "available"
	disclosureStatusSkipped       = "skipped"
	manifestCoverageComplete      = "complete"
	manifestCoveragePartial       = "partial"
)

type roleRule struct {
	groupID     string
	label       string
	folder      string
	role        string
	tokens      []string
	phrases     []string
	substrings  []string
	score       float64
	routingHint string
}

type trackProfile struct {
	TrackID       string
	TrackName     string
	ClipID        string
	ClipName      string
	SourcePath    string
	Duration      float64
	StartSeconds  float64
	SampleRateHz  float64
	BitDepth      int
	ChannelCount  int
	FormatFamily  string
	SourceExt     string
	AcousticReady bool
	RMSDBFS       *float64
	PeakDBFS      *float64
	HeadroomDB    *float64
	BalanceDB     *float64
	Correlation   *float64
	Tokens        []string
	CompactText   string
}

type assignmentDecision struct {
	groupID       string
	label         string
	folder        string
	role          string
	score         float64
	evidence      []Evidence
	clusterKeys   []string
	routingHint   string
	namingMatched bool
	needsReview   bool
}

var roleRules = []roleRule{
	{
		groupID: "backing_vocals", label: "Backing Vocals", folder: "Backing Vocals", role: "backing_vocal",
		tokens:     []string{"bv", "bgv", "bvox", "backing", "harmony", "harmonies", "adlib", "adlibs"},
		phrases:    []string{"backing vocal", "backing vocals", "back vox", "background vocal", "background vocals", "bg vocal", "bg vocals"},
		substrings: []string{"bgvox", "backvox"},
		score:      0.94, routingHint: "folder_or_summing_stack",
	},
	{
		groupID: "vocals", label: "Vocals", folder: "Vocals", role: "lead_vocal",
		tokens:     []string{"vocal", "vocals", "vox", "voice", "lv"},
		phrases:    []string{"lead vocal", "lead vocals", "lead vox", "main vocal", "main vocals", "main vox"},
		substrings: []string{"leadvox", "ldvox", "mainvox"},
		score:      0.92, routingHint: "folder_or_summing_stack",
	},
	{
		groupID: "drums", label: "Drums", folder: "Drums", role: "drums_or_percussion",
		tokens: []string{"drum", "drums", "kick", "kik", "snare", "hat", "hihat", "hh", "clap", "perc", "percussion", "tom", "toms", "cym", "cymbal", "cymbals", "crash", "ride", "shaker"},
		score:  0.91, routingHint: "summing_stack_preferred",
	},
	{
		groupID: "bass", label: "Bass", folder: "Bass", role: "bass",
		tokens: []string{"bass", "sub", "808"},
		score:  0.90, routingHint: "folder_or_summing_stack",
	},
	{
		groupID: "strings", label: "Strings", folder: "Strings", role: "strings",
		tokens: []string{"str", "string", "strings", "violin", "violins", "viola", "violas", "cello", "celli", "orch", "orchestra"},
		score:  0.88, routingHint: "folder_or_summing_stack",
	},
	{
		groupID: "synths", label: "Synths", folder: "Synths", role: "synth_or_pad",
		tokens: []string{"synth", "synths", "syn", "pad", "pads", "arp", "arps", "pluck", "plucks"},
		score:  0.86, routingHint: "folder_or_summing_stack",
	},
	{
		groupID: "guitars", label: "Guitars", folder: "Guitars", role: "guitar",
		tokens: []string{"guitar", "guitars", "gtr", "gtrs", "guit", "acg", "egtr"},
		score:  0.86, routingHint: "folder_or_summing_stack",
	},
	{
		groupID: "keys", label: "Keys", folder: "Keys", role: "keys_or_piano",
		tokens: []string{"keys", "key", "piano", "rhodes", "organ", "wurli", "ep", "clav"},
		score:  0.86, routingHint: "folder_or_summing_stack",
	},
	{
		groupID: "fx", label: "FX", folder: "FX", role: "effect_or_transition",
		tokens: []string{"fx", "sfx", "riser", "risers", "sweep", "sweeps", "impact", "impacts", "downlifter", "uplifter", "reverse", "noise", "transition"},
		score:  0.84, routingHint: "folder_track",
	},
	{
		groupID: "returns", label: "Returns", folder: "Returns", role: "fx_return",
		tokens: []string{"verb", "reverb", "delay", "echo", "return", "send", "plate", "hall"},
		score:  0.82, routingHint: "folder_or_aux_return_group",
	},
	{
		groupID: "buses_prints", label: "Buses / Prints", folder: "Buses / Prints", role: "bus_or_print",
		tokens: []string{"bus", "buss", "stem", "print", "printed", "group", "grp", "sum", "submix"},
		score:  0.78, routingHint: "folder_track_review_routing",
	},
}

var groupOrder = []string{
	"vocals", "backing_vocals", "drums", "bass", "guitars", "keys", "synths", "strings",
	"fx", "returns", "buses_prints", "silent_candidates", "hot_clipping_review",
	"mono_sources", "short_clips", "long_stereo_stems", "needs_review",
}

var ignoredNameTokens = map[string]bool{
	"audio": true, "aud": true, "track": true, "trk": true, "clip": true, "take": true,
	"stem": true, "stems": true, "file": true, "new": true, "copy": true, "bounce": true,
	"wav": true, "wave": true, "aif": true, "aiff": true, "mp3": true,
}

func BuildFromImportRows(input ImportInput) Projection {
	profiles := buildTrackProfiles(input)
	maxDuration := maxProfileDuration(profiles, input.Summary)
	groups := map[string]*GroupProposal{}
	needsReview := []TrackAssignment{}
	allAssignments := []TrackAssignment{}
	summary := OrganizationSummary{
		TrackCount:      len(profiles),
		GroupCounts:     map[string]int{},
		PrimaryStrategy: "name_id_first_then_technical_fallback",
	}

	for _, profile := range profiles {
		decision := decideAssignment(profile, maxDuration)
		assignment := TrackAssignment{
			TrackID:         profile.TrackID,
			TrackName:       profile.TrackName,
			ClipID:          profile.ClipID,
			ClipName:        profile.ClipName,
			SourcePath:      profile.SourcePath,
			GroupID:         decision.groupID,
			GroupLabel:      decision.label,
			RoleHypothesis:  decision.role,
			Confidence:      confidenceLabel(decision.score),
			ConfidenceScore: round3(decision.score),
			Evidence:        compactEvidence(decision.evidence, 8),
			ClusterKeys:     uniqueStrings(decision.clusterKeys...),
		}
		allAssignments = append(allAssignments, assignment)
		if decision.namingMatched {
			summary.NamingMatchedTrackCount++
		} else {
			summary.TechnicalFallbackTrackCount++
		}
		switch assignment.Confidence {
		case confidenceHigh:
			summary.HighConfidenceTrackCount++
		case confidenceMedium:
			summary.MediumConfidenceTrackCount++
		default:
			summary.LowConfidenceTrackCount++
		}
		if decision.needsReview || assignment.Confidence == confidenceLow {
			summary.NeedsReviewTrackCount++
			needsReview = append(needsReview, assignment)
		}
		summary.GroupCounts[decision.label]++
		group := ensureGroup(groups, decision)
		group.Assignments = append(group.Assignments, assignment)
		group.TrackCount = len(group.Assignments)
		group.ConfidenceScore = round3(weightedGroupConfidence(group.Assignments))
		group.Confidence = confidenceLabel(group.ConfidenceScore)
		group.NeedsConfirmation = group.Confidence != confidenceHigh || decision.needsReview
		group.Basis = mergeEvidence(group.Basis, decision.evidence, 6)
	}

	groupList := sortedGroups(groups)
	summary.ProposedGroupCount = len(groupList)
	namingReport := buildNamingSignalReport(profiles, allAssignments)
	fullManifest := buildFullAssignmentManifest(groupList, len(profiles))
	disclosurePlan := buildDisclosurePlan(summary, namingReport, input.DADWaveformRows)
	status := projectionStatus(summary)
	limitations := buildLimitations(input, summary)
	proj := Projection{
		SchemaVersion:       SchemaVersion,
		TOMVersion:          Version,
		ObservationID:       strings.TrimSpace(input.ObservationID),
		MixSessionID:        strings.TrimSpace(input.MixSessionID),
		Status:              status,
		OrganizationSummary: summary,
		DisclosurePlan:      disclosurePlan,
		NamingSignalReport:  namingReport,
		FullManifest:        fullManifest,
		GroupProposals:      groupList,
		NeedsReviewTracks:   capAssignments(needsReview, 24),
		EvidenceRefs:        evidenceRefs("import:track_refs", "tim:projection", "dad:track_waveform_envelopes"),
		Limitations:         limitations,
		GeneratedAt:         strings.TrimSpace(input.CreatedAt),
	}
	proj.LLMContext = BuildLLMContext(proj, input.TIMProjection)
	return proj
}

func buildTrackProfiles(input ImportInput) []trackProfile {
	dadByTrackClip, dadBySource := indexDADRows(input.DADWaveformRows)
	out := make([]trackProfile, 0, len(input.Rows))
	for i, row := range input.Rows {
		if len(row) == 0 {
			continue
		}
		profile := trackProfile{
			TrackID:      firstNonEmpty(fieldString(row, "track_id", "id"), fmt.Sprintf("track_%03d", i+1)),
			TrackName:    firstNonEmpty(fieldString(row, "track_name", "name"), fieldString(row, "clip_name", "file_name")),
			ClipID:       firstNonEmpty(fieldString(row, "clip_id", "primary_clip_id", "item_id"), fmt.Sprintf("clip_%03d", i+1)),
			ClipName:     firstNonEmpty(fieldString(row, "clip_name", "file_name", "name")),
			SourcePath:   firstNonEmpty(fieldString(row, "source_file_path", "current_source_path", "source_path", "file_path", "imported_file_path", "copied_file_path", "path")),
			Duration:     firstPositiveNumber(row, "length_seconds", "duration_seconds", "duration", "edit_length_seconds"),
			StartSeconds: firstNumberOrZero(row, "start_time_seconds", "start_seconds", "position_seconds"),
			SampleRateHz: firstPositiveNumber(row, "sample_rate_hz", "sample_rate", "source_sample_rate_hz", "source_sample_rate"),
			BitDepth:     firstPositiveInt(row, "bit_depth", "bits_per_sample", "source_bit_depth"),
			ChannelCount: firstPositiveInt(row, "channel_count", "channels", "num_channels", "source_channel_count"),
			FormatFamily: firstNonEmpty(fieldString(row, "format_family"), formatFamilyFromExt(sourceExtension(firstNonEmpty(fieldString(row, "source_file_path", "source_path", "file_path"), fieldString(row, "clip_name", "file_name"))))),
		}
		profile.SourceExt = sourceExtension(firstNonEmpty(profile.SourcePath, profile.ClipName))
		if dad := dadForProfile(profile, dadByTrackClip, dadBySource); len(dad) > 0 {
			mergeDADFacts(&profile, dad)
		}
		profile.Tokens, profile.CompactText = profileNameTokens(profile)
		out = append(out, profile)
	}
	return out
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

func dadForProfile(profile trackProfile, byTrackClip, bySource map[string]map[string]any) map[string]any {
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

func mergeDADFacts(profile *trackProfile, row map[string]any) {
	if profile == nil || len(row) == 0 {
		return
	}
	status := strings.ToLower(fieldString(row, "status"))
	profile.AcousticReady = status == "ready"
	if profile.Duration <= 0 {
		profile.Duration = firstPositiveNumber(row, "duration_seconds", "total_duration", "length_seconds")
	}
	if profile.SampleRateHz <= 0 {
		profile.SampleRateHz = firstPositiveNumber(row, "sample_rate_hz", "sample_rate")
	}
	if profile.ChannelCount <= 0 {
		profile.ChannelCount = firstPositiveInt(row, "channel_count", "channels")
	}
	for _, target := range []struct {
		keys []string
		set  func(float64)
	}{
		{[]string{"rms_dbfs"}, func(v float64) { profile.RMSDBFS = &v }},
		{[]string{"peak_dbfs"}, func(v float64) { profile.PeakDBFS = &v }},
		{[]string{"headroom_db"}, func(v float64) { profile.HeadroomDB = &v }},
		{[]string{"balance_db"}, func(v float64) { profile.BalanceDB = &v }},
		{[]string{"correlation_estimate", "correlation"}, func(v float64) { profile.Correlation = &v }},
	} {
		if value, ok := firstNumber(row, target.keys...); ok {
			rounded := round3(value)
			target.set(rounded)
		}
	}
}

func decideAssignment(profile trackProfile, maxDuration float64) assignmentDecision {
	if match, ok := matchNamingRole(profile); ok {
		match.evidence = append(match.evidence, technicalEvidence(profile, maxDuration)...)
		match.evidence = append(match.evidence, acousticEvidence(profile)...)
		return match
	}
	return technicalFallback(profile, maxDuration)
}

func matchNamingRole(profile trackProfile) (assignmentDecision, bool) {
	tokenSet := map[string]bool{}
	for _, token := range profile.Tokens {
		tokenSet[token] = true
	}
	best := assignmentDecision{}
	bestScore := 0.0
	for _, rule := range roleRules {
		score := 0.0
		evidence := []Evidence{}
		for _, phrase := range rule.phrases {
			if containsPhrase(profile.CompactText, phrase) {
				score = math.Max(score, rule.score+0.03)
				evidence = append(evidence, Evidence{Kind: "name_phrase", Detail: phrase, Weight: rule.score})
			}
		}
		for _, token := range rule.tokens {
			if tokenSet[token] {
				score = math.Max(score, rule.score)
				evidence = append(evidence, Evidence{Kind: "name_token", Detail: token, Weight: rule.score})
			}
		}
		compactNoSpace := strings.ReplaceAll(profile.CompactText, " ", "")
		for _, substring := range rule.substrings {
			if substring != "" && strings.Contains(compactNoSpace, substring) {
				score = math.Max(score, rule.score)
				evidence = append(evidence, Evidence{Kind: "name_compound", Detail: substring, Weight: rule.score})
			}
		}
		if score > 0 && len(evidence) > 1 {
			score += 0.03
		}
		if score > bestScore {
			bestScore = score
			best = assignmentDecision{
				groupID:       rule.groupID,
				label:         rule.label,
				folder:        rule.folder,
				role:          rule.role,
				score:         clamp(score, 0, 0.98),
				evidence:      compactEvidence(evidence, 6),
				clusterKeys:   []string{"naming:" + rule.groupID},
				routingHint:   rule.routingHint,
				namingMatched: true,
			}
		}
	}
	return best, bestScore > 0
}

func technicalFallback(profile trackProfile, maxDuration float64) assignmentDecision {
	evidence := append(acousticEvidence(profile), technicalEvidence(profile, maxDuration)...)
	switch {
	case profileLooksSilent(profile):
		return assignmentDecision{
			groupID: "silent_candidates", label: "Silent / Empty Candidates", folder: "Silent / Empty Candidates", role: "silence_or_empty_candidate",
			score: 0.64, evidence: evidence, clusterKeys: []string{"acoustic:near_silence"}, routingHint: "folder_track_review", needsReview: true,
		}
	case profileLooksHot(profile):
		return assignmentDecision{
			groupID: "hot_clipping_review", label: "Hot / Clipping Review", folder: "Hot / Clipping Review", role: "hot_or_clipping_candidate",
			score: 0.62, evidence: evidence, clusterKeys: []string{"acoustic:hot_or_clipping"}, routingHint: "folder_track_review", needsReview: true,
		}
	}
	switch {
	case profile.ChannelCount == 1:
		return assignmentDecision{
			groupID: "mono_sources", label: "Mono Sources", folder: "Mono Sources", role: "mono_source_unknown",
			score: 0.58, evidence: evidence, clusterKeys: []string{"technical:mono"}, routingHint: "folder_track", needsReview: true,
		}
	case profile.Duration > 0 && profile.Duration < 15:
		return assignmentDecision{
			groupID: "short_clips", label: "Short Clips / FX Candidates", folder: "Short Clips", role: "short_clip_unknown",
			score: 0.52, evidence: evidence, clusterKeys: []string{"technical:short_clip"}, routingHint: "folder_track", needsReview: true,
		}
	case profile.Duration > 0 && maxDuration > 0 && profile.Duration >= maxDuration*0.95 && profile.ChannelCount == 2:
		return assignmentDecision{
			groupID: "long_stereo_stems", label: "Long Stereo Stems", folder: "Long Stereo Stems", role: "long_stereo_stem_unknown",
			score: 0.50, evidence: evidence, clusterKeys: []string{"technical:long_stereo"}, routingHint: "folder_track", needsReview: true,
		}
	default:
		return assignmentDecision{
			groupID: "needs_review", label: "Needs Review", folder: "Needs Review", role: "unknown",
			score: 0.35, evidence: evidence, clusterKeys: []string{"technical:insufficient_naming_signal"}, routingHint: "folder_track", needsReview: true,
		}
	}
}

func technicalEvidence(profile trackProfile, maxDuration float64) []Evidence {
	evidence := []Evidence{}
	if profile.Duration > 0 {
		evidence = append(evidence, Evidence{Kind: "duration", Detail: fmt.Sprintf("%.2fs", profile.Duration), Weight: 0.35})
		if maxDuration > 0 && profile.Duration >= maxDuration*0.95 {
			evidence = append(evidence, Evidence{Kind: "duration_cluster", Detail: "near project-long stem", Weight: 0.35})
		}
	}
	if profile.ChannelCount > 0 {
		evidence = append(evidence, Evidence{Kind: "channel_count", Detail: fmt.Sprintf("%dch", profile.ChannelCount), Weight: 0.25})
	}
	if profile.FormatFamily != "" && profile.FormatFamily != "unknown" {
		evidence = append(evidence, Evidence{Kind: "format_family", Detail: profile.FormatFamily, Weight: 0.15})
	}
	if profile.SourceExt != "" {
		evidence = append(evidence, Evidence{Kind: "file_format", Detail: profile.SourceExt, Weight: 0.1})
	}
	return evidence
}

func acousticEvidence(profile trackProfile) []Evidence {
	evidence := []Evidence{}
	if profile.AcousticReady {
		evidence = append(evidence, Evidence{Kind: "dad_lightweight_fact", Detail: "DAD acoustic summary ready", Weight: 0.2})
	}
	if profile.RMSDBFS != nil {
		evidence = append(evidence, Evidence{Kind: "rms_dbfs", Detail: fmt.Sprintf("%.1f dBFS", *profile.RMSDBFS), Weight: 0.25})
	}
	if profile.PeakDBFS != nil {
		evidence = append(evidence, Evidence{Kind: "peak_dbfs", Detail: fmt.Sprintf("%.1f dBFS", *profile.PeakDBFS), Weight: 0.25})
	}
	if profile.HeadroomDB != nil {
		evidence = append(evidence, Evidence{Kind: "headroom_db", Detail: fmt.Sprintf("%.1f dB", *profile.HeadroomDB), Weight: 0.2})
	}
	if profile.BalanceDB != nil {
		evidence = append(evidence, Evidence{Kind: "balance_db", Detail: fmt.Sprintf("%.1f dB", *profile.BalanceDB), Weight: 0.12})
	}
	if profile.Correlation != nil {
		evidence = append(evidence, Evidence{Kind: "correlation_estimate", Detail: fmt.Sprintf("%.2f", *profile.Correlation), Weight: 0.12})
	}
	if profileLooksSilent(profile) {
		evidence = append(evidence, Evidence{Kind: "acoustic_review", Detail: "near digital silence", Weight: 0.4})
	}
	if profileLooksHot(profile) {
		evidence = append(evidence, Evidence{Kind: "acoustic_review", Detail: "peak or headroom near 0 dBFS", Weight: 0.35})
	}
	return compactEvidence(evidence, 8)
}

func profileLooksSilent(profile trackProfile) bool {
	if profile.RMSDBFS != nil && *profile.RMSDBFS <= -80 {
		return true
	}
	return profile.PeakDBFS != nil && *profile.PeakDBFS <= -90
}

func profileLooksHot(profile trackProfile) bool {
	if profile.PeakDBFS != nil && *profile.PeakDBFS >= -0.1 {
		return true
	}
	return profile.HeadroomDB != nil && *profile.HeadroomDB <= 0.1
}

func ensureGroup(groups map[string]*GroupProposal, decision assignmentDecision) *GroupProposal {
	group := groups[decision.groupID]
	if group != nil {
		return group
	}
	group = &GroupProposal{
		GroupID:           decision.groupID,
		Label:             decision.label,
		ProposedFolder:    firstNonEmpty(decision.folder, decision.label),
		RoleHypothesis:    decision.role,
		Confidence:        confidenceLabel(decision.score),
		ConfidenceScore:   round3(decision.score),
		Basis:             compactEvidence(decision.evidence, 6),
		NeedsConfirmation: decision.score < 0.82 || decision.needsReview,
		RoutingHint:       decision.routingHint,
	}
	groups[decision.groupID] = group
	return group
}

func sortedGroups(groups map[string]*GroupProposal) []GroupProposal {
	order := map[string]int{}
	for i, id := range groupOrder {
		order[id] = i
	}
	out := make([]GroupProposal, 0, len(groups))
	for _, group := range groups {
		out = append(out, *group)
	}
	sort.Slice(out, func(i, j int) bool {
		oi, iok := order[out[i].GroupID]
		oj, jok := order[out[j].GroupID]
		if iok && jok && oi != oj {
			return oi < oj
		}
		if iok != jok {
			return iok
		}
		if out[i].TrackCount != out[j].TrackCount {
			return out[i].TrackCount > out[j].TrackCount
		}
		return out[i].Label < out[j].Label
	})
	return out
}

func projectionStatus(summary OrganizationSummary) string {
	switch {
	case summary.TrackCount == 0:
		return statusMissing
	case summary.NamingMatchedTrackCount == 0:
		return statusPartial
	case summary.NeedsReviewTrackCount > 0:
		return statusPartial
	default:
		return statusReady
	}
}

func buildLimitations(input ImportInput, summary OrganizationSummary) []string {
	limits := []string{}
	if summary.TrackCount == 0 {
		limits = append(limits, "tom_import_rows_unavailable")
	}
	if summary.TechnicalFallbackTrackCount > 0 {
		limits = append(limits, "tom_used_technical_fallback_for_tracks_without_name_signal")
	}
	if summary.NeedsReviewTrackCount > 0 {
		limits = append(limits, "tom_low_confidence_groups_require_user_confirmation")
	}
	if len(input.DADWaveformRows) == 0 {
		limits = append(limits, "dad_lightweight_facts_not_available_to_tom")
	}
	if len(input.TIMProjection) == 0 {
		limits = append(limits, "tim_projection_not_available_to_tom")
	} else {
		if status := strings.ToLower(fieldString(input.TIMProjection, "status")); status != "" && status != statusReady {
			limits = append(limits, "tim_projection_status_"+status)
		}
		if risk := strings.ToLower(fieldString(mapValue(input.TIMProjection["risk_summary"]), "overall_risk")); risk != "" && risk != "none" {
			limits = append(limits, "tim_projection_risk_"+risk)
		}
	}
	return evidenceRefs(limits...)
}

func buildNamingSignalReport(profiles []trackProfile, assignments []TrackAssignment) NamingSignalReport {
	report := NamingSignalReport{
		TrackCount:              len(profiles),
		NamingMatchedTrackCount: 0,
		IgnoredTokens:           []string{},
	}
	if len(profiles) == 0 {
		return report
	}
	namingMatched := map[string]bool{}
	for _, assignment := range assignments {
		trackID := stableAssignmentTrackID(assignment)
		if trackID == "" || !assignmentHasNamingEvidence(assignment) {
			continue
		}
		namingMatched[trackID] = true
	}
	report.NamingMatchedTrackCount = len(namingMatched)

	tokenClusters := map[string]*nameClusterAccumulator{}
	prefixClusters := map[string]*nameClusterAccumulator{}
	ignoredSeen := map[string]bool{}
	for _, profile := range profiles {
		trackID := firstNonEmpty(profile.TrackID, profile.ClipID)
		if trackID == "" {
			continue
		}
		displayName := profileDisplayName(profile)
		for _, token := range profile.Tokens {
			switch {
			case isNumericToken(token):
				continue
			case ignoredNameTokens[token]:
				ignoredSeen[token] = true
				continue
			default:
				addNameCluster(tokenClusters, token, trackID, displayName)
			}
		}
		if key := profileNameStructureKey(profile); key != "" {
			addNameCluster(prefixClusters, key, trackID, displayName)
		}
	}

	structuralSignal := map[string]bool{}
	similarityClusters := []SimilarityCluster{}
	for _, cluster := range sortedNameClusters(prefixClusters, 0, 2) {
		meaningful := nameStructureKeyMeaningful(cluster.Token)
		ids := append([]string(nil), cluster.TrackIDs...)
		if meaningful {
			for _, trackID := range ids {
				structuralSignal[trackID] = true
			}
		}
		similarityClusters = append(similarityClusters, SimilarityCluster{
			Key:          cluster.Token,
			Count:        cluster.Count,
			TrackIDs:     ids,
			ExampleNames: append([]string(nil), cluster.ExampleNames...),
			Meaningful:   meaningful,
		})
		if len(similarityClusters) >= 12 {
			break
		}
	}

	signalTracks := map[string]bool{}
	for trackID := range namingMatched {
		signalTracks[trackID] = true
	}
	for trackID := range structuralSignal {
		signalTracks[trackID] = true
	}
	report.StructuralNameTrackCount = len(structuralSignal)
	report.NamingSignalTrackCount = len(signalTracks)
	report.NamingSignalCoverage = round3(float64(len(signalTracks)) / float64(len(profiles)))
	report.WeakNameTrackCount = len(profiles) - len(signalTracks)
	report.TopTokens = sortedNameClusters(tokenClusters, 12, 1)
	report.TokenClusters = sortedNameClusters(tokenClusters, 12, 2)
	report.CommonPrefixes = sortedNameClusters(prefixClusters, 8, 2)
	report.SimilarityClusters = similarityClusters
	report.IgnoredTokens = sortedBoolKeys(ignoredSeen)
	return report
}

type nameClusterAccumulator struct {
	Token        string
	TrackIDs     []string
	ExampleNames []string
	seenTracks   map[string]bool
	seenExamples map[string]bool
}

func addNameCluster(clusters map[string]*nameClusterAccumulator, token, trackID, exampleName string) {
	token = strings.TrimSpace(token)
	trackID = strings.TrimSpace(trackID)
	if token == "" || trackID == "" {
		return
	}
	cluster := clusters[token]
	if cluster == nil {
		cluster = &nameClusterAccumulator{
			Token:        token,
			seenTracks:   map[string]bool{},
			seenExamples: map[string]bool{},
		}
		clusters[token] = cluster
	}
	if !cluster.seenTracks[trackID] {
		cluster.seenTracks[trackID] = true
		cluster.TrackIDs = append(cluster.TrackIDs, trackID)
	}
	exampleName = strings.TrimSpace(exampleName)
	if exampleName != "" && !cluster.seenExamples[exampleName] && len(cluster.ExampleNames) < 4 {
		cluster.seenExamples[exampleName] = true
		cluster.ExampleNames = append(cluster.ExampleNames, exampleName)
	}
}

func sortedNameClusters(clusters map[string]*nameClusterAccumulator, maxItems int, minCount int) []TokenCluster {
	out := []TokenCluster{}
	for _, cluster := range clusters {
		count := len(cluster.TrackIDs)
		if count < minCount {
			continue
		}
		out = append(out, TokenCluster{
			Token:        cluster.Token,
			Count:        count,
			TrackIDs:     append([]string(nil), cluster.TrackIDs...),
			ExampleNames: append([]string(nil), cluster.ExampleNames...),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Token < out[j].Token
	})
	if maxItems > 0 && len(out) > maxItems {
		return append([]TokenCluster(nil), out[:maxItems]...)
	}
	return out
}

func assignmentHasNamingEvidence(assignment TrackAssignment) bool {
	for _, key := range assignment.ClusterKeys {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(key)), "naming:") {
			return true
		}
	}
	for _, item := range assignment.Evidence {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(item.Kind)), "name_") {
			return true
		}
	}
	return false
}

func stableAssignmentTrackID(assignment TrackAssignment) string {
	return firstNonEmpty(assignment.TrackID, assignment.ClipID)
}

func profileDisplayName(profile trackProfile) string {
	return firstNonEmpty(profile.TrackName, profile.ClipName, baseName(profile.SourcePath), profile.TrackID)
}

func profileNameStructureKey(profile trackProfile) string {
	candidates := []string{
		profile.TrackName,
		profile.ClipName,
		baseName(profile.SourcePath),
		profile.TrackID,
	}
	for _, candidate := range candidates {
		tokens := tokenizeName(candidate)
		if len(tokens) == 0 {
			continue
		}
		for len(tokens) > 0 && isNumericToken(tokens[len(tokens)-1]) {
			tokens = tokens[:len(tokens)-1]
		}
		keyTokens := []string{}
		for _, token := range tokens {
			if token == "" || isNumericToken(token) || ignoredNameTokens[token] {
				continue
			}
			keyTokens = append(keyTokens, token)
		}
		if len(keyTokens) > 0 {
			return strings.Join(keyTokens, " ")
		}
	}
	return ""
}

func nameStructureKeyMeaningful(key string) bool {
	for _, token := range tokenizeName(key) {
		if token != "" && !isNumericToken(token) && !ignoredNameTokens[token] {
			return true
		}
	}
	return false
}

func isNumericToken(token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	for _, r := range token {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func sortedBoolKeys(values map[string]bool) []string {
	out := []string{}
	for key, ok := range values {
		if ok && strings.TrimSpace(key) != "" {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

func buildFullAssignmentManifest(groups []GroupProposal, trackCount int) FullAssignmentManifest {
	manifest := FullAssignmentManifest{
		TrackCount:     trackCount,
		CoverageStatus: manifestCoveragePartial,
		Groups:         []ManifestGroup{},
	}
	seen := map[string]int{}
	for _, group := range groups {
		manifestGroup := ManifestGroup{
			GroupID:           group.GroupID,
			Label:             group.Label,
			ProposedFolder:    group.ProposedFolder,
			RoleHypothesis:    group.RoleHypothesis,
			Confidence:        group.Confidence,
			ConfidenceScore:   group.ConfidenceScore,
			NeedsConfirmation: group.NeedsConfirmation,
			RoutingHint:       group.RoutingHint,
			TrackIDs:          []string{},
			Assignments:       []ManifestAssignment{},
		}
		for _, assignment := range group.Assignments {
			trackID := stableAssignmentTrackID(assignment)
			if trackID == "" {
				continue
			}
			manifest.AssignmentCoverageCount++
			seen[trackID]++
			manifestGroup.TrackIDs = append(manifestGroup.TrackIDs, trackID)
			manifestGroup.Assignments = append(manifestGroup.Assignments, ManifestAssignment{
				TrackID:         trackID,
				TrackName:       assignment.TrackName,
				ClipID:          assignment.ClipID,
				GroupID:         assignment.GroupID,
				GroupLabel:      assignment.GroupLabel,
				Confidence:      assignment.Confidence,
				ConfidenceScore: assignment.ConfidenceScore,
				ClusterKeys:     append([]string(nil), assignment.ClusterKeys...),
			})
		}
		manifestGroup.TrackCount = len(manifestGroup.TrackIDs)
		manifest.Groups = append(manifest.Groups, manifestGroup)
	}
	for trackID, count := range seen {
		if count > 0 {
			manifest.UniqueTrackCount++
		}
		if count > 1 {
			manifest.DuplicateTrackIDs = append(manifest.DuplicateTrackIDs, trackID)
		}
	}
	sort.Strings(manifest.DuplicateTrackIDs)
	if manifest.AssignmentCoverageCount < trackCount {
		manifest.MissingTrackIDs = []string{fmt.Sprintf("<unknown_missing_%d>", trackCount-manifest.AssignmentCoverageCount)}
	}
	if manifest.AssignmentCoverageCount == trackCount && manifest.UniqueTrackCount == trackCount && len(manifest.DuplicateTrackIDs) == 0 {
		manifest.CoverageStatus = manifestCoverageComplete
	}
	return manifest
}

func buildDisclosurePlan(summary OrganizationSummary, namingReport NamingSignalReport, dadRows []map[string]any) DisclosurePlan {
	selected := disclosureStageNamingID
	reason := "naming/id signals cover the sortable import rows"
	roleCoverage := 0.0
	if summary.TrackCount > 0 {
		roleCoverage = float64(summary.NamingMatchedTrackCount) / float64(summary.TrackCount)
	}
	dadReady := dadReadyTrackCount(dadRows)
	switch {
	case summary.TrackCount == 0:
		selected = disclosureStageNamingID
		reason = "no import rows are available"
	case roleCoverage >= 0.70 && summary.TechnicalFallbackTrackCount <= maxInt(2, int(math.Ceil(float64(summary.TrackCount)*0.20))):
		selected = disclosureStageNamingID
		reason = "role-oriented name/id evidence is strong enough for the first disclosure stage"
	case summary.TechnicalFallbackTrackCount > 0 && dadReady > 0:
		selected = disclosureStageDADLightweight
		reason = "weak names require technical clustering plus lightweight DAD facts"
	case summary.TechnicalFallbackTrackCount > 0:
		selected = disclosureStageTechnical
		reason = "weak names require technical clustering"
	default:
		selected = disclosureStageNamingID
		reason = "all sortable tracks have name/id role evidence"
	}

	stages := []DisclosureStage{
		{
			Stage:         disclosureStageNamingID,
			Status:        disclosureStageStatus(disclosureStageNamingID, selected, true),
			Reason:        "track id/name/clip/source basename token and similarity analysis",
			TrackCount:    summary.TrackCount,
			CoverageCount: namingReport.NamingSignalTrackCount,
		},
		{
			Stage:         disclosureStageTechnical,
			Status:        disclosureStageStatus(disclosureStageTechnical, selected, summary.TechnicalFallbackTrackCount > 0),
			Reason:        "duration, channel count, format and stem-length clustering for weak names",
			TrackCount:    summary.TrackCount,
			CoverageCount: summary.TechnicalFallbackTrackCount,
		},
		{
			Stage:         disclosureStageDADLightweight,
			Status:        disclosureStageStatus(disclosureStageDADLightweight, selected, dadReady > 0),
			Reason:        "lightweight DAD waveform facts for silence, hot peaks, balance and correlation review",
			TrackCount:    summary.TrackCount,
			CoverageCount: dadReady,
		},
	}
	return DisclosurePlan{
		SelectedStage: selected,
		Reason:        reason,
		Stages:        stages,
	}
}

func disclosureStageStatus(stage, selected string, available bool) string {
	if stage == selected {
		return disclosureStatusSelected
	}
	if available {
		return disclosureStatusAvailable
	}
	return disclosureStatusSkipped
}

func dadReadyTrackCount(rows []map[string]any) int {
	seen := map[string]bool{}
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		status := strings.ToLower(fieldString(row, "status"))
		if status != "" && status != statusReady {
			continue
		}
		trackID := firstNonEmpty(fieldString(row, "track_id", "id"), fieldString(row, "clip_id", "primary_clip_id", "item_id"))
		if trackID == "" {
			continue
		}
		seen[trackID] = true
	}
	return len(seen)
}

func BuildLLMContext(proj Projection, timProjection ...map[string]any) LLMContext {
	facts := []map[string]any{
		{
			"layer":                      "organization_summary",
			"status":                     proj.Status,
			"track_count":                proj.OrganizationSummary.TrackCount,
			"proposed_group_count":       proj.OrganizationSummary.ProposedGroupCount,
			"naming_matched_track_count": proj.OrganizationSummary.NamingMatchedTrackCount,
			"technical_fallback_count":   proj.OrganizationSummary.TechnicalFallbackTrackCount,
			"needs_review_track_count":   proj.OrganizationSummary.NeedsReviewTrackCount,
			"primary_strategy":           proj.OrganizationSummary.PrimaryStrategy,
			"disclosure_selected_stage":  proj.DisclosurePlan.SelectedStage,
			"assignment_coverage_count":  proj.FullManifest.AssignmentCoverageCount,
			"assignment_coverage_status": proj.FullManifest.CoverageStatus,
		},
	}
	if len(timProjection) > 0 {
		if fact := compactTIMProjectionFact(timProjection[0]); len(fact) > 0 {
			facts = append(facts, fact)
		}
	}
	if fact := compactDisclosurePlanFact(proj.DisclosurePlan); len(fact) > 0 {
		facts = append(facts, fact)
	}
	if fact := compactNamingSignalReportFact(proj.NamingSignalReport); len(fact) > 0 {
		facts = append(facts, fact)
	}
	facts = append(facts, map[string]any{
		"layer":  "group_proposals",
		"groups": compactGroups(proj.GroupProposals, 12, 0),
	})
	if fact := compactFullManifestFact(proj.FullManifest); len(fact) > 0 {
		facts = append(facts, fact)
	}
	if len(proj.NeedsReviewTracks) > 0 {
		facts = append(facts, map[string]any{
			"layer":  "needs_review_excerpt",
			"tracks": compactAssignments(proj.NeedsReviewTracks, 12),
		})
	}
	return LLMContext{
		SummaryMD:              summaryMD(proj),
		CompactFacts:           facts,
		DoNotIncludeRawPackage: true,
		EvidenceRefs:           proj.EvidenceRefs,
		LimitationNotes:        proj.Limitations,
		SuggestedNextStep:      "Ask the user whether to apply the proposed folder/group organization before writing any track layout changes.",
	}
}

func compactTIMProjectionFact(proj map[string]any) map[string]any {
	if len(proj) == 0 {
		return nil
	}
	fact := map[string]any{"layer": "tim_projection"}
	if status := fieldString(proj, "status"); status != "" {
		fact["status"] = status
	}
	summary := mapValue(proj["technical_summary"])
	for _, key := range []string{"track_count", "clip_count", "source_present_count", "playback_invalid_count", "acoustic_ready_track_count"} {
		if value, ok := summary[key]; ok {
			fact[key] = value
		}
	}
	risk := mapValue(proj["risk_summary"])
	for _, key := range []string{"overall_risk", "issue_count", "primary_codes"} {
		if value, ok := risk[key]; ok {
			fact[key] = value
		}
	}
	if len(fact) == 1 {
		return nil
	}
	return fact
}

func compactDisclosurePlanFact(plan DisclosurePlan) map[string]any {
	if plan.SelectedStage == "" && len(plan.Stages) == 0 {
		return nil
	}
	stages := []map[string]any{}
	for _, stage := range plan.Stages {
		stages = append(stages, map[string]any{
			"stage":          stage.Stage,
			"status":         stage.Status,
			"coverage_count": stage.CoverageCount,
			"track_count":    stage.TrackCount,
			"reason":         stage.Reason,
		})
	}
	return map[string]any{
		"layer":          "disclosure_plan",
		"selected_stage": plan.SelectedStage,
		"reason":         plan.Reason,
		"stages":         stages,
	}
}

func compactNamingSignalReportFact(report NamingSignalReport) map[string]any {
	if report.TrackCount == 0 {
		return nil
	}
	return map[string]any{
		"layer":                       "naming_signal_report",
		"track_count":                 report.TrackCount,
		"naming_matched_track_count":  report.NamingMatchedTrackCount,
		"structural_name_track_count": report.StructuralNameTrackCount,
		"naming_signal_track_count":   report.NamingSignalTrackCount,
		"naming_signal_coverage":      report.NamingSignalCoverage,
		"weak_name_track_count":       report.WeakNameTrackCount,
		"top_tokens":                  compactTokenClusters(report.TopTokens, 8),
		"token_clusters":              compactTokenClusters(report.TokenClusters, 8),
		"similarity_clusters":         compactSimilarityClusters(report.SimilarityClusters, 8),
		"ignored_tokens":              report.IgnoredTokens,
	}
}

func compactFullManifestFact(manifest FullAssignmentManifest) map[string]any {
	groups := []map[string]any{}
	for _, group := range manifest.Groups {
		if group.TrackCount == 0 {
			continue
		}
		groups = append(groups, map[string]any{
			"group_id":           group.GroupID,
			"label":              group.Label,
			"proposed_folder":    group.ProposedFolder,
			"confidence":         group.Confidence,
			"track_count":        group.TrackCount,
			"track_ids_csv":      strings.Join(group.TrackIDs, ", "),
			"needs_confirmation": group.NeedsConfirmation,
			"routing_hint":       group.RoutingHint,
		})
	}
	return map[string]any{
		"layer":                     "full_assignment_manifest",
		"track_count":               manifest.TrackCount,
		"assignment_coverage_count": manifest.AssignmentCoverageCount,
		"unique_track_count":        manifest.UniqueTrackCount,
		"coverage_status":           manifest.CoverageStatus,
		"groups":                    groups,
		"duplicate_track_ids":       manifest.DuplicateTrackIDs,
		"missing_track_ids":         manifest.MissingTrackIDs,
	}
}

func compactTokenClusters(clusters []TokenCluster, maxItems int) []map[string]any {
	out := []map[string]any{}
	for _, cluster := range clusters {
		out = append(out, map[string]any{
			"token":         cluster.Token,
			"count":         cluster.Count,
			"track_ids_csv": strings.Join(cluster.TrackIDs, ", "),
			"example_names": cluster.ExampleNames,
		})
		if maxItems > 0 && len(out) >= maxItems {
			break
		}
	}
	return out
}

func compactSimilarityClusters(clusters []SimilarityCluster, maxItems int) []map[string]any {
	out := []map[string]any{}
	for _, cluster := range clusters {
		out = append(out, map[string]any{
			"key":           cluster.Key,
			"count":         cluster.Count,
			"track_ids_csv": strings.Join(cluster.TrackIDs, ", "),
			"example_names": cluster.ExampleNames,
			"meaningful":    cluster.Meaningful,
		})
		if maxItems > 0 && len(out) >= maxItems {
			break
		}
	}
	return out
}

func summaryMD(proj Projection) string {
	return fmt.Sprintf(
		"TOM %s status=%s tracks=%d groups=%d naming=%d technical_fallback=%d needs_review=%d disclosure=%s manifest=%d/%d %s. Role labels are hypotheses; folder proposals require user confirmation.",
		proj.TOMVersion,
		proj.Status,
		proj.OrganizationSummary.TrackCount,
		proj.OrganizationSummary.ProposedGroupCount,
		proj.OrganizationSummary.NamingMatchedTrackCount,
		proj.OrganizationSummary.TechnicalFallbackTrackCount,
		proj.OrganizationSummary.NeedsReviewTrackCount,
		proj.DisclosurePlan.SelectedStage,
		proj.FullManifest.AssignmentCoverageCount,
		proj.FullManifest.TrackCount,
		proj.FullManifest.CoverageStatus,
	)
}

func ContextProjection(proj Projection) map[string]any {
	return map[string]any{
		"schema_version":           proj.SchemaVersion,
		"tom_version":              proj.TOMVersion,
		"observation_id":           proj.ObservationID,
		"mix_session_id":           proj.MixSessionID,
		"status":                   proj.Status,
		"organization_summary":     proj.OrganizationSummary,
		"disclosure_plan":          proj.DisclosurePlan,
		"naming_signal_report":     proj.NamingSignalReport,
		"full_assignment_manifest": proj.FullManifest,
		"group_proposals":          compactGroups(proj.GroupProposals, 24, maxContextAssignmentsPerGroup),
		"needs_review_excerpt":     compactAssignments(proj.NeedsReviewTracks, 24),
		"limitations":              proj.Limitations,
		"llm_context":              proj.LLMContext,
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

func compactGroups(groups []GroupProposal, maxGroups int, maxAssignments int) []map[string]any {
	out := []map[string]any{}
	for _, group := range groups {
		row := map[string]any{
			"group_id":           group.GroupID,
			"label":              group.Label,
			"proposed_folder":    group.ProposedFolder,
			"role_hypothesis":    group.RoleHypothesis,
			"confidence":         group.Confidence,
			"confidence_score":   group.ConfidenceScore,
			"track_count":        group.TrackCount,
			"basis":              compactEvidence(group.Basis, 6),
			"needs_confirmation": group.NeedsConfirmation,
			"routing_hint":       group.RoutingHint,
		}
		if maxAssignments > 0 {
			row["assignment_excerpt"] = compactAssignments(group.Assignments, maxAssignments)
			if omitted := len(group.Assignments) - maxAssignments; omitted > 0 {
				row["assignment_omitted_count"] = omitted
			}
		}
		out = append(out, row)
		if maxGroups > 0 && len(out) >= maxGroups {
			break
		}
	}
	return out
}

func compactAssignments(assignments []TrackAssignment, maxItems int) []map[string]any {
	out := []map[string]any{}
	for _, assignment := range assignments {
		row := map[string]any{
			"track_id":         assignment.TrackID,
			"track_name":       assignment.TrackName,
			"clip_id":          assignment.ClipID,
			"group_id":         assignment.GroupID,
			"group_label":      assignment.GroupLabel,
			"role_hypothesis":  assignment.RoleHypothesis,
			"confidence":       assignment.Confidence,
			"confidence_score": assignment.ConfidenceScore,
			"cluster_keys":     assignment.ClusterKeys,
		}
		out = append(out, row)
		if maxItems > 0 && len(out) >= maxItems {
			break
		}
	}
	return out
}

func profileNameTokens(profile trackProfile) ([]string, string) {
	values := []string{
		profile.TrackID,
		profile.TrackName,
		profile.ClipID,
		profile.ClipName,
		baseName(profile.SourcePath),
	}
	seen := map[string]bool{}
	tokens := []string{}
	for _, value := range values {
		for _, token := range tokenizeName(value) {
			if token != "" && !seen[token] {
				seen[token] = true
				tokens = append(tokens, token)
			}
		}
	}
	sort.Strings(tokens)
	return tokens, " " + strings.Join(tokens, " ") + " "
}

func tokenizeName(value string) []string {
	value = insertCamelBoundaries(strings.TrimSpace(value))
	if value == "" {
		return nil
	}
	var b strings.Builder
	for _, r := range value {
		r = unicode.ToLower(r)
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	return strings.Fields(b.String())
}

func insertCamelBoundaries(value string) string {
	var out strings.Builder
	var prev rune
	for i, r := range value {
		if i > 0 && unicode.IsLower(prev) && unicode.IsUpper(r) {
			out.WriteByte(' ')
		}
		out.WriteRune(r)
		prev = r
	}
	return out.String()
}

func containsPhrase(compactText, phrase string) bool {
	phraseTokens := tokenizeName(phrase)
	if len(phraseTokens) == 0 {
		return false
	}
	return strings.Contains(compactText, " "+strings.Join(phraseTokens, " ")+" ")
}

func baseName(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	if value == "" {
		return ""
	}
	base := path.Base(value)
	ext := path.Ext(base)
	if ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	return base
}

func maxProfileDuration(profiles []trackProfile, summary map[string]any) float64 {
	maxDuration := firstPositiveNumber(summary, "edit_length_seconds", "duration_seconds", "max_duration_seconds", "longest_duration_seconds", "timeline_length_seconds")
	for _, profile := range profiles {
		if profile.Duration > maxDuration {
			maxDuration = profile.Duration
		}
	}
	return maxDuration
}

func weightedGroupConfidence(assignments []TrackAssignment) float64 {
	if len(assignments) == 0 {
		return 0
	}
	total := 0.0
	for _, assignment := range assignments {
		total += assignment.ConfidenceScore
	}
	return total / float64(len(assignments))
}

func confidenceLabel(score float64) string {
	switch {
	case score >= 0.82:
		return confidenceHigh
	case score >= 0.55:
		return confidenceMedium
	default:
		return confidenceLow
	}
}

func capAssignments(assignments []TrackAssignment, maxItems int) []TrackAssignment {
	if maxItems <= 0 || len(assignments) <= maxItems {
		return assignments
	}
	return append([]TrackAssignment(nil), assignments[:maxItems]...)
}

func mergeEvidence(existing []Evidence, incoming []Evidence, maxItems int) []Evidence {
	out := append([]Evidence(nil), existing...)
	seen := map[string]bool{}
	for _, item := range out {
		seen[item.Kind+"|"+item.Detail] = true
	}
	for _, item := range incoming {
		key := item.Kind + "|" + item.Detail
		if item.Kind == "" || item.Detail == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
		if maxItems > 0 && len(out) >= maxItems {
			break
		}
	}
	return out
}

func compactEvidence(evidence []Evidence, maxItems int) []Evidence {
	out := []Evidence{}
	seen := map[string]bool{}
	for _, item := range evidence {
		key := item.Kind + "|" + item.Detail
		if item.Kind == "" || item.Detail == "" || seen[key] {
			continue
		}
		seen[key] = true
		item.Weight = round3(item.Weight)
		out = append(out, item)
		if maxItems > 0 && len(out) >= maxItems {
			break
		}
	}
	return out
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

func firstNumberOrZero(row map[string]any, keys ...string) float64 {
	value, _ := firstNumber(row, keys...)
	return value
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

func round3(value float64) float64 {
	if value == 0 {
		return 0
	}
	return math.Round(value*1000) / 1000
}

func clamp(value, minValue, maxValue float64) float64 {
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

func uniqueStrings(values ...string) []string {
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

var _ = rowsFromAny
