package lowendrelation

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
)

// BuildModel assembles the B4 low-end relation analysis from CCB evidence.
// It is deliberately read-only: no project mutation state is modified.
func BuildModel(in Input) Model {
	generatedAt := in.GeneratedAt.UTC()
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	mom := firstMap(
		in.MOMProjection,
		mapValue(in.MixObservation["mom_projection"]),
		mapValue(mapValue(in.MixObservation["observation"])["mom_projection"]),
	)
	relation := mapValue(mom["multitrack_relation"])
	profile := mapValue(mom["project_mix_profile"])

	tracks, _ := extractLowEndTracks(relation)
	conflicts := extractLowEndConflicts(relation)
	enrichTracksWithConflicts(tracks, conflicts)
	enrichTracksWithProjectState(tracks, in.ProjectState, in.ContextSnapshot)
	enrichTracksWithTOM(tracks, in.TOMProjection)

	summary := buildLowEndSummary(tracks, conflicts, profile)
	coverage := measureLowEndCoverage(tracks, conflicts, summary)
	refs := buildEvidenceRefs(mom, in.MixObservation)
	readiness := assessReadiness(coverage, refs)
	observations := buildObservations(tracks, conflicts, summary, coverage)
	limitations := buildLimitations(readiness, coverage)

	momSummary := compactMap(mom, "mom_version", "intent", "observation_id",
		"multitrack_relation", "project_mix_profile", "trust_quality")
	observationID := firstText(in.MixObservation, "observation_id")
	if observationID == "" {
		observationID = firstText(mom, "observation_id")
	}

	model := Model{
		SchemaVersion: ModelSchemaVersion,
		CapabilityID:  CapabilityID,
		GeneratedAt:   generatedAt.Format(time.RFC3339),
		UserIntent:    compact(in.UserIntent, 256),
		ObservationID: observationID,
		Tracks:        tracks,
		Conflicts:     conflicts,
		Summary:       summary,
		Coverage:      coverage,
		Readiness:     readiness,
		Observations:  observations,
		EvidenceRefs:  unique(refs),
		Limitations:   unique(limitations),
		MOMSummary:    momSummary,
	}
	model.ModelID = stableID("lerm", map[string]any{
		"tracks":         tracks,
		"observation_id": observationID,
	})
	return model
}

// Analyze converts a built Model into the Result summary for the Context Pack.
func Analyze(m Model) Result {
	return Result{
		SchemaVersion:      ResultSchemaVersion,
		ResultID:           stableID("lerr", m.ModelID),
		ModelID:            m.ModelID,
		CapabilityID:       m.CapabilityID,
		GeneratedAt:        m.GeneratedAt,
		ObservationID:      m.ObservationID,
		Readiness:          m.Readiness,
		Observations:       append([]Observation(nil), m.Observations...),
		Conflicts:          append([]LowEndConflict(nil), m.Conflicts...),
		Summary:            m.Summary,
		AnalyzedTrackCount: len(m.Tracks),
		EvidenceRefs:       append([]string(nil), m.EvidenceRefs...),
		Limitations:        append([]string(nil), m.Limitations...),
	}
}

func extractLowEndTracks(relation map[string]any) ([]LowEndTrack, map[string]*LowEndTrack) {
	occupancy := rowsValue(relation["band_occupancy"])
	byID := map[string]*LowEndTrack{}
	for _, row := range occupancy {
		band := text(row, "band")
		if band != "sub" && band != "bass" {
			continue
		}
		leaders := rowsValue(row["decision_tracks"])
		if len(leaders) == 0 {
			leaders = rowsValue(row["leaders"])
		}
		for rank, leader := range leaders {
			id := text(leader, "track_id")
			if id == "" {
				continue
			}
			t := byID[id]
			if t == nil {
				t = &LowEndTrack{TrackID: id}
				byID[id] = t
			}
			if t.TrackName == "" {
				t.TrackName = firstText(leader, "name", "track_name", "user_label")
			}
			if t.Role == "" {
				if role := text(leader, "role_guess"); role != "" && role != "unknown" {
					t.Role = role
					t.RoleSource = "mom_role_guess"
					t.RoleConfidence = 0.65
				}
			}
			uEnergy, hasU := numericField(leader, "unit_energy")
			eDB, hasDB := numericField(leader, "energy_db")
			switch band {
			case "sub":
				t.SubRank = rank + 1
				if hasU {
					t.SubUnitEnergy = ptr(uEnergy)
				}
				if hasDB {
					t.SubEnergyDB = ptr(eDB)
				}
			case "bass":
				t.BassRank = rank + 1
				if hasU {
					t.BassUnitEnergy = ptr(uEnergy)
				}
				if hasDB {
					t.BassEnergyDB = ptr(eDB)
				}
			}
		}
	}
	out := make([]LowEndTrack, 0, len(byID))
	for _, t := range byID {
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool {
		si := lowEndScore(out[i])
		sj := lowEndScore(out[j])
		if si != sj {
			return si > sj
		}
		return out[i].TrackID < out[j].TrackID
	})
	ptrMap := map[string]*LowEndTrack{}
	for i := range out {
		ptrMap[out[i].TrackID] = &out[i]
	}
	return out, ptrMap
}

