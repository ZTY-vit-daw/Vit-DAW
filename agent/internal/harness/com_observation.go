package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-zeromq/zmq4"

	"vit-daw-agent/internal/com"
	"vit-daw-agent/internal/mixboard"
)

func (h *Harness) prepareMixObservationCOMEvidence(ctx context.Context, cmd map[string]any, target mixboard.TargetRef, resolved map[string]any) (map[string]any, map[string]any, error) {
	out := cloneAnyMap(cmd)
	mode := strings.ToLower(firstString(out, "com_mode"))
	if mode != com.ModePairedIO || firstString(out, "com_artifact_path") != "" {
		return out, nil, nil
	}
	trackID := firstNonEmpty(firstString(out, "track_id"), firstString(resolved, "track_id"))
	pluginID := firstString(out, "plugin_id", "plugin_instance_id")
	clipID := firstNonEmpty(firstString(out, "clip_id"), firstString(resolved, "clip_id"))
	topologyClass := firstString(out, "topology_class")
	topologyGeneration := firstString(out, "topology_generation")
	if trackID == "" || pluginID == "" || topologyClass == "" || topologyGeneration == "" {
		return out, nil, fmt.Errorf("COM paired_io live capture requires track_id, plugin_id, topology_class, and topology_generation")
	}
	startSample := int64(numberFromAny(out["start_sample"]))
	endSample := int64(numberFromAny(out["end_sample"]))
	exactSampleWindow := endSample > startSample
	requestID := firstNonEmpty(firstString(out, "com_request_id"), "mixboard_com2_"+safeRequestIDPart(trackID)+"_"+time.Now().UTC().Format("20060102T150405.000000000"))
	kernelCmd := map[string]any{
		"cmd": "compressor_dual_tap_probe", "request_id": requestID,
		"track_id": trackID, "plugin_id": pluginID, "topology_class": topologyClass,
		"topology_generation": topologyGeneration, "support_class": "single_band_broadband",
		"deterministic": true,
	}
	windowSource := "explicit_sample_window"
	if exactSampleWindow {
		kernelCmd["start_sample"], kernelCmd["end_sample"] = startSample, endSample
	} else {
		if !boolValueDefault(out["resolve_semantic_compressor_window"], false) {
			return out, nil, fmt.Errorf("COM paired_io live capture requires an exact start_sample/end_sample window")
		}
		startSeconds, endSeconds, source, err := semanticCompressorObservationSeconds(out, resolved)
		if err != nil {
			return out, nil, err
		}
		kernelCmd["start_seconds"], kernelCmd["end_seconds"] = startSeconds, endSeconds
		windowSource = source
	}
	if clipID != "" {
		kernelCmd["clip_id"] = clipID
	}
	collector := h.comProbeCollect
	if collector == nil {
		collector = h.collectMixObservationCOMProbe
	}
	receipt, reply, err := collector(ctx, kernelCmd, requestID, trackID, pluginID)
	if err != nil {
		return out, nil, err
	}
	if len(receipt) == 0 {
		return out, nil, fmt.Errorf("COM paired_io terminal receipt was not observed")
	}
	receiptRaw, _ := json.Marshal(receipt)
	typedReceipt, err := com.DecodePairedEvidenceReceipt(receiptRaw)
	if err != nil {
		return out, nil, err
	}
	expectation := com.PairedEvidenceExpectation{
		TrackID: trackID, ClipID: clipID, PluginInstanceID: pluginID, TopologyGeneration: topologyGeneration,
	}
	if exactSampleWindow {
		expectation.StartSample, expectation.EndSample = startSample, endSample
	}
	validation := com.ValidatePairedEvidenceReceipt(typedReceipt, expectation)
	if !validation.Ready {
		return out, nil, fmt.Errorf("COM paired_io receipt failed validation: %s", strings.Join(validation.Reasons, ","))
	}
	artifactPath := comEvidenceArtifactPath(typedReceipt.PairID)
	if artifactPath == "" {
		return out, nil, fmt.Errorf("COM evidence workspace root is unavailable")
	}
	if info, statErr := os.Stat(artifactPath); statErr != nil || !info.Mode().IsRegular() || info.Size() <= 0 {
		return out, nil, fmt.Errorf("COM evidence artifact is unavailable for pair %s", typedReceipt.PairID)
	}
	out["com_artifact_path"] = artifactPath
	out["clip_id"] = typedReceipt.ClipID
	out["start_sample"], out["end_sample"] = typedReceipt.StartSample, typedReceipt.EndSample
	out["sample_rate"], out["channel_count"] = typedReceipt.SampleRate, typedReceipt.ChannelCount
	capture := map[string]any{
		"status": typedReceipt.Status, "request_id": requestID, "pair_id": typedReceipt.PairID,
		"evidence_ref": typedReceipt.EvidenceRef, "artifact_sha256": typedReceipt.ArtifactSHA256,
		"track_id": typedReceipt.TrackID, "clip_id": typedReceipt.ClipID,
		"plugin_instance_id": typedReceipt.PluginInstanceID, "processor_state_hash": typedReceipt.ProcessorStateHash,
		"render_revision": typedReceipt.RenderRevision, "source_revision": typedReceipt.SourceRevision,
		"start_sample": typedReceipt.StartSample, "end_sample": typedReceipt.EndSample,
		"start_seconds": float64(typedReceipt.StartSample) / typedReceipt.SampleRate,
		"end_seconds":   float64(typedReceipt.EndSample) / typedReceipt.SampleRate,
		"sample_rate":   typedReceipt.SampleRate, "channel_count": typedReceipt.ChannelCount,
		"window_source": windowSource,
	}
	if len(reply) > 0 {
		capture["kernel_reply"] = compactSelectedAny(reply, []string{"status", "pair_id", "request_id", "track_id", "clip_id", "plugin_instance_id", "message", "error"})
	}
	return out, capture, nil
}

