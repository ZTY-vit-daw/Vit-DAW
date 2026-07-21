package capabilitycontext

import "math"

const GainStagingSuggestionSchemaVersion = "static_mix.gain_staging.suggestion.v0"

type GainStagingSuggestionSet struct {
	SchemaVersion string                  `json:"schema_version"`
	CapabilityID  string                  `json:"capability_id"`
	PackID        string                  `json:"pack_id,omitempty"`
	Status        string                  `json:"status"`
	Summary       map[string]any          `json:"summary,omitempty"`
	PrimaryAction *GainStagingSuggestion  `json:"primary_action,omitempty"`
	Actions       []GainStagingSuggestion `json:"actions,omitempty"`
	Limitations   []string                `json:"limitations,omitempty"`
}

type GainStagingSuggestion struct {
	ActionKind           string         `json:"action_kind"`
	Tool                 string         `json:"tool"`
	Operation            string         `json:"operation,omitempty"`
	TrackIDs             []string       `json:"track_ids,omitempty"`
	TrackID              string         `json:"track_id,omitempty"`
	TrackName            string         `json:"track_name,omitempty"`
	ClipID               string         `json:"clip_id,omitempty"`
	ClipName             string         `json:"clip_name,omitempty"`
	CurrentDB            *float64       `json:"current_db,omitempty"`
	TargetDB             *float64       `json:"target_db,omitempty"`
	DeltaDB              *float64       `json:"delta_db,omitempty"`
	Risk                 string         `json:"risk,omitempty"`
	Reason               string         `json:"reason,omitempty"`
	EvidenceRefs         []string       `json:"evidence_refs,omitempty"`
	RequiresConfirmation bool           `json:"requires_confirmation"`
	Metadata             map[string]any `json:"metadata,omitempty"`
}

func BuildGainStagingSuggestions(pack Pack) GainStagingSuggestionSet {
	out := GainStagingSuggestionSet{
		SchemaVersion: GainStagingSuggestionSchemaVersion,
		CapabilityID:  GainStagingCapabilityID,
		PackID:        pack.PackID,
		Status:        "no_action",
		Summary: map[string]any{
			"action_count": 0,
		},
	}
	if pack.CapabilityID != "" && pack.CapabilityID != GainStagingCapabilityID {
		out.Status = "blocked"
		out.Limitations = append(out.Limitations, "pack capability is not B1 gain staging")
		return out
	}
	var actions []GainStagingSuggestion
	if action, ok := gainStagingFaderUnitySuggestion(pack); ok {
		actions = append(actions, action)
	}
	if batch, batchActions, ok := gainStagingSourceLevelCalibrationBatchSuggestion(pack); ok {
		actions = append(actions, batch)
		actions = append(actions, batchActions...)
	}
	if action, ok := gainStagingClipGainSuggestion(pack); ok {
		actions = append(actions, action)
	}
	if len(actions) == 0 {
		out.Limitations = append(out.Limitations, pack.Limitations...)
		if len(pack.Rankings["headroom_risk"]) > 0 {
			out.Limitations = append(out.Limitations, "headroom_risk_requires_source_or_clip_trim_calibration")
		}
		if len(pack.Rankings["low_level"]) > 0 {
			out.Limitations = append(out.Limitations, "low_level_risk_requires_source_or_clip_trim_calibration")
		}
		return out
	}
	out.Status = "suggested"
	out.Actions = actions
	out.PrimaryAction = &out.Actions[0]
	out.Summary["action_count"] = len(actions)
	out.Summary["primary_action_kind"] = out.PrimaryAction.ActionKind
	out.Summary["primary_tool"] = out.PrimaryAction.Tool
	return out
}

