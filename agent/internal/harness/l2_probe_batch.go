package harness

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/mixboard"
)

type L2RenderProbeBatchRequest struct {
	SessionID            string
	GoalText             string
	TrackIDs             []string
	TapPoint             string
	FeatureSnapshotPath  string
	ForceFresh           bool
	StartSeconds         float64
	EndSeconds           float64
	TailSeconds          float64
	RequireMaskingFrames bool
}

type L2RenderProbeBatchResult struct {
	Status            string           `json:"status"`
	TapPoint          string           `json:"tap_point"`
	Rows              []map[string]any `json:"rows"`
	RequestedTrackIDs []string         `json:"requested_track_ids"`
	RenderedTrackIDs  []string         `json:"rendered_track_ids,omitempty"`
	CacheHitTrackIDs  []string         `json:"cache_hit_track_ids,omitempty"`
	EvidenceRefs      []string         `json:"evidence_refs,omitempty"`
	ElapsedMS         int64            `json:"elapsed_ms"`
}

// CollectL2RenderProbeBatch reuses the existing Kernel L2 Render Probe path
// but deliberately skips MixBoard RequestObservation. It returns only compact
// per-track evidence for a specialist target preflight or verification.
func (h *Harness) CollectL2RenderProbeBatch(ctx context.Context, req L2RenderProbeBatchRequest) (L2RenderProbeBatchResult, error) {
	started := time.Now()
	result := L2RenderProbeBatchResult{Status: "missing", TapPoint: firstNonEmpty(strings.TrimSpace(req.TapPoint), "track_post_fader")}
	if h == nil {
		return result, fmt.Errorf("harness is unavailable")
	}
	trackIDs := uniqueSortedProbeTrackIDs(req.TrackIDs)
	result.RequestedTrackIDs = trackIDs
	if len(trackIDs) == 0 {
		return result, fmt.Errorf("at least one exact track id is required")
	}
	state := h.UserStateSummary(ctx)
	if refresh := h.refreshShadowWithStatus(ctx, "c1_target_l2_probe_batch"); boolValueDefault(refresh["shadow_refreshed"], false) {
		state = h.UserStateSummary(ctx)
	}
	for _, trackID := range trackIDs {
		fingerprint := mixObservationTrackStateFingerprint(state, trackID)
		if fingerprint == "" {
			return result, fmt.Errorf("track %s is absent from the current project state", trackID)
		}
		cmd := map[string]any{
			"scope": "track", "track_id": trackID, "project_context": true, "observation_only": true,
			"tap_point": result.TapPoint, "mix_session_id": strings.TrimSpace(req.SessionID), "goal_text": strings.TrimSpace(req.GoalText),
		}
		if req.EndSeconds > req.StartSeconds {
			cmd["range"] = []any{req.StartSeconds, req.EndSeconds}
			cmd["tail_seconds"] = req.TailSeconds
		}
		if strings.TrimSpace(req.FeatureSnapshotPath) != "" {
			cmd["feature_snapshot_path"] = strings.TrimSpace(req.FeatureSnapshotPath)
		}
		cmd = mixObservationCommandWithLiveProjectIdentity(cmd, state)
		target := mixTargetFromCommand(cmd)
		resolved := resolveMixObservationTargetContext(cmd, state, target)
		cmd = canonicalizeMixObservationCommand(cmd, target, resolved)
		if !req.ForceFresh {
			if row := cachedL2RenderProbeForStateRange(cmd, trackID, result.TapPoint, fingerprint, req.StartSeconds, req.EndSeconds, req.TailSeconds, req.RequireMaskingFrames); len(row) > 0 {
				result.Rows = append(result.Rows, compactL2RenderProbeBatchRow(row))
				result.CacheHitTrackIDs = append(result.CacheHitTrackIDs, trackID)
				result.EvidenceRefs = append(result.EvidenceRefs, firstString(row, "evidence_ref"))
				continue
			}
		}
		packet := h.requestMixObservationL2RenderProbe(ctx, cmd, state, target, resolved)
		requestID := firstString(packet, "request_id")
		row := l2RenderProbeRowByRequest(cmd, requestID)
		if len(row) == 0 || !strings.EqualFold(firstString(row, "status"), "ready") || (req.RequireMaskingFrames && len(mapAnyFromAny(row["masking_frames"])) == 0) {
			return result, fmt.Errorf("track %s L2 probe did not produce ready evidence: %s", trackID, firstNonEmpty(firstString(packet, "reason"), firstString(packet, "status"), "missing"))
		}
		result.Rows = append(result.Rows, compactL2RenderProbeBatchRow(row))
		result.RenderedTrackIDs = append(result.RenderedTrackIDs, trackID)
		result.EvidenceRefs = append(result.EvidenceRefs, firstString(row, "evidence_ref"))
	}
	result.EvidenceRefs = uniqueSortedProbeTrackIDs(result.EvidenceRefs)
	result.Status = "ready"
	result.ElapsedMS = time.Since(started).Milliseconds()
	return result, nil
}