func extractLowEndConflicts(relation map[string]any) []LowEndConflict {
	candidates := rowsValue(relation["band_conflict_candidates"])
	out := []LowEndConflict{}
	for _, c := range candidates {
		band := text(c, "band")
		if band != "sub" && band != "bass" {
			continue
		}
		// Conflict membership is deliberately narrower than the complete
		// decision-track roster. Every evidence-qualified low-end track remains
		// available to the LLM through Model.Tracks, but observing a track must
		// not automatically assert that it belongs to this concrete conflict.
		tracks := rowsValue(c["tracks"])
		trackIDs := make([]string, 0, len(tracks))
		for _, track := range tracks {
			if id := text(track, "track_id"); id != "" {
				trackIDs = append(trackIDs, id)
			}
		}
		sort.Strings(trackIDs)
		out = append(out, LowEndConflict{
			ID:         stableID("low_end_conflict", map[string]any{"band": band, "track_ids": trackIDs}),
			Band:       band,
			Tracks:     tracks,
			Confidence: text(c, "confidence"),
			Reason:     text(c, "reason"),
		})
	}
	return out
}

func enrichTracksWithConflicts(tracks []LowEndTrack, conflicts []LowEndConflict) {
	inConflict := map[string][]string{}
	for _, c := range conflicts {
		for _, t := range c.Tracks {
			if id := text(t, "track_id"); id != "" {
				inConflict[id] = append(inConflict[id], c.Band)
			}
		}
	}
	for i := range tracks {
		if bands, ok := inConflict[tracks[i].TrackID]; ok {
			tracks[i].ConflictBands = unique(bands)
		}
	}
}

func enrichTracksWithProjectState(tracks []LowEndTrack, projectState, contextSnapshot map[string]any) {
	stateRows := bestTrackRows(projectState, contextSnapshot)
	byID := map[string]map[string]any{}
	for _, row := range stateRows {
		if id := firstText(row, "track_id", "id"); id != "" {
			byID[id] = row
		}
	}
	for i := range tracks {
		row := byID[tracks[i].TrackID]
		if row == nil {
			continue
		}
		if v, ok := numericField(row, "volume_db", "fader_db", "track_gain_db"); ok {
			tracks[i].FaderDB = ptr(v)
		}
	}
}

func enrichTracksWithTOM(tracks []LowEndTrack, tomProjection map[string]any) {
	if len(tomProjection) == 0 {
		return
	}
	assignments := rowsValue(mapValue(tomProjection["group_proposals"])["assignment_excerpt"])
	if len(assignments) == 0 {
		assignments = rowsValue(mapValue(mapValue(tomProjection["full_assignment_manifest"])["groups"])["assignments"])
	}
	byID := map[string]map[string]any{}
	for _, row := range assignments {
		if id := firstText(row, "track_id", "id"); id != "" {
			byID[id] = row
		}
	}
	for i := range tracks {
		row := byID[tracks[i].TrackID]
		if row == nil {
			continue
		}
		role := firstText(row, "role_guess", "role_hypothesis", "role")
		if role != "" && role != "unknown" {
			tracks[i].Role = role
			tracks[i].RoleSource = "tom_assignment"
			tracks[i].RoleConfidence = 0.80
		}
	}
}

