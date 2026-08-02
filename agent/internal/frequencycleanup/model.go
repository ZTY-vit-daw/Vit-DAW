package frequencycleanup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/capabilityruntime"
	"vit-daw-agent/internal/mom"
)

func BuildModel(in Input) Model {
	generatedAt := in.GeneratedAt.UTC()
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	relation := decodeFrequencyRelationship(in.MOMProjection)
	tracks := extractTrackProfiles(relation.TrackProfiles)
	candidates := extractDiagnosisCandidates(relation)
	coverage := frequencyCoverage(relation, tracks, candidates)
	readiness := assessReadiness(relation, coverage)
	observationID := firstText(in.MixObservation, "observation_id")
	if observationID == "" {
		observationID = firstText(in.MOMProjection, "observation_id")
	}
	refs := unique(append(append([]string(nil), relation.EvidenceRefs...), candidateEvidenceRefs(candidates)...))
	model := Model{
		SchemaVersion: ModelSchemaVersion, CapabilityID: CapabilityID,
		GeneratedAt: generatedAt.Format(time.RFC3339), UserIntent: compact(in.UserIntent, 512),
		ObservationID: observationID, ProjectCutRef: relation.ProjectCutRef,
		FrequencyRelationship: relation, Tracks: tracks, Candidates: candidates,
		Coverage: coverage, Readiness: readiness, EvidenceRefs: refs,
		Limitations: unique(append([]string(nil), relation.Limitations...)),
	}
	model.ModelID = stableID("fcm", map[string]any{
		"observation_id": observationID, "project_cut_ref": relation.ProjectCutRef,
		"track_ids": trackIDs(tracks), "tap_point": relation.TapPoint, "candidates": candidates,
	})
	return model
}

func Analyze(model Model) Result {
	return Result{
		SchemaVersion: ResultSchemaVersion, ResultID: stableID("fcr", model.ModelID),
		ModelID: model.ModelID, CapabilityID: model.CapabilityID, ObservationID: model.ObservationID,
		Coverage: model.Coverage, Readiness: model.Readiness,
		Candidates:   append([]DiagnosisCandidate(nil), model.Candidates...),
		EvidenceRefs: append([]string(nil), model.EvidenceRefs...), Limitations: append([]string(nil), model.Limitations...),
	}
}

func decodeFrequencyRelationship(projection map[string]any) mom.FrequencyRelationship {
	value, _ := projection["frequency_relationship"]
	data, _ := json.Marshal(value)
	var relation mom.FrequencyRelationship
	_ = json.Unmarshal(data, &relation)
	return relation
}

func extractTrackProfiles(rows []map[string]any) []TrackProfile {
	out := make([]TrackProfile, 0, len(rows))
	for _, row := range rows {
		trackID := text(row["track_id"])
		if trackID == "" {
			continue
		}
		role := ""
		if hypothesis := mapValue(row["role_hypothesis"]); len(hypothesis) > 0 {
			role = text(hypothesis["value"])
		}
		bands := map[string]map[string]any{}
		for name, value := range mapValue(row["bands"]) {
			if band := mapValue(value); len(band) > 0 {
				bands[name] = cloneMap(band)
			}
		}
		out = append(out, TrackProfile{
			TrackID: trackID, TrackName: firstNonEmpty(text(row["name"]), trackID), Role: role,
			Status: text(row["status"]), Freshness: text(row["freshness"]), TapPoint: text(row["tap_point"]),
			Bands: bands, EvidenceRef: text(row["evidence_ref"]), SilenceConfirmed: boolValue(row["silence_confirmed"]), SilenceReason: text(row["silence_reason"]),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].TrackID < out[j].TrackID })
	return out
}

