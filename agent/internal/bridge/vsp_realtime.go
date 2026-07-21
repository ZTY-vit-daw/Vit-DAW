package bridge

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"vit-daw-agent/internal/vspclient"
)

const (
	vspRealtimeClientID      = "vit.agent.realtime_publisher"
	vspRealtimeClientName    = "VitAgent Realtime Publisher"
	vspRealtimeClientVersion = "vsp-realtime-publisher-v1"
)

var vspRealtimeFrameSeq atomic.Int64

func (b *Bridge) enqueueVSPRealtimeTelemetry(packet map[string]any) {
	if b == nil || b.realtimePublishCh == nil {
		return
	}
	payload, ok := b.vspRealtimePublishPayload(packet)
	if !ok {
		return
	}
	select {
	case b.realtimePublishCh <- payload:
		return
	default:
	}
	select {
	case <-b.realtimePublishCh:
	default:
	}
	select {
	case b.realtimePublishCh <- payload:
	default:
	}
}

func (b *Bridge) runVSPRealtimePublisher(ctx context.Context) {
	hubURL := strings.TrimSpace(b.cfg.VSPHubURL)
	if hubURL == "" {
		return
	}
	client := vspclient.New(hubURL, vspRealtimeClientID, "agent", vspRealtimeClientName, vspRealtimeClientVersion, 2*time.Second)
	if b.logger != nil {
		b.logger.Info("VSP realtime publisher enabled hub=%s", hubURL)
	}
	nextWarnAt := time.Time{}
	failures := 0
	for {
		select {
		case <-ctx.Done():
			return
		case payload := <-b.realtimePublishCh:
			if len(payload) == 0 {
				continue
			}
			if err := b.publishVSPRealtimePayload(ctx, client, payload); err != nil {
				failures++
				if b.logger != nil && time.Now().After(nextWarnAt) {
					b.logger.Warn("VSP realtime publish pending failures=%d error=%v", failures, err)
					nextWarnAt = time.Now().Add(5 * time.Second)
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(250 * time.Millisecond):
				}
				continue
			}
			failures = 0
		}
	}
}

func (b *Bridge) publishVSPRealtimePayload(ctx context.Context, client *vspclient.Client, payload map[string]any) error {
	if client == nil {
		return fmt.Errorf("VSP realtime client is nil")
	}
	if strings.TrimSpace(client.SessionID) == "" {
		opCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		_, err := client.Hello(opCtx,
			[]string{"realtime.publish"},
			[]string{"vsp.hub.http"},
		)
		cancel()
		if err != nil {
			return err
		}
	}
	opCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	_, err := client.Send(opCtx, client.SessionID, "realtime", "realtime.publish", "vsp.realtime.publish.v1", payload)
	cancel()
	if err != nil {
		client.SessionID = ""
		return err
	}
	return nil
}

func (b *Bridge) vspRealtimePublishPayload(packet map[string]any) (map[string]any, bool) {
	topic := strings.ToLower(strings.TrimSpace(stringField(packet, "topic")))
	switch topic {
	case "transport":
		return map[string]any{
			"frames": []map[string]any{b.vspTransportFrame(packet)},
		}, true
	case "levels":
		frames := make([]map[string]any, 0, 2)
		tracks := realtimeTracksFromTelemetry(packet["tracks"])
		if len(tracks) == 0 {
			return nil, false
		}
		frames = append(frames, b.vspMetersFrame(tracks))
		frames = append(frames, b.vspSpectrumReferenceFrame(tracks))
		return map[string]any{"frames": frames}, true
	default:
		return nil, false
	}
}

func (b *Bridge) vspTransportFrame(packet map[string]any) map[string]any {
	data := map[string]any{
		"source":           "engine_telemetry",
		"timestamp_ms":     time.Now().UnixMilli(),
		"position_seconds": float64FromAny(packet["position_seconds"]),
		"is_playing":       boolFromAny(packet["is_playing"]),
		"is_recording":     boolFromAny(packet["is_recording"]),
	}
	if value, ok := packet["position_beats"]; ok {
		data["position_beats"] = float64FromAny(value)
	}
	if value, ok := packet["recording_waveforms"]; ok {
		data["recording_waveforms"] = cloneAny(value)
	}
	return map[string]any{
		"stream":      "transport.playhead",
		"frame_index": vspRealtimeFrameSeq.Add(1),
		"data":        data,
	}
}