func cachedL2RenderProbeForState(cmd map[string]any, trackID, tapPoint, fingerprint string) map[string]any {
	return cachedL2RenderProbeForStateRange(cmd, trackID, tapPoint, fingerprint, 0, 0, 0, false)
}

func cachedL2RenderProbeForStateRange(cmd map[string]any, trackID, tapPoint, fingerprint string, startSeconds, endSeconds, tailSeconds float64, requireMaskingFrames bool) map[string]any {
	path := mixboard.FeatureSnapshotPath(cmd)
	mixboardFeatureSnapshotWriteMu.Lock()
	snapshot := readMixboardFeatureSnapshotFile(path)
	mixboardFeatureSnapshotWriteMu.Unlock()
	rows := mapRowsFromAny(snapshot["l2_render_probes"])
	if primary, _ := snapshot["l2_render_probe"].(map[string]any); len(primary) > 0 {
		rows = append(rows, primary)
	}
	for index := len(rows) - 1; index >= 0; index-- {
		row := rows[index]
		if firstString(row, "track_id") != trackID || !strings.EqualFold(firstString(row, "tap_point"), tapPoint) {
			continue
		}
		if !strings.EqualFold(firstString(row, "status"), "ready") || firstString(row, "render_revision") == "" || firstString(row, "evidence_ref") == "" {
			continue
		}
		if firstString(row, "track_state_fingerprint") != fingerprint || len(mapAnyFromAny(row["bands"])) == 0 {
			continue
		}
		if endSeconds > startSeconds {
			analyzed := mapAnyFromAny(row["analyzed_range"])
			if math.Abs(numberFromAny(analyzed["start_seconds"])-startSeconds) > 0.001 || math.Abs(numberFromAny(analyzed["end_seconds"])-endSeconds) > 0.001 {
				continue
			}
			if value, ok := row["tail_seconds"]; ok {
				if math.Abs(numberFromAny(value)-tailSeconds) > 0.001 {
					continue
				}
			} else if captured, ok := mapAnyFromAny(row["quality_evidence"])["tail_captured"]; ok && boolValueDefault(captured, false) != (tailSeconds > 0) {
				continue
			}
		}
		if requireMaskingFrames && !maskingFrameRangeMatches(mapAnyFromAny(row["masking_frames"]), startSeconds, endSeconds) {
			continue
		}
		return row
	}
	return nil
}

func maskingFrameRangeMatches(block map[string]any, startSeconds, endSeconds float64) bool {
	if len(block) == 0 || !strings.EqualFold(firstString(block, "status"), "ready") {
		return false
	}
	frames := mapRowsFromAny(block["frames"])
	if len(frames) == 0 {
		return false
	}
	first, last := frames[0], frames[len(frames)-1]
	return math.Abs(numberFromAny(first["start_seconds"])-startSeconds) <= 0.001 &&
		math.Abs(numberFromAny(last["end_seconds"])-endSeconds) <= 0.001
}

func l2RenderProbeRowByRequest(cmd map[string]any, requestID string) map[string]any {
	if requestID == "" {
		return nil
	}
	path := mixboard.FeatureSnapshotPath(cmd)
	mixboardFeatureSnapshotWriteMu.Lock()
	snapshot := readMixboardFeatureSnapshotFile(path)
	mixboardFeatureSnapshotWriteMu.Unlock()
	rows := mapRowsFromAny(snapshot["l2_render_probes"])
	if primary, _ := snapshot["l2_render_probe"].(map[string]any); len(primary) > 0 {
		rows = append(rows, primary)
	}
	for index := len(rows) - 1; index >= 0; index-- {
		if firstString(rows[index], "request_id") == requestID {
			return rows[index]
		}
	}
	return nil
}

func mixObservationTrackStateFingerprint(state map[string]any, trackID string) string {
	for _, track := range mapRowsFromAny(state["tracks"]) {
		if firstNonEmpty(firstString(track, "track_id"), firstString(track, "id")) != trackID {
			continue
		}
		project := mapAnyFromAny(state["project"])
		payload := map[string]any{
			"project_uuid": firstNonEmpty(firstString(state, "project_uuid", "project_id"), firstString(project, "project_uuid", "uuid", "id")),
			"track":        track,
		}
		data, _ := json.Marshal(payload)
		sum := sha256.Sum256(data)
		return "trackstate_" + hex.EncodeToString(sum[:8])
	}
	return ""
}

func compactL2RenderProbeBatchRow(row map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{
		"schema_version", "status", "quality_status", "project_id", "session_id", "track_id", "clip_id", "tap_point", "render_mode",
		"request_id", "render_revision", "track_state_fingerprint", "source_revision", "clip_revision", "analyzer_revision", "tail_seconds", "bands",
		"rms_dbfs", "peak_dbfs", "headroom_db", "crest_db", "balance_db", "correlation_estimate", "analyzed_range", "masking_frames", "evidence_ref", "updated_at",
	} {
		if value, ok := row[key]; ok {
			out[key] = value
		}
	}
	return out
}

func uniqueSortedProbeTrackIDs(values []string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			seen[value] = true
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
