package lowendrelation

import (
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/capabilityruntime"
)

const TargetPostFXBaselineSchema = "low_end_relation.target_post_fx_baseline.v1"

type TargetProbeEvidence struct {
	TrackID               string         `json:"track_id"`
	Status                string         `json:"status"`
	TapPoint              string         `json:"tap_point"`
	RenderRevision        string         `json:"render_revision"`
	TrackStateFingerprint string         `json:"track_state_fingerprint"`
	Bands                 map[string]any `json:"bands,omitempty"`
	Metrics               map[string]any `json:"metrics,omitempty"`
	EvidenceRef           string         `json:"evidence_ref"`
}

type TargetPostFXBaseline struct {
	SchemaVersion     string                      `json:"schema_version"`
	BaselineID        string                      `json:"baseline_id"`
	SessionID         string                      `json:"session_id"`
	Phase             string                      `json:"phase"`
	ProjectID         string                      `json:"project_id"`
	TapPoint          string                      `json:"tap_point"`
	RequestedTrackIDs []string                    `json:"requested_track_ids"`
	Tracks            []TargetProbeEvidence       `json:"tracks"`
	Readiness         capabilityruntime.Readiness `json:"readiness"`
	EvidenceRefs      []string                    `json:"evidence_refs,omitempty"`
	Collection        TargetBaselineCollection    `json:"collection"`
	CreatedAt         string                      `json:"created_at"`
}

type TargetBaselineCollection struct {
	ElapsedMS        int64    `json:"elapsed_ms"`
	RenderedTrackIDs []string `json:"rendered_track_ids,omitempty"`
	CacheHitTrackIDs []string `json:"cache_hit_track_ids,omitempty"`
}

type TargetPostFXVerification struct {
	Status            string   `json:"status"`
	Comparable        bool     `json:"comparable"`
	BeforeBaselineID  string   `json:"before_baseline_id"`
	AfterBaselineID   string   `json:"after_baseline_id"`
	TapPoint          string   `json:"tap_point"`
	TrackIDs          []string `json:"track_ids"`
	ChangedDimensions []string `json:"changed_dimensions,omitempty"`
	Reasons           []string `json:"reasons,omitempty"`
	EvidenceRefs      []string `json:"evidence_refs,omitempty"`
	UserAcceptance    string   `json:"user_acceptance"`
}

// TargetPostFXTrackIDs limits B4 verification to exact treatment targets and
// the peers in the relationship conflicts those targets reference.
func TargetPostFXTrackIDs(plan TreatmentPlan, model Model) []string {
	selected := map[string]bool{}
	referenced := map[string]bool{}
	for _, target := range plan.Targets {
		if trackID := strings.TrimSpace(target.TrackID); trackID != "" {
			selected[trackID] = true
		}
		for _, ref := range target.RelationshipRefs {
			if ref = strings.TrimSpace(ref); ref != "" {
				referenced[ref] = true
			}
		}
	}
	for _, conflict := range model.Conflicts {
		if !referenced[conflict.ID] {
			continue
		}
		for _, peer := range conflict.Tracks {
			if trackID := text(peer, "track_id"); trackID != "" {
				selected[trackID] = true
			}
		}
	}
	ids := make([]string, 0, len(selected))
	for trackID := range selected {
		ids = append(ids, trackID)
	}
	sort.Strings(ids)
	return ids
}

