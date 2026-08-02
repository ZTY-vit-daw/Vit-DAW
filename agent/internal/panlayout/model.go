package panlayout

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/capabilityruntime"
	"vit-daw-agent/internal/mixstyle"
	"vit-daw-agent/internal/staticbalance"
)

func BuildModel(in Input) Model {
	generatedAt := in.GeneratedAt.UTC()
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	style := mixstyle.Normalize(in.Style)
	if mixstyle.Validate(style) != nil {
		style = mixstyle.Default()
	}
	base := staticbalance.BuildModel(staticbalance.Input{
		UserIntent: in.UserIntent, ProjectState: in.ProjectState, MixObservation: in.MixObservation,
		MOMProjection: in.MOMProjection,
		TOMProjection: in.TOMProjection, ContextSnapshot: in.ContextSnapshot,
		RequestContext: in.RequestContext, ExecutionMemory: in.ExecutionMemory,
		Style: style, StyleExplicit: in.StyleExplicit, GeneratedAt: generatedAt,
	})
	rows := projectRows(in.ProjectState, in.ContextSnapshot, in.RequestContext)
	rowsByID := map[string]map[string]any{}
	for _, row := range rows {
		if id := text(row, "track_id", "id"); id != "" {
			rowsByID[id] = row
		}
	}
	stereoByID := collectStereoEvidence(in.MOMProjection, in.MixObservation)
	tracks := make([]Track, 0, len(base.Tracks))
	for _, source := range base.Tracks {
		row := rowsByID[source.TrackID]
		track := Track{
			TrackID: source.TrackID, TrackName: source.TrackName, TrackType: source.TrackType,
			ParentTrackID: text(row, "parent_track_id", "parent_folder_track_id"),
			Role:          source.Role, Function: source.Function, RoleSource: source.RoleSource,
			RoleConfidence: source.RoleConfidence, CenterAnchor: centerAnchorRole(source.Role),
			PairKey: pairKey(source.TrackName, source.Role),
		}
		track.CurrentPan, _ = number(row, "pan", "pan_value", "track_pan")
		track.ChannelCount = channelCount(row)
		switch track.ChannelCount {
		case 1:
			track.SourceFormat = "mono"
		case 2:
			track.SourceFormat = "stereo"
		default:
			track.SourceFormat = "unknown"
		}
		if evidence := stereoByID[track.TrackID]; len(evidence) > 0 {
			track.StereoBalanceDB, _ = number(evidence, "balance_db", "stereo_balance_db")
			track.Correlation, _ = number(evidence, "correlation_estimate", "correlation", "stereo_correlation")
		}
		track.Eligible, track.Exclusion, track.RiskFlags = panEligibility(source, track)
		tracks = append(tracks, track)
	}
	markPairEligibility(tracks)
	coverage := measureCoverage(tracks)
	layout := summarizeLayout(tracks)
	mom := firstMap(in.MOMProjection, mapValue(in.MixObservation["mom_projection"]), mapValue(mapValue(in.MixObservation["observation"])["mom_projection"]))
	refs := []string{"project.state"}
	if len(mom) > 0 {
		refs = append(refs, "mix.observe", "MOM")
	}
	if len(in.TOMProjection) > 0 || len(base.TOMSummary) > 0 {
		refs = append(refs, "TOM")
	}
	if coverage.StereoEvidenceKnownCount > 0 {
		refs = append(refs, "stereo_relation_summary")
	}
	observationID := firstText(in.MixObservation, "observation_id")
	readiness := assessReadiness(tracks, coverage, mom, style, refs)
	limitations := append(append([]string(nil), readiness.BlockedBy...), readiness.Warnings...)
	modelFingerprint := map[string]any{"tracks": tracks, "style_hash": mixstyle.Hash(style), "observation_id": observationID}
	model := Model{
		SchemaVersion: ModelSchemaVersion, CapabilityID: CapabilityID,
		GeneratedAt: generatedAt.Format(time.RFC3339), UserIntent: compact(in.UserIntent, 256),
		ObservationID: observationID,
		Style: Style{SchemaVersion: style.SchemaVersion, ID: style.ID, Name: style.Name,
			Version: style.Identity.Version, Hash: mixstyle.Hash(style), Explicit: in.StyleExplicit,
			Dimensions: style.Capabilities.PanLayout.PanLayoutDimensions},
		Tracks: tracks, Coverage: coverage, Layout: layout, Readiness: readiness,
		EvidenceRefs: unique(refs), Limitations: unique(limitations),
		MOMSummary: compactMap(mom, "mom_version", "intent", "observation_id", "multitrack_relation", "trust_quality"),
		TOMSummary: base.TOMSummary,
	}
	model.ModelID = stableID("plm", modelFingerprint)
	return model
}