func buildLowEndSummary(tracks []LowEndTrack, conflicts []LowEndConflict, profile map[string]any) LowEndSummary {
	tendency := "unknown_or_unavailable"
	if bt := mapValue(mapValue(profile["band_tendency"])["low_end"]); len(bt) > 0 {
		if s := text(bt, "state"); s != "" {
			tendency = s
		}
	}
	subCount, bassCount := 0, 0
	domSubID, domBassID := "", ""
	for _, t := range tracks {
		if t.SubRank == 1 {
			domSubID = t.TrackID
		}
		if t.BassRank == 1 {
			domBassID = t.TrackID
		}
		if t.SubRank > 0 {
			subCount++
		}
		if t.BassRank > 0 {
			bassCount++
		}
	}
	return LowEndSummary{
		LowEndTendency:      tendency,
		DominantSubTrackID:  domSubID,
		DominantBassTrackID: domBassID,
		ConflictCount:       len(conflicts),
		SubBandTrackCount:   subCount,
		BassBandTrackCount:  bassCount,
	}
}

func measureLowEndCoverage(tracks []LowEndTrack, conflicts []LowEndConflict, summary LowEndSummary) Coverage {
	roleKnown := 0
	for _, t := range tracks {
		if t.Role != "" && t.Role != "unknown" {
			roleKnown++
		}
	}
	total := len(tracks)
	cov := 1.0
	if total == 0 {
		cov = 0.0
	}
	return Coverage{
		AnalyzedTrackCount:      total,
		BandEvidenceKnownCount:  total,
		SubBandLeaderCount:      summary.SubBandTrackCount,
		BassBandLeaderCount:     summary.BassBandTrackCount,
		ConflictCandidateCount:  len(conflicts),
		LowEndTendencyAvailable: summary.LowEndTendency != "unknown_or_unavailable",
		RoleKnownCount:          roleKnown,
		BandEvidenceCoverage:    cov,
	}
}

func assessReadiness(c Coverage, refs []string) capabilityruntime.Readiness {
	hasBandData := c.SubBandLeaderCount > 0 || c.BassBandLeaderCount > 0
	conditions := []capabilityruntime.Condition{
		{
			ID: "mom_low_end_band_occupancy", Required: true,
			Status:     condition_status(hasBandData),
			KnownCount: c.SubBandLeaderCount + c.BassBandLeaderCount,
			TotalCount: c.AnalyzedTrackCount,
			Summary: fmt.Sprintf("MOM sub/bass band occupancy: %d sub leaders, %d bass leaders",
				c.SubBandLeaderCount, c.BassBandLeaderCount),
			EvidenceRefs: refs,
			Remediation:  []string{"Run mix.observe with full-project multitrack intent."},
		},
		{
			ID: "low_end_tendency_context", Required: false,
			Status:       warningStatus(c.LowEndTendencyAvailable),
			Summary:      "MOM low_end tendency (sub+bass state) for overall landscape context",
			EvidenceRefs: refs,
		},
		{
			ID: "track_role_context", Required: false,
			Status:       warningStatus(c.RoleKnownCount > 0),
			KnownCount:   c.RoleKnownCount,
			TotalCount:   c.AnalyzedTrackCount,
			Summary:      "role context for low-end tracks; missing roles reduce diagnostic specificity",
			EvidenceRefs: refs,
		},
	}
	return capabilityruntime.Evaluate(CapabilityID, conditions)
}

func condition_status(ready bool) string {
	if ready {
		return capabilityruntime.ConditionReady
	}
	return capabilityruntime.ConditionBlocked
}

func warningStatus(ready bool) string {
	if ready {
		return capabilityruntime.ConditionReady
	}
	return capabilityruntime.ConditionWarning
}

