package mom

import (
	"sort"
	"strings"
)

const maxMaskingCandidates = 24

func buildMaskingRelationship(input Input) MaskingRelationship {
	measurement := mapValue(input.ProjectPackage["masking_relationship_inputs"])
	status := StatusFromSource(text(measurement["status"]))
	if len(measurement) == 0 {
		status = StatusMissing
	}
	model := mapValue(measurement["model"])
	candidateOnly := boolValue(model["candidate_only"])
	limitations := append([]string{}, stringValues(measurement["limitations"])...)
	if !candidateOnly {
		limitations = append(limitations, "measurement_does_not_declare_candidate_only_semantics")
		if status == StatusReady {
			status = StatusSuspect
		}
	}
	limitations = append(limitations,
		"masking_candidates_are_directional_improvement_risks_not_deterministic_mix_defects",
		"no_processor_choice_parameter_or_mutation_authority",
	)

	candidates := make([]MaskingCandidate, 0)
	for _, row := range rowsFromAny(measurement["pair_bands"]) {
		if !strings.EqualFold(text(row["status"]), "candidate") {
			continue
		}
		candidate := MaskingCandidate{
			MaskerTrackID:   text(row["masker_track_id"]),
			MaskerTrackName: text(row["masker_track_name"]),
			TargetTrackID:   text(row["target_track_id"]),
			TargetTrackName: text(row["target_track_name"]),
			BandID:          text(row["band_id"]),
			MinHz:           number(row["min_hz"]),
			MaxHz:           number(row["max_hz"]),
			ActiveFrames:    int(number(row["active_frame_count"])),
			RiskFrames:      int(number(row["risk_frame_count"])),
			CoverageRatio:   round3mom(number(row["risk_coverage_ratio"])),
			MedianMarginDB:  round3mom(number(row["median_margin_db"])),
			P90MarginDB:     round3mom(number(row["p90_margin_db"])),
			MaxMarginDB:     round3mom(number(row["max_margin_db"])),
		}
		if candidate.MaskerTrackID == "" || candidate.TargetTrackID == "" || candidate.BandID == "" {
			continue
		}
		candidates = append(candidates, candidate)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].CoverageRatio != candidates[j].CoverageRatio {
			return candidates[i].CoverageRatio > candidates[j].CoverageRatio
		}
		if candidates[i].P90MarginDB != candidates[j].P90MarginDB {
			return candidates[i].P90MarginDB > candidates[j].P90MarginDB
		}
		return candidates[i].MaskerTrackID+candidates[i].TargetTrackID+candidates[i].BandID < candidates[j].MaskerTrackID+candidates[j].TargetTrackID+candidates[j].BandID
	})
	coverage := cloneMOMMap(mapValue(measurement["coverage"]))
	coverage["projected_candidate_count"] = len(candidates)
	if len(candidates) > maxMaskingCandidates {
		candidates = candidates[:maxMaskingCandidates]
		coverage["candidates_truncated"] = true
	}
	conditions := compactMap(mapValue(measurement["conditions"]),
		"tap_point", "range_start_seconds", "range_end_seconds", "tail_seconds", "sample_rate", "analyzer_revision", "synchronized", "frame_count_per_track")
	if !boolValue(conditions["synchronized"]) && status == StatusReady {
		status = StatusSuspect
		limitations = append(limitations, "measurement_does_not_confirm_synchronized_track_windows")
	}
	return MaskingRelationship{
		SchemaVersion:  MaskingRelationshipSchema,
		Status:         status,
		Freshness:      FreshnessForStatus(status),
		MeasurementID:  text(measurement["measurement_id"]),
		ModelVersion:   text(model["model_version"]),
		CandidateOnly:  candidateOnly,
		ProjectBinding: compactMap(mapValue(measurement["project_binding"]), "project_uuid", "project_revision", "project_state_hash"),
		Conditions:     conditions,
		Coverage:       coverage,
		Candidates:     candidates,
		EvidenceRefs:   evidenceRefs(append([]string{"mix.read:project.masking_relationship_inputs"}, stringValues(measurement["evidence_refs"])...)...),
		Limitations:    evidenceRefs(limitations...),
	}
}

func stringValues(value any) []string {
	rows, ok := value.([]string)
	if ok {
		return append([]string(nil), rows...)
	}
	values, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if item := text(value); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func cloneMOMMap(input map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range input {
		out[key] = value
	}
	return out
}
