package frequencycleanup

import (
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/capabilityruntime"
)

const TargetPostFXBaselineSchema = "frequency_cleanup.target_post_fx_baseline.v1"

type TargetProbeEvidence struct {
	TrackID               string         `json:"track_id"`
	Status                string         `json:"status"`
	TapPoint              string         `json:"tap_point"`
	RenderRevision        string         `json:"render_revision"`
	TrackStateFingerprint string         `json:"track_state_fingerprint"`
	SourceRevision        string         `json:"source_revision,omitempty"`
	ClipRevision          string         `json:"clip_revision,omitempty"`
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

// TargetPreflightTrackIDs returns the executable targets plus only the
// relationship peers that share an observable candidate with those targets.
func TargetPreflightTrackIDs(plan TreatmentPlan, model Model) []string {
	selected := map[string]bool{}
	for _, item := range StaticEQItems(plan) {
		selected[item.TrackID] = true
	}
	for _, candidate := range model.Candidates {
		related := false
		for _, trackID := range candidate.TrackIDs {
			if selected[trackID] {
				related = true
				break
			}
		}
		if related {
			for _, trackID := range candidate.TrackIDs {
				selected[trackID] = true
			}
		}
	}
	ids := make([]string, 0, len(selected))
	for trackID := range selected {
		if strings.TrimSpace(trackID) != "" {
			ids = append(ids, strings.TrimSpace(trackID))
		}
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
		trackID := text(row["track_id"])
		if trackID == "" {
			continue
		}
		byTrack[trackID] = row
	}
	tracks := make([]TargetProbeEvidence, 0, len(requested))
	refs := []string{}
	readyCount := 0
	uniformTap := ""
	uniform := true
	currentRevisionCount := 0
	for _, trackID := range requested {
		row := byTrack[trackID]
		status := strings.ToLower(text(row["status"]))
		if quality := strings.ToLower(text(row["quality_status"])); quality == "suspect" || quality == "stale" || quality == "failed" {
			status = quality
		}
		tap := strings.ToLower(text(row["tap_point"]))
		renderRevision := text(row["render_revision"])
		fingerprint := text(row["track_state_fingerprint"])
		bands := cloneMap(mapValue(row["bands"]))
		evidenceRef := text(row["evidence_ref"])
		valid := status == "ready" && tap == "track_post_fader" && renderRevision != "" && fingerprint != "" && len(bands) > 0 && evidenceRef != ""
		if valid {
			readyCount++
			currentRevisionCount++
		}
		if tap != "" {
			if uniformTap == "" {
				uniformTap = tap
			} else if uniformTap != tap {
				uniform = false
			}
		} else {
			uniform = false
		}
		metrics := compactTargetMetrics(row)
		tracks = append(tracks, TargetProbeEvidence{
			TrackID: trackID, Status: firstNonEmpty(status, "missing"), TapPoint: firstNonEmpty(tap, "unknown"),
			RenderRevision: renderRevision, TrackStateFingerprint: fingerprint,
			SourceRevision: text(row["source_revision"]), ClipRevision: text(row["clip_revision"]),
			Bands: bands, Metrics: metrics, EvidenceRef: evidenceRef,
		})
		if evidenceRef != "" {
			refs = append(refs, evidenceRef)
		}
	}
	readiness := capabilityruntime.Evaluate(CapabilityID, []capabilityruntime.Condition{
		{ID: "target_scope_selected", Required: true, Status: status(len(requested) > 0), KnownCount: len(requested), TotalCount: len(requested)},
		{ID: "exact_target_coverage", Required: true, Status: status(len(requested) > 0 && readyCount == len(requested)), KnownCount: readyCount, TotalCount: len(requested), EvidenceRefs: unique(refs)},
		{ID: "target_post_fx_same_tap_baseline", Required: true, Status: status(uniform && uniformTap == "track_post_fader" && readyCount == len(requested)), EvidenceRefs: unique(refs)},
		{ID: "current_track_revision", Required: true, Status: status(currentRevisionCount == len(requested) && len(requested) > 0), KnownCount: currentRevisionCount, TotalCount: len(requested), EvidenceRefs: unique(refs)},
	})
	baseline := TargetPostFXBaseline{
		SchemaVersion: TargetPostFXBaselineSchema, SessionID: strings.TrimSpace(sessionID), Phase: strings.TrimSpace(phase),
		ProjectID: strings.TrimSpace(projectID), TapPoint: firstNonEmpty(uniformTap, "unknown"), RequestedTrackIDs: requested,
		Tracks: tracks, Readiness: readiness, EvidenceRefs: unique(refs), CreatedAt: createdAt.UTC().Format(time.RFC3339Nano),
	}
	baseline.BaselineID = stableID("fcb", map[string]any{"project_id": baseline.ProjectID, "tap_point": baseline.TapPoint, "tracks": baseline.Tracks})
	return baseline
}

func VerifyTargetPostFXBaselines(before, after TargetPostFXBaseline) Verification {
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
		beforeRows := targetEvidenceByTrack(before.Tracks)
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
	verificationStatus := "inconclusive"
	if comparable {
		verificationStatus = "observed"
	}
	return Verification{
		SchemaVersion: VerificationSchema, Status: verificationStatus, Comparable: comparable, PostFXComparable: comparable,
		BeforeObservation: before.BaselineID, AfterObservation: after.BaselineID, TapPoint: before.TapPoint,
		TrackIDs: append([]string(nil), before.RequestedTrackIDs...), ChangedDimensions: unique(changed), Reasons: unique(reasons),
		EvidenceRefs: unique(append(append([]string(nil), before.EvidenceRefs...), after.EvidenceRefs...)), UserAcceptance: "unknown",
	}
}

func compactTargetMetrics(row map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"rms_dbfs", "peak_dbfs", "headroom_db", "crest_db", "balance_db", "correlation_estimate"} {
		if value, ok := row[key]; ok {
			out[key] = value
		}
	}
	return out
}

func targetEvidenceByTrack(rows []TargetProbeEvidence) map[string]TargetProbeEvidence {
	out := map[string]TargetProbeEvidence{}
	for _, row := range rows {
		out[row.TrackID] = row
	}
	return out
}