func gainStagingFaderUnitySuggestion(pack Pack) (GainStagingSuggestion, bool) {
	trackIDs := []string{}
	seen := map[string]bool{}
	var first RankRow
	for _, row := range pack.Rankings["track_gain_outliers"] {
		if row.TrackID == "" || math.Abs(row.Value) < 6 {
			continue
		}
		if len(trackIDs) == 0 {
			first = row
		}
		if seen[row.TrackID] {
			continue
		}
		seen[row.TrackID] = true
		trackIDs = append(trackIDs, row.TrackID)
	}
	if len(trackIDs) == 0 {
		return GainStagingSuggestion{}, false
	}
	target := 0.0
	current := round3(first.Value)
	delta := round3(target - current)
	return GainStagingSuggestion{
		ActionKind:           "track_fader_unity_reset",
		Tool:                 "track.group.apply_control",
		Operation:            "track_fader_absolute_reset",
		TrackIDs:             trackIDs,
		TrackID:              first.TrackID,
		TrackName:            first.TrackName,
		CurrentDB:            &current,
		TargetDB:             &target,
		DeltaDB:              &delta,
		Risk:                 first.Risk,
		Reason:               "large track fader offset; reset track faders to unity before B2 static balance",
		EvidenceRefs:         append([]string(nil), first.EvidenceRefs...),
		RequiresConfirmation: true,
		Metadata: map[string]any{
			"ranking_kind":  first.Kind,
			"ranking_unit":  first.Unit,
			"ranking_value": first.Value,
			"target_count":  len(trackIDs),
		},
	}, true
}

func gainStagingSourceLevelCalibrationBatchSuggestion(pack Pack) (GainStagingSuggestion, []GainStagingSuggestion, bool) {
	rows := gainStagingSourceLevelCalibrationRows(pack)
	if len(rows) == 0 {
		return GainStagingSuggestion{}, nil, false
	}
	actions := make([]GainStagingSuggestion, 0, len(rows))
	for _, row := range rows {
		action, ok := gainStagingSourceLevelCalibrationSuggestionFromRow(row)
		if !ok {
			continue
		}
		actions = append(actions, action)
	}
	if len(actions) == 0 {
		return GainStagingSuggestion{}, nil, false
	}
	if len(actions) == 1 {
		return actions[0], nil, true
	}
	minTarget := 0.0
	maxTarget := 0.0
	maxAbsDelta := 0.0
	trackIDs := make([]string, 0, len(actions))
	for i, action := range actions {
		if action.TrackID != "" {
			trackIDs = append(trackIDs, action.TrackID)
		}
		if action.TargetDB != nil {
			if i == 0 || *action.TargetDB < minTarget {
				minTarget = *action.TargetDB
			}
			if i == 0 || *action.TargetDB > maxTarget {
				maxTarget = *action.TargetDB
			}
		}
		if action.DeltaDB != nil && math.Abs(*action.DeltaDB) > maxAbsDelta {
			maxAbsDelta = math.Abs(*action.DeltaDB)
		}
	}
	metadata := map[string]any{
		"batch_action_count": len(actions),
		"target_count":       len(actions),
		"track_count":        len(trackIDs),
		"min_target_db":      round3(minTarget),
		"max_target_db":      round3(maxTarget),
		"max_abs_delta_db":   round3(maxAbsDelta),
	}
	if len(actions[0].Metadata) > 0 {
		for _, key := range []string{"reference_metric", "reference_level", "reference_unit", "eligible_track_count", "covered_track_count", "candidate_count", "calibration_ready_count", "comparison_status", "calibration_mode", "rlm_projection_id", "strict_reference_model", "metric_approximate", "metric_last_resort"} {
			if value, ok := actions[0].Metadata[key]; ok {
				metadata[key] = value
			}
		}
	}
	target := round3(maxTarget)
	delta := round3(maxAbsDelta)
	batch := GainStagingSuggestion{
		ActionKind:           "source_clip_gain_calibration_batch",
		Tool:                 "clip.gain.set_batch",
		Operation:            "source_level_clip_gain_set_batch",
		TrackIDs:             trackIDs,
		TargetDB:             &target,
		DeltaDB:              &delta,
		Risk:                 highestRisk(actions),
		Reason:               "B1.2 full-project source levels differ from the same technical reference; calibrate primary clip gains as one confirmed batch",
		RequiresConfirmation: true,
		Metadata:             metadata,
	}
	return batch, actions, true
}