func BuildTargetPostFXBaseline(sessionID, phase, projectID string, requestedTrackIDs []string, rows []map[string]any, createdAt time.Time) TargetPostFXBaseline {
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	requested := unique(requestedTrackIDs)
	byTrack := map[string]map[string]any{}
	for _, row := range rows {
		if trackID := text(row, "track_id"); trackID != "" {
			byTrack[trackID] = row
		}
	}
	tracks := make([]TargetProbeEvidence, 0, len(requested))
	refs := []string{}
	readyCount := 0
	uniformTap := ""
	uniformTapReady := true
	for _, trackID := range requested {
		row := byTrack[trackID]
		status := strings.ToLower(text(row, "status"))
		if quality := strings.ToLower(text(row, "quality_status")); quality == "suspect" || quality == "stale" || quality == "failed" {
			status = quality
		}
		tap := strings.ToLower(text(row, "tap_point"))
		renderRevision := text(row, "render_revision")
		fingerprint := text(row, "track_state_fingerprint")
		bands := copyBaselineMap(mapValue(row["bands"]))
		evidenceRef := text(row, "evidence_ref")
		valid := status == "ready" && tap == "track_post_fader" && renderRevision != "" && fingerprint != "" && len(bands) > 0 && evidenceRef != ""
		if valid {
			readyCount++
		}
		if tap == "" {
			uniformTapReady = false
		} else if uniformTap == "" {
			uniformTap = tap
		} else if uniformTap != tap {
			uniformTapReady = false
		}
		metrics := map[string]any{}
		for _, key := range []string{"rms_dbfs", "peak_dbfs", "headroom_db", "crest_db", "balance_db", "correlation_estimate"} {
			if value, ok := row[key]; ok {
				metrics[key] = value
			}
		}
		tracks = append(tracks, TargetProbeEvidence{
			TrackID: trackID, Status: firstNonEmptyString(status, "missing"), TapPoint: firstNonEmptyString(tap, "unknown"),
			RenderRevision: renderRevision, TrackStateFingerprint: fingerprint, Bands: bands, Metrics: metrics, EvidenceRef: evidenceRef,
		})
		if evidenceRef != "" {
			refs = append(refs, evidenceRef)
		}
	}
	ready := len(requested) > 0 && readyCount == len(requested)
	readiness := capabilityruntime.Evaluate(CapabilityID, []capabilityruntime.Condition{
		{ID: "target_scope_selected", Required: true, Status: baselineCondition(len(requested) > 0), KnownCount: len(requested), TotalCount: len(requested)},
		{ID: "exact_target_peer_coverage", Required: true, Status: baselineCondition(ready), KnownCount: readyCount, TotalCount: len(requested), EvidenceRefs: unique(refs)},
		{ID: "target_post_fx_same_tap_baseline", Required: true, Status: baselineCondition(ready && uniformTapReady && uniformTap == "track_post_fader"), EvidenceRefs: unique(refs)},
	})
	baseline := TargetPostFXBaseline{
		SchemaVersion: TargetPostFXBaselineSchema, SessionID: strings.TrimSpace(sessionID), Phase: strings.TrimSpace(phase),
		ProjectID: strings.TrimSpace(projectID), TapPoint: firstNonEmptyString(uniformTap, "unknown"), RequestedTrackIDs: requested,
		Tracks: tracks, Readiness: readiness, EvidenceRefs: unique(refs), CreatedAt: createdAt.UTC().Format(time.RFC3339Nano),
	}
	baseline.BaselineID = stableID("b4fx", map[string]any{"project_id": baseline.ProjectID, "tap_point": baseline.TapPoint, "tracks": baseline.Tracks})
	return baseline
}

func VerifyTargetPostFXBaselines(before, after TargetPostFXBaseline) TargetPostFXVerification {
	reasons := []string{}
	comparable := true
	if before.SchemaVersion != TargetPostFXBaselineSchema || after.SchemaVersion != TargetPostFXBaselineSchema {
		comparable = false
		reasons = append(reasons, "target_baseline_schema_mismatch")
	}
	if before.ProjectID == "" || before.ProjectID != after.ProjectID {
		comparable = false
		reasons = append(reasons, "project_identity_mismatch")
	}
	if before.TapPoint != "track_post_fader" || before.TapPoint != after.TapPoint {
		comparable = false
		reasons = append(reasons, "post_fx_tap_mismatch")
	}
	if stableID("tracks", before.RequestedTrackIDs) != stableID("tracks", after.RequestedTrackIDs) {
		comparable = false
		reasons = append(reasons, "target_scope_mismatch")
	}
	if !before.Readiness.CanProceed || !after.Readiness.CanProceed {
		comparable = false
		reasons = append(reasons, "target_baseline_not_ready")
	}
	if before.BaselineID == after.BaselineID {
		comparable = false
		reasons = append(reasons, "post_execution_baseline_not_fresh")
	}
	changed := []string{}
	if comparable {
		beforeRows := map[string]TargetProbeEvidence{}
		for _, row := range before.Tracks {
			beforeRows[row.TrackID] = row
		}
		for _, row := range after.Tracks {
			prior := beforeRows[row.TrackID]
			if stableID("bands", prior.Bands) != stableID("bands", row.Bands) {
				changed = append(changed, "track_frequency_bands:"+row.TrackID)
			}
			if stableID("metrics", prior.Metrics) != stableID("metrics", row.Metrics) {
				changed = append(changed, "track_post_fx_metrics:"+row.TrackID)
			}
		}
	}
	status := "inconclusive"
	if comparable && len(changed) > 0 {
		status = "observed_change"
	} else if comparable {
		reasons = append(reasons, "no_post_fx_change_observed")
	}
	return TargetPostFXVerification{
		Status: status, Comparable: comparable, BeforeBaselineID: before.BaselineID, AfterBaselineID: after.BaselineID,
		TapPoint: before.TapPoint, TrackIDs: append([]string(nil), before.RequestedTrackIDs...), ChangedDimensions: unique(changed),
		Reasons: unique(reasons), EvidenceRefs: unique(append(append([]string(nil), before.EvidenceRefs...), after.EvidenceRefs...)), UserAcceptance: "unknown",
	}
}

func baselineCondition(ok bool) string {
	if ok {
		return capabilityruntime.ConditionReady
	}
	return capabilityruntime.ConditionMissing
}

func copyBaselineMap(source map[string]any) map[string]any {
	if len(source) == 0 {
		return nil
	}
	out := make(map[string]any, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}
