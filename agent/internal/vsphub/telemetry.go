package vsphub

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-zeromq/zmq4"
)

// relay liveness counters power /health diagnostics and the resilience
// injection leg ("hub unreachable window" quantification).
type relayState struct {
	lastRecvUnixMs atomic.Int64
	recvCount      atomic.Int64
	errorCount     atomic.Int64
	lastErrorUnix  atomic.Int64
}

func (h *Hub) relaySnapshot() map[string]any {
	out := map[string]any{
		"recv_count":  h.relay.recvCount.Load(),
		"error_count": h.relay.errorCount.Load(),
	}
	if lastMs := h.relay.lastRecvUnixMs.Load(); lastMs > 0 {
		out["last_recv_at"] = time.UnixMilli(lastMs).UTC().Format(time.RFC3339Nano)
		out["last_recv_age_ms"] = time.Since(time.UnixMilli(lastMs)).Milliseconds()
	}
	if errMs := h.relay.lastErrorUnix.Load(); errMs > 0 {
		out["last_error_at"] = time.UnixMilli(errMs).UTC().Format(time.RFC3339Nano)
	}
	return out
}

func (h *Hub) RunTelemetryRelay(ctx context.Context) error {
	for {
		if err := h.telemetryOnce(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			h.relay.errorCount.Add(1)
			h.relay.lastErrorUnix.Store(time.Now().UTC().UnixMilli())
			if h.logger != nil {
				h.logger.Warn("VSP telemetry relay reconnecting after error: %v", err)
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Second):
			}
			continue
		}
	}
}

func (h *Hub) telemetryOnce(ctx context.Context) error {
	sub := zmq4.NewSub(ctx, zmq4.WithTimeout(time.Second), zmq4.WithAutomaticReconnect(true))
	defer sub.Close()
	if err := sub.SetOption(zmq4.OptionSubscribe, ""); err != nil {
		return err
	}
	if err := sub.Dial(h.cfg.KernelSubURL); err != nil {
		return err
	}
	if h.logger != nil {
		h.logger.Info("VSP telemetry relay started: %s -> %s", h.cfg.KernelSubURL, TransportWebSocket)
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		msg, err := sub.Recv()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timeout") {
				continue
			}
			return err
		}
		h.relay.recvCount.Add(1)
		h.relay.lastRecvUnixMs.Store(time.Now().UTC().UnixMilli())
		payload := string(msgPayload(msg))
		h.broadcastTelemetry(payload)
	}
}

func (h *Hub) broadcastTelemetry(raw string) {
	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		parsed = map[string]any{"raw": raw}
	}
	h.assets.RecordTelemetry(parsed)
	h.events.RecordTelemetry(parsed)
	if h.streams.Count() == 0 {
		return
	}
	data := h.telemetryEventEnvelope(parsed, false, 0)
	for _, stream := range h.streams.Clients() {
		if !h.events.ShouldDeliverTelemetry(stream.BoundSessionID(), parsed) {
			continue
		}
		stream.Write(data)
	}
}

func (h *Hub) replayCachedTelemetry(stream *StreamClient) {
	if h == nil || stream == nil {
		return
	}
	cached := h.events.CachedTelemetryForSession(stream.BoundSessionID(), eventReplayMax)
	for _, item := range cached {
		stream.Write(h.telemetryEventEnvelope(item.Telemetry, true, item.Seq))
	}
}

func (h *Hub) telemetryEventEnvelope(telemetry any, replay bool, replaySeq int64) []byte {
	payload := map[string]any{
		"status":    "ok",
		"source":    "kernel.telemetry",
		"telemetry": telemetry,
	}
	if replay {
		payload["replay"] = true
		payload["replay_seq"] = replaySeq
	}
	env := map[string]any{
		"vsp_version": VSPVersion,
		"schema":      "vsp.event.notification.v1",
		"message_id":  NewID("msg_hub"),
		"session_id":  "broadcast",
		"client_id":   "vsp.hub",
		"role":        "hub",
		"channel":     "event",
		"type":        "event.notification",
		"created_at":  time.Now().UTC().Format(time.RFC3339Nano),
		"payload":     payload,
	}
	data, _ := json.Marshal(env)
	return append(data, '\n')
}

func msgPayload(msg zmq4.Msg) []byte {
	if len(msg.Frames) == 0 {
		return nil
	}
	return msg.Frames[len(msg.Frames)-1]
}