const (
	semanticCompressorDefaultObservationSeconds = 10.0
	semanticCompressorMaxObservationSeconds     = 120.0
)

func semanticCompressorObservationSeconds(cmd, resolved map[string]any) (float64, float64, string, error) {
	clipStart := firstNumberDefault(resolved, 0, "clip_start_seconds", "start_seconds", "start_time", "position_seconds")
	clipDuration := firstPositiveNumber(resolved, "duration_seconds", "length_seconds", "duration")
	clipEnd := clipStart + clipDuration

	for _, candidate := range semanticCompressorExplicitTimeRanges(cmd) {
		start, startOK := firstNumber(candidate.row, "start_seconds", "range_start_seconds", "start")
		end, endOK := firstNumber(candidate.row, "end_seconds", "range_end_seconds", "end")
		if !startOK || !endOK || end <= start {
			continue
		}
		if clipDuration > 0 {
			start = math.Max(start, clipStart)
			end = math.Min(end, clipEnd)
		}
		if end <= start {
			return 0, 0, "", fmt.Errorf("semantic compressor observation range does not overlap the resolved clip")
		}
		if end-start > semanticCompressorMaxObservationSeconds {
			return 0, 0, "", fmt.Errorf("semantic compressor observation range exceeds 120 seconds")
		}
		return start, end, candidate.source, nil
	}

	start := clipStart
	if playhead, ok := firstNumber(cmd, "current_playhead_seconds", "playhead_seconds", "transport_position_seconds"); ok &&
		playhead >= clipStart && (clipDuration <= 0 || playhead < clipEnd) {
		start = playhead
	}
	end := start + semanticCompressorDefaultObservationSeconds
	if clipDuration > 0 {
		end = math.Min(end, clipEnd)
	}
	if end <= start {
		return 0, 0, "", fmt.Errorf("semantic compressor observation clip has no usable time window")
	}
	return start, end, "resolved_clip_default_10s", nil
}

type semanticCompressorTimeRange struct {
	row    map[string]any
	source string
}

func semanticCompressorExplicitTimeRanges(cmd map[string]any) []semanticCompressorTimeRange {
	out := []semanticCompressorTimeRange{}
	appendRange := func(value any, source string) {
		row := mapFromAny(value)
		if len(row) > 0 && (!rowHasBool(row, "active") || boolValueDefault(row["active"], false)) {
			out = append(out, semanticCompressorTimeRange{row: row, source: source})
		}
	}
	appendRange(cmd["selected_clip_range"], "selected_clip_range")
	for _, row := range mapRowsFromAny(cmd["selected_clip_ranges"]) {
		appendRange(row, "selected_clip_ranges")
		break
	}
	appendRange(cmd["time_selection"], "time_selection")
	selectedClipWindow := mapFromAny(cmd["selected_clip_time_range"])
	if !strings.EqualFold(firstString(selectedClipWindow, "source"), "selected_clip") {
		appendRange(selectedClipWindow, "selected_clip")
	}
	if start, startOK := firstNumber(cmd, "listen_time_start_seconds"); startOK {
		if end, endOK := firstNumber(cmd, "listen_time_end_seconds"); endOK {
			out = append(out, semanticCompressorTimeRange{row: map[string]any{"start_seconds": start, "end_seconds": end}, source: "listen_time"})
		}
	}
	return out
}