func assessReadiness(tracks []Track, c Coverage, mom map[string]any, style mixstyle.MixStyle, refs []string) capabilityruntime.Readiness {
	conditions := []capabilityruntime.Condition{
		condition("project_structure", true, len(tracks) > 0, len(tracks), len(tracks), "project.state contains the full track inventory", refs, "Refresh project.state before B3."),
		condition("current_pan_state", true, c.PanKnownCount >= 2 && c.PanCoverage >= .95, c.PanKnownCount, c.RoleCandidateCount, fmt.Sprintf("current track-pan coverage is %.0f%%", c.PanCoverage*100), refs, "Refresh project.state so B3 can fingerprint current pan values."),
		condition("track_role_relationships", true, c.RoleKnownCount >= 2 && c.RoleCoverage >= .55, c.RoleKnownCount, c.RoleCandidateCount, fmt.Sprintf("functional role coverage is %.0f%%", c.RoleCoverage*100), refs, "Refresh TOM or confirm ambiguous roles."),
		condition("layout_relationships", true, c.PairEligibleTrackCount >= 2 || c.CenterAnchorCount >= 1, c.PairEligibleTrackCount+c.CenterAnchorCount, len(tracks), "at least one center anchor or complementary role group is available", refs, "Confirm center anchors or complementary/doubled parts."),
	}
	momReady := len(mom) > 0 && !strings.EqualFold(text(mapValue(mom["multitrack_relation"]), "status"), "missing")
	conditions = append(conditions, condition("mom_multitrack_relationships", true, momReady, boolInt(momReady), 1, "full-project MOM relationship evidence is available", refs, "Refresh a full-project mix.observe MOM projection."))
	styleReady := mixstyle.Validate(style) == nil
	conditions = append(conditions, condition("mix_style_pan_layout", true, styleReady, boolInt(styleReady), 1, "VMS pan_layout capability schema and ranges are valid", refs, "Select or repair a valid vit.mix_style.v1 file."))
	conditions = append(conditions, capabilityruntime.Condition{ID: "source_channel_identity", Required: false, Status: warningStatus(c.ChannelCoverage >= .5), Summary: fmt.Sprintf("source channel-format coverage is %.0f%%; unknown sources use conservative pan bounds", c.ChannelCoverage*100), KnownCount: c.ChannelKnownCount, TotalCount: c.RoleCandidateCount, EvidenceRefs: refs})
	conditions = append(conditions, capabilityruntime.Condition{ID: "stereo_mono_risk_evidence", Required: false, Status: warningStatus(c.StereoEvidenceCoverage >= .25), Summary: fmt.Sprintf("track stereo relation coverage is %.0f%%; risky stereo sources without evidence remain unchanged", c.StereoEvidenceCoverage*100), KnownCount: c.StereoEvidenceKnownCount, TotalCount: c.RoleCandidateCount, EvidenceRefs: refs})
	return capabilityruntime.Evaluate(CapabilityID, conditions)
}

func condition(id string, required, ready bool, known, total int, summary string, refs []string, remediation string) capabilityruntime.Condition {
	status := capabilityruntime.ConditionBlocked
	if ready {
		status = capabilityruntime.ConditionReady
	}
	return capabilityruntime.Condition{ID: id, Required: required, Status: status, KnownCount: known, TotalCount: total, Summary: summary, EvidenceRefs: refs, Remediation: []string{remediation}}
}

