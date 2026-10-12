package chat

import (
	"context"
	"encoding/json"
	"os"

	"vit-daw-agent/internal/fxm"
	"vit-daw-agent/internal/harness"
)

// M2-TIMEFX-2 FXM differential producer wiring: after a successful reverb or
// delay semantic control write (apply or restore - both are chain-state
// writes), automatically trigger one kernel timefx_dual_tap_probe on the
// written processor, assemble the fxm.Input from the returned evidence
// artifact, and attach the FXM projection to the apply result.
//
// The probe is best-effort evidence enrichment: it must never turn an
// already-successful parameter write into a failure. Every failure mode
// (kernel unavailable, probe rejected, artifact unreadable, artifact
// rejected by the fail-closed gates) is disclosed in the attached map.

const (
	timefxProbeAutoRequestIDPrefix = "timefx_fx_"
	timefxFXMResultKey             = "fxm_probe"
)

// attachTimefxFXMProbe runs the post-write differential probe and stores the
// outcome under result["fxm_probe"]. When the probe completes with a ready
// artifact the payload carries status=ready, the compact FXM projection, the
// full fxm_measurement (the mixboard observation path consumes the same
// shape via req.Args["fxm_measurement"]), and the differential numbers.
func (s *Server) attachTimefxFXMProbe(ctx context.Context, result map[string]any, trackID, pluginID, family, topologyClass, generation string) {
	if result == nil {
		return
	}
	outcome := s.runTimefxFXMProbe(ctx, trackID, pluginID, family, topologyClass, generation)
	if outcome == nil {
		return
	}
	result[timefxFXMResultKey] = outcome
}

func (s *Server) runTimefxFXMProbe(ctx context.Context, trackID, pluginID, family, topologyClass, generation string) map[string]any {
	failure := func(reason string) map[string]any {
		return map[string]any{"status": "failed", "reason": reason, "track_id": trackID, "plugin_id": pluginID, "family": family}
	}
	if s.harness == nil {
		return failure("harness_unavailable")
	}
	if topologyClass == "" || generation == "" {
		return failure("recognizer_identity_unavailable")
	}
	requestID := timefxProbeAutoRequestIDPrefix + family + "_" + randomID()
	kernelCmd := map[string]any{
		"cmd": "timefx_dual_tap_probe", "request_id": requestID,
		"track_id": trackID, "plugin_id": pluginID,
		"family": family, "topology_class": topologyClass, "topology_generation": generation,
		"deterministic": true,
	}
	receipt, _, err := s.harness.CollectTimefxDualTapProbe(ctx, kernelCmd, requestID, trackID, pluginID)
	if err != nil {
		return failure("probe_send_failed: " + err.Error())
	}
	if len(receipt) == 0 {
		return failure("probe_receipt_timeout")
	}
	pairID := firstNonEmptyText(receipt, "pair_id")
	status := firstNonEmptyText(receipt, "status")
	if status != "ready" && status != "partial" {
		return failure("probe_not_ready: " + firstNonEmptyText(receipt, "reason", status))
	}
	if pairID == "" {
		return failure("probe_pair_identity_missing")
	}
	artifactPath := harness.TimefxEvidenceArtifactPath(pairID)
	if artifactPath == "" {
		return failure("artifact_root_unavailable")
	}
	raw, err := os.ReadFile(artifactPath)
	if err != nil || len(raw) == 0 {
		return failure("artifact_unreadable")
	}
	artifact, err := fxm.DecodeTimefxEvidenceArtifact(raw)
	if err != nil {
		return failure("artifact_decode_failed: " + err.Error())
	}
	input, err := fxm.InputFromTimefxEvidence(artifact)
	if err != nil {
		return failure("artifact_rejected: " + err.Error())
	}
	projection := fxm.Build(input)
	out := map[string]any{
		"status":         "ready",
		"pair_id":        pairID,
		"evidence_ref":   artifact.EvidenceRef,
		"fxm_status":     projection.Status,
		"fxm_projection": fxm.ContextProjection(projection),
	}
	if measurement, err := json.Marshal(input); err == nil {
		var measurementMap map[string]any
		if json.Unmarshal(measurement, &measurementMap) == nil {
			out["fxm_measurement"] = measurementMap
		}
	}
	if projection.EffectDelta.RMSDB != nil {
		out["rms_delta_db"] = *projection.EffectDelta.RMSDB
	}
	if projection.EffectDelta.LUFSDB != nil {
		out["lufs_delta_db"] = *projection.EffectDelta.LUFSDB
	}
	if projection.EffectDelta.PeakDB != nil {
		out["peak_delta_db"] = *projection.EffectDelta.PeakDB
	}
	if projection.EffectDelta.CrestDB != nil {
		out["crest_delta_db"] = *projection.EffectDelta.CrestDB
	}
	return out
}