func buildObservations(tracks []LowEndTrack, conflicts []LowEndConflict, summary LowEndSummary, c Coverage) []Observation {
	var out []Observation
	seq := 0
	nextID := func(kind string) string {
		seq++
		return fmt.Sprintf("obs_%s_%d", kind, seq)
	}
	if c.SubBandLeaderCount == 0 && c.BassBandLeaderCount == 0 {
		out = append(out, Observation{
			ID: nextID("no_evidence"), Kind: "no_low_end_band_evidence",
			Summary: "no sub or bass band occupancy data available; low-end analysis cannot proceed without MOM band data",
			Risk:    "none",
		})
		return out
	}
	if summary.LowEndTendency == "prominent" {
		out = append(out, Observation{
			ID:   nextID("tendency"),
			Kind: "low_end_prominent",
			Summary: "sub and bass bands are both dominant across the project mix; " +
				"total low-end energy is prominent relative to other frequency regions",
			Risk:     "medium",
			Evidence: map[string]any{"low_end_tendency": summary.LowEndTendency},
		})
	}
	for _, conflict := range conflicts {
		refs := trackRefs(conflict.Tracks)
		out = append(out, Observation{
			ID:         nextID("conflict"),
			Kind:       conflict.Band + "_masking_conflict",
			Summary:    fmt.Sprintf("%s band: %d tracks have close relative energy — masking conflict candidate", conflict.Band, len(conflict.Tracks)),
			TrackRefs:  refs,
			Risk:       "medium",
			Confidence: conflict.Confidence,
			Evidence: map[string]any{
				"band":                    conflict.Band,
				"conflicting_track_count": len(conflict.Tracks),
			},
		})
	}
	if len(conflicts) == 0 && (c.SubBandLeaderCount > 0 || c.BassBandLeaderCount > 0) {
		out = append(out, Observation{
			ID:      nextID("clear"),
			Kind:    "no_low_end_conflict",
			Summary: "no masking conflict candidates detected in sub or bass bands",
			Risk:    "none",
		})
	}
	return out
}

func buildLimitations(readiness capabilityruntime.Readiness, c Coverage) []string {
	var out []string
	out = append(out, readiness.BlockedBy...)
	if !c.LowEndTendencyAvailable {
		out = append(out, "low_end_tendency_unavailable")
	}
	if c.RoleKnownCount == 0 && c.AnalyzedTrackCount > 0 {
		out = append(out, "track_role_context_missing")
	}
	return out
}

func buildEvidenceRefs(mom, mixObservation map[string]any) []string {
	refs := []string{}
	if obsID := firstText(mom, "observation_id"); obsID != "" {
		refs = append(refs, "MOM:"+obsID)
	} else if len(mom) > 0 {
		refs = append(refs, "mix.observe")
	}
	if obsID := firstText(mixObservation, "observation_id"); obsID != "" {
		refs = append(refs, "mix.observe:"+obsID)
	}
	return unique(refs)
}

// --- local utility helpers (same pattern as panlayout/model.go) ---

func lowEndScore(t LowEndTrack) float64 {
	score := 0.0
	if t.SubUnitEnergy != nil {
		score += *t.SubUnitEnergy * 2
	}
	if t.BassUnitEnergy != nil {
		score += *t.BassUnitEnergy
	}
	return score
}

func trackRefs(tracks []map[string]any) []string {
	out := []string{}
	for _, t := range tracks {
		if id := text(t, "track_id"); id != "" {
			out = append(out, id)
		}
	}
	return out
}

func bestTrackRows(sources ...map[string]any) []map[string]any {
	for _, source := range sources {
		for _, candidate := range []map[string]any{
			source,
			mapValue(source["daw_state_summary"]),
			mapValue(source["project_state"]),
		} {
			for _, key := range []string{"tracks", "visible_tracks"} {
				if rows := rowsValue(candidate[key]); len(rows) > 0 {
					return rows
				}
			}
		}
	}
	return nil
}

func numericField(row map[string]any, keys ...string) (float64, bool) {
	for _, k := range keys {
		switch v := row[k].(type) {
		case float64:
			return v, !math.IsNaN(v) && !math.IsInf(v, 0)
		case float32:
			return float64(v), true
		case int:
			return float64(v), true
		case int64:
			return float64(v), true
		}
	}
	return 0, false
}

func ptr(v float64) *float64   { return &v }
func round3(v float64) float64 { return math.Round(v*1000) / 1000 }

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
		for _, item := range x {
			if r, ok := item.(map[string]any); ok {
				out = append(out, r)
			}
		}
		return out
	}
	return nil
}

func text(row map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := row[k].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func firstText(row map[string]any, keys ...string) string { return text(row, keys...) }

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