func warningStatus(ready bool) string {
	if ready {
		return capabilityruntime.ConditionReady
	}
	return capabilityruntime.ConditionWarning
}

func panEligibility(base staticbalance.Track, track Track) (bool, string, []string) {
	if !base.Eligible {
		return false, base.Exclusion, nil
	}
	if track.CurrentPan == nil {
		return false, "pan_state_missing", nil
	}
	risks := []string{}
	if track.SourceFormat == "stereo" && track.Correlation != nil && *track.Correlation < .15 {
		risks = append(risks, "low_stereo_correlation")
		return false, "stereo_mono_risk", risks
	}
	if track.SourceFormat == "stereo" && track.Correlation == nil {
		risks = append(risks, "stereo_relation_missing")
	}
	if track.SourceFormat == "unknown" {
		risks = append(risks, "source_channel_unknown")
	}
	return true, "", risks
}

func markPairEligibility(tracks []Track) {
	counts := map[string]int{}
	for i := range tracks {
		if tracks[i].Eligible && tracks[i].PairKey != "" && !tracks[i].CenterAnchor {
			counts[tracks[i].PairKey]++
		}
	}
	for i := range tracks {
		if tracks[i].Eligible && !tracks[i].CenterAnchor && counts[tracks[i].PairKey] < 2 {
			tracks[i].RiskFlags = append(tracks[i].RiskFlags, "unpaired_role_member")
		}
	}
}

func measureCoverage(tracks []Track) Coverage {
	out := Coverage{AnalyzedTrackCount: len(tracks)}
	functions, pairs := map[string]bool{}, map[string]int{}
	for _, t := range tracks {
		candidate := t.Exclusion == "" || t.Exclusion == "role_unresolved" || t.Exclusion == "role_confidence_low" || t.Exclusion == "pan_state_missing" || t.Exclusion == "stereo_mono_risk"
		if candidate {
			out.RoleCandidateCount++
			if t.Role != "" && t.Role != "unknown" {
				out.RoleKnownCount++
			}
			if t.CurrentPan != nil {
				out.PanKnownCount++
			}
			if t.ChannelCount > 0 {
				out.ChannelKnownCount++
			}
			if t.Correlation != nil || t.StereoBalanceDB != nil {
				out.StereoEvidenceKnownCount++
			}
		}
		if t.Eligible {
			out.EligibleTrackCount++
			functions[t.Function] = true
			if t.CenterAnchor {
				out.CenterAnchorCount++
			}
			if t.PairKey != "" && !t.CenterAnchor {
				pairs[t.PairKey]++
			}
		}
	}
	for _, count := range pairs {
		if count >= 2 {
			out.PairEligibleTrackCount += count
		}
	}
	out.FunctionCount = len(functions)
	if out.RoleCandidateCount > 0 {
		total := float64(out.RoleCandidateCount)
		out.RoleCoverage = round3(float64(out.RoleKnownCount) / total)
		out.PanCoverage = round3(float64(out.PanKnownCount) / total)
		out.ChannelCoverage = round3(float64(out.ChannelKnownCount) / total)
		out.StereoEvidenceCoverage = round3(float64(out.StereoEvidenceKnownCount) / total)
	}
	return out
}

func summarizeLayout(tracks []Track) LayoutSummary {
	out := LayoutSummary{}
	pairs := map[string]int{}
	count := 0
	for _, t := range tracks {
		if t.CurrentPan == nil {
			continue
		}
		count++
		value := *t.CurrentPan
		out.MeanPan += value
		out.MeanAbsPan += math.Abs(value)
		switch {
		case value < -.05:
			out.LeftCount++
		case value > .05:
			out.RightCount++
		default:
			out.CenterCount++
		}
		if t.PairKey != "" {
			pairs[t.PairKey]++
		}
		if contains(t.RiskFlags, "low_stereo_correlation") || contains(t.RiskFlags, "stereo_relation_missing") {
			out.StereoRiskCount++
		}
	}
	if count > 0 {
		out.MeanPan = round3(out.MeanPan / float64(count))
		out.MeanAbsPan = round3(out.MeanAbsPan / float64(count))
	}
	for _, n := range pairs {
		if n >= 2 {
			out.PairGroupCount++
		}
	}
	return out
}

