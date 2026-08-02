package mom

import (
	"sort"
	"strings"
)

func buildStaticLevelRelationship(input Input) *StaticLevelRelationship {
	projected := mapValue(input.ProjectPackage["static_level_relationship_inputs"])
	if len(projected) == 0 {
		return nil
	}
	rows := rowsFromAny(projected["tracks"])
	tracks := make([]StaticLevelTrack, 0, len(rows))
	ready, usable := 0, 0
	refs := evidenceRefs(observationRef(input), "project_package.static_level_relationship_inputs")
	limitations := stringsFromAny(projected["limitations"])
	for _, row := range rows {
		trackID := text(row["track_id"])
		if trackID == "" {
			continue
		}
		status := StatusFromSource(text(row["status"]))
		track := StaticLevelTrack{
			TrackID:           trackID,
			Status:            status,
			Freshness:         FreshnessForStatus(status),
			Metric:            firstNonEmpty(text(row["metric"]), "effective_static_rms_dbfs"),
			TapPoint:          firstNonEmpty(text(row["tap_point"]), "derived_static_control_model"),
			IncludedClipIDs:   stringsFromAny(row["included_clip_ids"]),
			MissingClipIDs:    stringsFromAny(row["missing_clip_ids"]),
			AggregationMethod: firstNonEmpty(text(row["aggregation_method"]), "duration_weighted_linear_energy"),
			EvidenceRefs:      evidenceRefs(stringsFromAny(row["evidence_refs"])...),
			Limitations:       stringsFromAny(row["limitations"]),
		}
		if value, ok := optionalNumber(row["source_rms_dbfs"]); ok {
			track.SourceRMSDBFS = &value
		}
		if value, ok := optionalNumber(row["effective_static_rms_dbfs"]); ok {
			track.EffectiveStaticRMSDBFS = &value
			if staticLevelRelationshipUsable(status) {
				usable++
			}
		}
		if value, ok := optionalNumber(row["effective_static_peak_dbfs"]); ok {
			track.EffectiveStaticPeakDBFS = &value
		}
		if value, ok := optionalNumber(row["fader_db"]); ok {
			track.FaderDB = &value
		}
		if status == StatusReady {
			ready++
		}
		refs = evidenceRefs(append(refs, track.EvidenceRefs...)...)
		tracks = append(tracks, track)
	}
	sort.SliceStable(tracks, func(i, j int) bool { return tracks[i].TrackID < tracks[j].TrackID })
	status := StatusFromSource(text(projected["status"]))
	if len(tracks) == 0 {
		status = StatusMissing
	}
	return &StaticLevelRelationship{
		SchemaVersion: firstNonEmpty(text(projected["schema_version"]), StaticLevelRelationshipSchema),
		Status:        status,
		Freshness:     FreshnessForStatus(status),
		ProjectCutRef: firstNonEmpty(text(projected["project_cut_ref"]), text(input.ProjectPackage["project_state_hash"]), text(input.ProjectPackage["project_revision"])),
		Coverage: map[string]any{
			"track_count":        len(tracks),
			"usable_track_count": usable,
			"ready_track_count":  ready,
		},
		Tracks:       tracks,
		EvidenceRefs: refs,
		Limitations:  limitations,
	}
}

func optionalNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case jsonNumber:
		parsed, err := typed.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

// jsonNumber is the small interface implemented by encoding/json.Number. It
// keeps this projection parser independent from the concrete decoder used by
// the caller.
type jsonNumber interface {
	Float64() (float64, error)
}

func staticLevelRelationshipValue(proj Projection) StaticLevelRelationship {
	if proj.StaticLevelRelationship == nil {
		return StaticLevelRelationship{}
	}
	return *proj.StaticLevelRelationship
}

func staticLevelRelationshipUsable(status string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	return status == StatusReady || status == StatusApprox || status == StatusPartial
}