func extractDiagnosisCandidates(relation mom.FrequencyRelationship) []DiagnosisCandidate {
	out := make([]DiagnosisCandidate, 0, len(relation.ConflictCandidates)+len(relation.TonalTendencies))
	for _, row := range relation.ConflictCandidates {
		ids := stringsValue(row["track_ids"])
		if len(ids) == 0 {
			for _, track := range rowsValue(row["tracks"]) {
				ids = append(ids, text(track["track_id"]))
			}
		}
		ids = unique(ids)
		region := firstNonEmpty(text(row["region"]), text(row["band"]), text(row["strongest_region"]))
		id := firstNonEmpty(text(row["conflict_id"]), stableID("fcc", map[string]any{"region": region, "track_ids": ids}))
		out = append(out, DiagnosisCandidate{
			ID: id, Kind: "frequency_overlap_candidate", TrackIDs: ids, Region: region,
			Confidence: text(row["confidence"]), Summary: firstNonEmpty(text(row["reason"]), "Observable frequency-energy overlap candidate"),
			Evidence: cloneMap(row), EvidenceRefs: relation.EvidenceRefs,
		})
	}
	for _, row := range relation.TonalTendencies {
		trackID := text(row["track_id"])
		region := firstNonEmpty(text(row["region"]), text(row["band"]))
		id := firstNonEmpty(text(row["tendency_id"]), stableID("fct", map[string]any{"track_id": trackID, "region": region, "row": row}))
		out = append(out, DiagnosisCandidate{
			ID: id, Kind: "relative_tonal_tendency", TrackIDs: unique([]string{trackID}), Region: region,
			Confidence: text(row["confidence"]), Summary: firstNonEmpty(text(row["summary"]), text(row["state"]), "Observable relative tonal tendency"),
			Evidence: cloneMap(row), EvidenceRefs: relation.EvidenceRefs,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func frequencyCoverage(relation mom.FrequencyRelationship, tracks []TrackProfile, candidates []DiagnosisCandidate) Coverage {
	projectCount := intValue(relation.Coverage["project_track_count"])
	if projectCount == 0 {
		projectCount = len(stringsValue(relation.Scope["track_ids"]))
	}
	eligible := intValue(relation.Coverage["eligible_track_count"])
	missing := intValue(relation.Coverage["missing_track_count"])
	ratio := number(relation.Coverage["eligible_track_ratio"])
	if ratio == 0 && projectCount > 0 {
		ratio = float64(eligible) / float64(projectCount)
	}
	conflicts, tendencies := 0, 0
	for _, candidate := range candidates {
		if candidate.Kind == "frequency_overlap_candidate" {
			conflicts++
		} else {
			tendencies++
		}
	}
	return Coverage{
		ProjectTrackCount: projectCount, ProfileCount: len(tracks), EligibleTrackCount: eligible,
		MissingTrackCount: missing, EligibleTrackRatio: ratio, ConflictCandidateCount: conflicts,
		TonalTendencyCount: tendencies, TapPoint: firstNonEmpty(relation.TapPoint, "unknown"),
		FullProjectCoverage:     projectCount > 0 && len(tracks) == projectCount && eligible == projectCount && missing == 0,
		SupportsStaticDiagnosis: boolValue(relation.Coverage["supports_static_diagnosis"]),
		SupportsPostFXCompare:   boolValue(relation.Coverage["supports_post_fx_compare"]),
	}
}

func assessReadiness(relation mom.FrequencyRelationship, coverage Coverage) Readiness {
	refs := append([]string(nil), relation.EvidenceRefs...)
	diagnosis := capabilityruntime.Evaluate(CapabilityID, []capabilityruntime.Condition{
		{ID: "mom_frequency_relationship_v1", Required: true, Status: status(relation.SchemaVersion == mom.FrequencyRelationshipSchema), EvidenceRefs: refs, Remediation: []string{"request project_frequency_relationship_observation"}},
		{ID: "full_project_track_roster", Required: true, Status: status(coverage.ProjectTrackCount > 0 && coverage.ProfileCount == coverage.ProjectTrackCount), KnownCount: coverage.ProfileCount, TotalCount: coverage.ProjectTrackCount, EvidenceRefs: refs},
		{ID: "static_frequency_evidence", Required: true, Status: status(coverage.SupportsStaticDiagnosis && coverage.EligibleTrackCount > 0), KnownCount: coverage.EligibleTrackCount, TotalCount: coverage.ProjectTrackCount, EvidenceRefs: refs, Remediation: []string{"complete project band analysis"}},
		{ID: "full_project_frequency_coverage", Required: true, Status: status(coverage.FullProjectCoverage), KnownCount: coverage.EligibleTrackCount, TotalCount: coverage.ProjectTrackCount, EvidenceRefs: refs, Remediation: []string{"complete or refresh the existing offline band summaries"}},
		{ID: "uniform_observation_tap", Required: true, Status: status(relation.TapPoint != "" && relation.TapPoint != "unknown" && relation.TapPoint != "mixed"), EvidenceRefs: refs},
	})
	planning := capabilityruntime.Evaluate(CapabilityID, []capabilityruntime.Condition{
		{ID: "diagnosis_ready", Required: true, Status: status(diagnosis.CanProceed), EvidenceRefs: refs},
		{ID: "ccb_frequency_summary", Required: true, Status: status(coverage.FullProjectCoverage && coverage.ProfileCount == coverage.ProjectTrackCount), KnownCount: coverage.ProfileCount, TotalCount: coverage.ProjectTrackCount, EvidenceRefs: refs},
	})
	mutation := capabilityruntime.Evaluate(CapabilityID, []capabilityruntime.Condition{
		{ID: "planning_ready", Required: true, Status: status(planning.CanProceed), EvidenceRefs: refs},
		{ID: "target_scope_selected", Required: true, Status: capabilityruntime.ConditionMissing, EvidenceRefs: refs, Remediation: []string{"classify the C1 CCB and select exact static-EQ targets first"}},
		{ID: "target_post_fx_same_tap_baseline", Required: true, Status: capabilityruntime.ConditionMissing, EvidenceRefs: refs, Remediation: []string{"collect or reuse current-revision track_post_fader evidence for selected targets and relationship peers"}},
		{ID: "frequency_persistence", Required: false, Status: warningStatus(text(relation.PersistenceSummary["status"])), EvidenceRefs: refs},
	})
	return Readiness{Diagnosis: diagnosis, Planning: planning, Mutation: mutation}
}

func status(ok bool) string {
	if ok {
		return capabilityruntime.ConditionReady
	}
	return capabilityruntime.ConditionMissing
}
func warningStatus(value string) string {
	if value == mom.StatusReady {
		return capabilityruntime.ConditionReady
	}
	return capabilityruntime.ConditionWarning
}

func trackIDs(tracks []TrackProfile) []string {
	out := make([]string, 0, len(tracks))
	for _, t := range tracks {
		out = append(out, t.TrackID)
	}
	return out
}
func candidateEvidenceRefs(candidates []DiagnosisCandidate) []string {
	var out []string
	for _, c := range candidates {
		out = append(out, c.EvidenceRefs...)
	}
	return out
}
func stableID(prefix string, value any) string {
	data, _ := json.Marshal(value)
	sum := sha256.Sum256(data)
	return prefix + "_" + hex.EncodeToString(sum[:])[:16]
}
func compact(value string, limit int) string {
	value = strings.TrimSpace(value)
	r := []rune(value)
	if limit > 0 && len(r) > limit {
		return string(r[:limit]) + "..."
	}
	return value
}
func text(value any) string {
	if value == nil {
		return ""
	}
	s := strings.TrimSpace(fmt.Sprint(value))
	if s == "<nil>" {
		return ""
	}
	return s
}
func firstText(row map[string]any, keys ...string) string {
	for _, k := range keys {
		if v := text(row[k]); v != "" {
			return v
		}
	}
	return ""
}
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
func mapValue(value any) map[string]any {
	if row, ok := value.(map[string]any); ok {
		return row
	}
	data, _ := json.Marshal(value)
	var row map[string]any
	_ = json.Unmarshal(data, &row)
	return row
}
func rowsValue(value any) []map[string]any {
	data, _ := json.Marshal(value)
	var rows []map[string]any
	_ = json.Unmarshal(data, &rows)
	return rows
}
func cloneMap(value map[string]any) map[string]any {
	data, _ := json.Marshal(value)
	var out map[string]any
	_ = json.Unmarshal(data, &out)
	return out
}
func stringsValue(value any) []string {
	data, _ := json.Marshal(value)
	var out []string
	if json.Unmarshal(data, &out) == nil {
		return out
	}
	return nil
}
func boolValue(value any) bool { b, _ := value.(bool); return b }
func number(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		n, _ := v.Float64()
		return n
	}
	return 0
}
func intValue(value any) int { return int(number(value)) }
func unique(values []string) []string {
	seen := map[string]bool{}
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			seen[v] = true
		}
	}
	out := make([]string, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