func firstNumber(row map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		value, ok := row[key]
		if !ok || value == nil {
			continue
		}
		number := numberFromAny(value)
		if !math.IsNaN(number) && !math.IsInf(number, 0) {
			return number, true
		}
	}
	return 0, false
}

func firstNumberDefault(row map[string]any, fallback float64, keys ...string) float64 {
	if value, ok := firstNumber(row, keys...); ok {
		return value
	}
	return fallback
}

func rowHasBool(row map[string]any, key string) bool {
	_, ok := row[key]
	return ok
}

func (h *Harness) collectMixObservationCOMProbe(ctx context.Context, kernelCmd map[string]any, requestID, trackID, pluginID string) (map[string]any, map[string]any, error) {
	if h == nil || h.kernel == nil {
		return nil, nil, fmt.Errorf("kernel_client_unavailable")
	}
	timeout := mixboardL2RenderProbeCollectWait()
	if timeout < 30*time.Second {
		timeout = 30 * time.Second
	}
	var reply map[string]any
	var sendErr error
	send := func() error {
		sendCtx, cancel := context.WithTimeout(context.Background(), mixboardFeatureBackgroundSendWait())
		defer cancel()
		reply, _, sendErr = h.kernel.SendCommand(sendCtx, kernelCmd)
		if sendErr != nil {
			return sendErr
		}
		if !kernelReplySucceeded(reply) {
			return fmt.Errorf("%s", firstNonEmpty(firstString(reply, "message", "error"), "compressor_dual_tap_probe request failed"))
		}
		return nil
	}
	receipt, err := collectCOMProbeEvent(ctx, mixboardFeatureSubURL, requestID, trackID, pluginID, timeout, send)
	return receipt, reply, err
}

func collectCOMProbeEvent(ctx context.Context, subURL, requestID, trackID, pluginID string, timeout time.Duration, afterSubscribe func() error) (map[string]any, error) {
	opCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	sub := zmq4.NewSub(opCtx, zmq4.WithTimeout(120*time.Millisecond), zmq4.WithAutomaticReconnect(true))
	defer sub.Close()
	if err := sub.SetOption(zmq4.OptionSubscribe, ""); err != nil {
		return nil, err
	}
	if err := sub.Dial(firstNonEmpty(subURL, mixboardFeatureSubURL)); err != nil {
		return nil, err
	}
	warmMixboardFeatureSubscriber(opCtx)
	if afterSubscribe != nil {
		if err := afterSubscribe(); err != nil {
			return nil, err
		}
	}
	for {
		select {
		case <-opCtx.Done():
			return nil, nil
		default:
		}
		msg, err := sub.Recv()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timeout") {
				continue
			}
			if errors.Is(err, context.Canceled) {
				return nil, nil
			}
			return nil, err
		}
		event := map[string]any{}
		if json.Unmarshal([]byte(zmqMsgPayload(msg)), &event) != nil || firstString(event, "feature_type") != "compressor_dual_tap_probe" {
			continue
		}
		if requestID != "" && firstString(event, "request_id") != requestID {
			continue
		}
		if trackID != "" && firstString(event, "track_id") != trackID {
			continue
		}
		if pluginID != "" && firstString(event, "plugin_instance_id") != pluginID {
			continue
		}
		if command := firstString(event, "command"); command == "compressor_dual_tap_probe_ready" || command == "compressor_dual_tap_probe_failed" {
			return event, nil
		}
	}
}

func comEvidenceArtifactPath(pairID string) string {
	for _, key := range []string{"VIT_DAW_DEV_ROOT", "VIT_DEV_ROOT", "VIT_ROOT"} {
		if root := strings.TrimSpace(os.Getenv(key)); root != "" {
			return filepath.Join(root, "VitApp", "Workspace", "Artifacts", "com_evidence", pairID, pairID+".json")
		}
	}
	return ""
}