func centerAnchorRole(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "lead_vocal", "lead_instrument", "kick", "snare", "bass", "sub_bass", "low_synth":
		return true
	default:
		return false
	}
}

func pairKey(name, role string) string {
	_ = name
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" || role == "unknown" || centerAnchorRole(role) {
		return ""
	}
	return role
}

func channelCount(row map[string]any) int {
	if value, ok := number(row, "channel_count", "channels", "source_channel_count"); ok && *value > 0 {
		return int(*value)
	}
	for _, clip := range rowsValue(row["clips"]) {
		if value, ok := number(clip, "channel_count", "channels", "source_channel_count"); ok && *value > 0 {
			return int(*value)
		}
	}
	return 0
}

func collectStereoEvidence(sources ...map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	var walk func(any, int)
	walk = func(value any, depth int) {
		if depth > 10 {
			return
		}
		switch v := value.(type) {
		case map[string]any:
			id := text(v, "track_id", "id")
			if id != "" {
				if _, ok1 := number(v, "correlation_estimate", "correlation", "stereo_correlation"); ok1 {
					out[id] = v
				} else if _, ok2 := number(v, "balance_db", "stereo_balance_db"); ok2 {
					out[id] = v
				}
			}
			for _, child := range v {
				walk(child, depth+1)
			}
		case []any:
			for _, child := range v {
				walk(child, depth+1)
			}
		case []map[string]any:
			for _, child := range v {
				walk(child, depth+1)
			}
		}
	}
	for _, source := range sources {
		walk(source, 0)
	}
	return out
}

func projectRows(sources ...map[string]any) []map[string]any {
	for _, source := range sources {
		for _, candidate := range []map[string]any{source, mapValue(source["daw_state_summary"]), mapValue(source["project_state"])} {
			if rows := rowsValue(candidate["tracks"]); len(rows) > 0 {
				return rows
			}
		}
	}
	return nil
}

func mapValue(v any) map[string]any {
	if out, ok := v.(map[string]any); ok {
		return out
	}
	return map[string]any{}
}
func rowsValue(v any) []map[string]any {
	switch x := v.(type) {
	case []map[string]any:
		return x
	case []any:
		out := []map[string]any{}
		for _, v := range x {
			if r, ok := v.(map[string]any); ok {
				out = append(out, r)
			}
		}
		return out
	}
	return nil
}
func text(row map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := strings.TrimSpace(fmt.Sprint(row[k])); s != "" && s != "<nil>" {
			return s
		}
	}
	return ""
}
func firstText(row map[string]any, keys ...string) string { return text(row, keys...) }
func number(row map[string]any, keys ...string) (*float64, bool) {
	for _, k := range keys {
		switch v := row[k].(type) {
		case float64:
			x := v
			return &x, true
		case float32:
			x := float64(v)
			return &x, true
		case int:
			x := float64(v)
			return &x, true
		case int64:
			x := float64(v)
			return &x, true
		}
	}
	return nil, false
}
func firstMap(values ...map[string]any) map[string]any {
	for _, v := range values {
		if len(v) > 0 {
			return v
		}
	}
	return map[string]any{}
}
func compactMap(in map[string]any, keys ...string) map[string]any {
	out := map[string]any{}
	for _, k := range keys {
		if v, ok := in[k]; ok {
			out[k] = v
		}
	}
	return out
}
func stableID(prefix string, value any) string {
	data, _ := json.Marshal(value)
	sum := sha256.Sum256(data)
	return prefix + "_" + hex.EncodeToString(sum[:8])
}
func compact(v string, max int) string {
	v = strings.TrimSpace(v)
	r := []rune(v)
	if len(r) > max {
		return string(r[:max])
	}
	return v
}
func unique(values []string) []string {
	set := map[string]bool{}
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			set[v] = true
		}
	}
	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
func round3(v float64) float64 { return math.Round(v*1000) / 1000 }