func gainStagingSourceLevelCalibrationRows(pack Pack) []RankRow {
	if rows := gainStagingSourceLevelCalibrationRowsFromKey(pack, "source_level_reference_calibration"); len(rows) > 0 {
		return rows
	}
	return gainStagingSourceLevelCalibrationRowsFromKey(pack, "source_level_outliers")
}

func gainStagingSourceLevelCalibrationRowsFromKey(pack Pack, key string) []RankRow {
	out := make([]RankRow, 0, len(pack.Rankings["source_level_outliers"]))
	for _, row := range pack.Rankings[key] {
		if row.ClipID == "" {
			continue
		}
		if _, ok := numberValue(row.Metadata["target_clip_gain_db"]); !ok {
			continue
		}
		out = append(out, row)
	}
	return out
}

func gainStagingSourceLevelCalibrationSuggestionFromRow(row RankRow) (GainStagingSuggestion, bool) {
	target, ok := numberValue(row.Metadata["target_clip_gain_db"])
	if !ok {
		return GainStagingSuggestion{}, false
	}
	current := 0.0
	if value, ok := numberValue(row.Metadata["current_clip_gain_db"]); ok {
		current = value
	}
	target = round3(target)
	current = round3(current)
	delta := round3(target - current)
	metadata := cloneMap(row.Metadata)
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["ranking_kind"] = row.Kind
	metadata["ranking_unit"] = row.Unit
	return GainStagingSuggestion{
		ActionKind:           "source_clip_gain_calibration",
		Tool:                 "clip.gain.set",
		Operation:            "source_level_clip_gain_set",
		TrackID:              row.TrackID,
		TrackName:            row.TrackName,
		ClipID:               row.ClipID,
		ClipName:             row.ClipName,
		CurrentDB:            &current,
		TargetDB:             &target,
		DeltaDB:              &delta,
		Risk:                 row.Risk,
		Reason:               "B1.2 source level differs from project technical reference; calibrate the primary clip gain, not the track fader",
		EvidenceRefs:         append([]string(nil), row.EvidenceRefs...),
		RequiresConfirmation: true,
		Metadata:             metadata,
	}, true
}

func highestRisk(actions []GainStagingSuggestion) string {
	score := map[string]int{"": 0, "low": 1, "medium": 2, "high": 3}
	best := ""
	for _, action := range actions {
		if score[action.Risk] > score[best] {
			best = action.Risk
		}
	}
	return best
}

func gainStagingClipGainSuggestion(pack Pack) (GainStagingSuggestion, bool) {
	for _, row := range pack.Rankings["clip_gain_outliers"] {
		if row.ClipID == "" || math.Abs(row.Value) < 6 {
			continue
		}
		target := 0.0
		current := round3(row.Value)
		delta := round3(target - current)
		return GainStagingSuggestion{
			ActionKind:           "clip_gain_set",
			Tool:                 "clip.gain.set",
			TrackID:              row.TrackID,
			TrackName:            row.TrackName,
			ClipID:               row.ClipID,
			ClipName:             row.ClipName,
			CurrentDB:            &current,
			TargetDB:             &target,
			DeltaDB:              &delta,
			Risk:                 row.Risk,
			Reason:               "large static clip gain offset",
			EvidenceRefs:         append([]string(nil), row.EvidenceRefs...),
			RequiresConfirmation: true,
			Metadata: map[string]any{
				"ranking_kind": row.Kind,
				"ranking_unit": row.Unit,
			},
		}, true
	}
	return GainStagingSuggestion{}, false
}