func (b *Bridge) vspMetersFrame(tracks []map[string]any) map[string]any {
	trackIDs := make([]string, 0, len(tracks))
	for _, track := range tracks {
		if id := firstNonEmptyString(stringField(track, "track_id"), stringField(track, "id")); id != "" {
			trackIDs = append(trackIDs, id)
		}
	}
	return map[string]any{
		"stream":      "meters.visible_tracks",
		"track_ids":   trackIDs,
		"frame_index": vspRealtimeFrameSeq.Add(1),
		"data": map[string]any{
			"source":              "engine_level_meter",
			"timestamp_ms":        time.Now().UnixMilli(),
			"tracks":              tracks,
			"visible_track_count": len(tracks),
		},
	}
}

func (b *Bridge) vspSpectrumReferenceFrame(tracks []map[string]any) map[string]any {
	refs := make([]map[string]any, 0, len(tracks))
	trackIDs := make([]string, 0, len(tracks))
	for _, track := range tracks {
		trackID := firstNonEmptyString(stringField(track, "track_id"), stringField(track, "id"))
		if trackID == "" {
			continue
		}
		trackIDs = append(trackIDs, trackID)
		source := "engine_level_meter_no_frame"
		if _, ok := track["spectrum_bin_count"]; ok {
			source = "engine_level_meter"
		}
		refs = append(refs, map[string]any{
			"track_id": trackID,
			"kind":     "spectrum_frame",
			"uri":      "vit-cache://project_current/tracks/" + trackID + "/spectrum/latest",
			"source":   source,
		})
	}
	return map[string]any{
		"stream":      "spectrum.visible_tracks",
		"track_ids":   trackIDs,
		"frame_index": vspRealtimeFrameSeq.Add(1),
		"data": map[string]any{
			"source":              "engine_level_meter",
			"timestamp_ms":        time.Now().UnixMilli(),
			"asset_refs":          refs,
			"visible_track_count": len(refs),
			"inline_bins":         false,
		},
	}
}

func realtimeTracksFromTelemetry(value any) []map[string]any {
	rows, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		track, ok := row.(map[string]any)
		if !ok {
			continue
		}
		next := map[string]any{}
		for _, key := range []string{
			"track_id", "id", "name", "track_name", "edit_index",
			"level_db", "peak_db", "rms_db",
			"left_level_db", "right_level_db", "left_peak_db", "right_peak_db",
			"clipped", "peak_held", "channel_count",
			"source", "spectrum_bin_count", "spectrum_min_hz", "spectrum_max_hz",
		} {
			if value, ok := track[key]; ok {
				next[key] = value
			}
		}
		id := firstNonEmptyString(stringField(next, "track_id"), stringField(next, "id"))
		if id == "" {
			continue
		}
		next["track_id"] = id
		next["id"] = id
		if _, ok := next["level_db"]; !ok {
			if _, hasPeak := next["peak_db"]; hasPeak {
				next["level_db"] = float64FromAny(next["peak_db"])
			}
		}
		if _, ok := next["peak_db"]; !ok {
			next["peak_db"] = float64FromAny(next["level_db"])
		}
		if _, ok := next["rms_db"]; !ok {
			next["rms_db"] = float64FromAny(next["level_db"])
		}
		if _, ok := next["source"]; !ok {
			next["source"] = "engine_level_meter"
		}
		out = append(out, next)
	}
	return out
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = cloneAny(v)
	}
	return out
}

func cloneAny(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return cloneMap(v)
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, cloneAny(item))
		}
		return out
	default:
		return value
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func boolFromAny(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "on":
			return true
		}
	}
	return false
}

func float64FromAny(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case jsonNumber:
		f, _ := v.Float64()
		return f
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return 0
	}
	var out float64
	_, _ = fmt.Sscanf(text, "%f", &out)
	return out
}

type jsonNumber interface {
	Float64() (float64, error)
}
