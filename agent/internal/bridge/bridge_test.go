package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"vit-daw-agent/internal/vspclient"
	"vit-daw-agent/internal/vsphub"
)

type bridgeTestKernel struct {
	mu    sync.Mutex
	calls []string
}

func (k *bridgeTestKernel) SendRaw(_ context.Context, payload string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.calls = append(k.calls, payload)
	return `{"vsp_version":"1.0","channel":"session","type":"session.hello_ack","session_id":"sess_agent_realtime","payload":{"status":"ok"}}`, nil
}

func TestControlReplyPayloadKeepsSmallReplyInline(t *testing.T) {
	b := New(Config{FileReplyDir: t.TempDir()}, nil, nil, nil)
	reply := `{"status":"ok"}`

	out, spilled, err := b.controlReplyPayload(map[string]any{"cmd": "ping", "request_id": "req_1"}, reply)
	if err != nil {
		t.Fatalf("controlReplyPayload returned error: %v", err)
	}
	if spilled {
		t.Fatalf("small reply unexpectedly spilled to file")
	}
	if string(out) != reply {
		t.Fatalf("inline reply mismatch: got %q want %q", string(out), reply)
	}
}

func TestControlReplyPayloadSpillsLargeReplyToFile(t *testing.T) {
	dir := t.TempDir()
	b := New(Config{FileReplyDir: dir}, nil, nil, nil)
	reply := fmt.Sprintf(`{"status":"ok","parameters":["%s"]}`, strings.Repeat("x", maxDirectUDPReplyBytes))

	out, spilled, err := b.controlReplyPayload(map[string]any{
		"cmd":        "get_plugin_parameters",
		"request_id": "req/with unsafe chars",
	}, reply)
	if err != nil {
		t.Fatalf("controlReplyPayload returned error: %v", err)
	}
	if !spilled {
		t.Fatalf("large reply was not spilled")
	}

	var envelope map[string]any
	if err := json.Unmarshal(out, &envelope); err != nil {
		t.Fatalf("file reply envelope is not JSON: %v", err)
	}
	if got := fmt.Sprint(envelope["transport"]); got != "file_reply" {
		t.Fatalf("transport = %q, want file_reply", got)
	}
	replyFile := fmt.Sprint(envelope["reply_file"])
	if replyFile == "" {
		t.Fatalf("reply_file missing in envelope: %v", envelope)
	}
	body, err := os.ReadFile(filepath.FromSlash(replyFile))
	if err != nil {
		t.Fatalf("reading spilled reply failed: %v", err)
	}
	if string(body) != reply {
		t.Fatalf("spilled reply body mismatch")
	}
	base := filepath.Base(replyFile)
	if strings.Contains(base, "/") || strings.Contains(base, "\\") || !strings.Contains(base, "get_plugin_parameters") {
		t.Fatalf("unexpected spilled filename: %q", base)
	}
}

func TestTelemetryPayloadKeepsSmallPacketInline(t *testing.T) {
	b := New(Config{FileReplyDir: t.TempDir()}, nil, nil, nil)
	packet := []byte(`{"topic":"levels","tracks":[]}`)

	out, spilled, err := b.telemetryPayload(packet)
	if err != nil {
		t.Fatalf("telemetryPayload returned error: %v", err)
	}
	if spilled {
		t.Fatalf("small telemetry packet unexpectedly spilled to file")
	}
	if string(out) != string(packet) {
		t.Fatalf("inline telemetry mismatch: got %q want %q", string(out), string(packet))
	}
}

func TestTelemetryPayloadSpillsLargePacketToFile(t *testing.T) {
	dir := t.TempDir()
	b := New(Config{FileReplyDir: dir}, nil, nil, nil)
	packet := []byte(fmt.Sprintf(`{"topic":"project","subtopic":"state","tracks":["%s"]}`, strings.Repeat("x", maxDirectUDPTelemetryBytes)))

	out, spilled, err := b.telemetryPayload(packet)
	if err != nil {
		t.Fatalf("telemetryPayload returned error: %v", err)
	}
	if !spilled {
		t.Fatalf("large telemetry packet was not spilled")
	}

	var envelope map[string]any
	if err := json.Unmarshal(out, &envelope); err != nil {
		t.Fatalf("telemetry file envelope is not JSON: %v", err)
	}
	if got := fmt.Sprint(envelope["transport"]); got != "file_reply" {
		t.Fatalf("transport = %q, want file_reply", got)
	}
	telemetryFile := fmt.Sprint(envelope["telemetry_file"])
	if telemetryFile == "" {
		t.Fatalf("telemetry_file missing in envelope: %v", envelope)
	}
	if got := fmt.Sprint(envelope["reply_file"]); got != telemetryFile {
		t.Fatalf("reply_file = %q, want telemetry_file %q", got, telemetryFile)
	}
	body, err := os.ReadFile(filepath.FromSlash(telemetryFile))
	if err != nil {
		t.Fatalf("reading spilled telemetry failed: %v", err)
	}
	if string(body) != string(packet) {
		t.Fatalf("spilled telemetry body mismatch")
	}
	base := filepath.Base(telemetryFile)
	if strings.Contains(base, "/") || strings.Contains(base, "\\") || !strings.Contains(base, "project_state") {
		t.Fatalf("unexpected spilled filename: %q", base)
	}
}

func TestNormalizeTelemetryEnqueuesVSPRealtimeTransportPublishAndSuppressesLegacyUDP(t *testing.T) {
	b := New(Config{FileReplyDir: t.TempDir(), VSPHubURL: "http://127.0.0.1:8787/vsp"}, nil, nil, nil)
	var lastSeq int64
	out := b.normalizeTelemetry(`{"topic":"transport","is_playing":true,"is_recording":false,"position_seconds":12.5}`, nil, &lastSeq)
	if len(out) != 0 {
		t.Fatalf("legacy transport UDP should be suppressed when VSP realtime is enabled: %s", string(out))
	}
	select {
	case payload := <-b.realtimePublishCh:
		frames, ok := payload["frames"].([]map[string]any)
		if !ok || len(frames) != 1 {
			t.Fatalf("frames = %#v", payload["frames"])
		}
		if got := fmt.Sprint(frames[0]["stream"]); got != "transport.playhead" {
			t.Fatalf("stream = %q", got)
		}
		data, ok := frames[0]["data"].(map[string]any)
		if !ok {
			t.Fatalf("frame data = %#v", frames[0]["data"])
		}
		if data["is_playing"] != true || data["position_seconds"] != 12.5 || data["source"] != "engine_telemetry" {
			t.Fatalf("transport data = %#v", data)
		}
	default:
		t.Fatalf("expected realtime publish payload")
	}
}

func TestNormalizeTelemetryEnqueuesVSPRealtimeLevelsPublish(t *testing.T) {
	b := New(Config{FileReplyDir: t.TempDir(), VSPHubURL: "http://127.0.0.1:8787/vsp"}, nil, nil, nil)
	var lastSeq int64
	b.normalizeTelemetry(`{"topic":"levels","tracks":[{"id":"1007","level_db":-6.5,"left_level_db":-7.0,"right_level_db":-8.0,"spectrum_bin_count":64,"spectrum_left":[0.1,0.2],"spectrum_right":[0.3,0.4],"spectrum_phase":[0.0],"spectrum_weight":[1.0]},{"id":"1008","level_db":-12.0}]}`, nil, &lastSeq)
	select {
	case payload := <-b.realtimePublishCh:
		frames, ok := payload["frames"].([]map[string]any)
		if !ok || len(frames) != 2 {
			t.Fatalf("frames = %#v", payload["frames"])
		}
		if fmt.Sprint(frames[0]["stream"]) != "meters.visible_tracks" {
			t.Fatalf("first frame = %#v", frames[0])
		}
		meterData := frames[0]["data"].(map[string]any)
		tracks := meterData["tracks"].([]map[string]any)
		if len(tracks) != 2 || tracks[0]["track_id"] != "1007" || tracks[0]["peak_db"] != -6.5 {
			t.Fatalf("meter tracks = %#v", tracks)
		}
		if _, ok := tracks[0]["spectrum_left"]; ok {
			t.Fatalf("meter tracks should not carry inline spectrum arrays: %#v", tracks[0])
		}
		if tracks[0]["spectrum_bin_count"] != float64(64) {
			t.Fatalf("meter tracks should keep light spectrum metadata: %#v", tracks[0])
		}
		if fmt.Sprint(frames[1]["stream"]) != "spectrum.visible_tracks" {
			t.Fatalf("second frame = %#v", frames[1])
		}
		spectrumData := frames[1]["data"].(map[string]any)
		refs := spectrumData["asset_refs"].([]map[string]any)
		if len(refs) != 2 || !strings.Contains(fmt.Sprint(refs[0]["uri"]), "1007") {
			t.Fatalf("spectrum refs = %#v", refs)
		}
	default:
		t.Fatalf("expected realtime publish payload")
	}
}

func TestNormalizeTelemetrySuppressesLegacyLevelsUDPWhenVSPRealtimeEnabled(t *testing.T) {
	t.Setenv("VIT_BRIDGE_KEEP_LEGACY_REALTIME_UDP", "0")
	b := New(Config{FileReplyDir: t.TempDir(), VSPHubURL: "http://127.0.0.1:8787/vsp"}, nil, nil, nil)
	var lastSeq int64
	out := b.normalizeTelemetry(`{"topic":"levels","tracks":[{"id":"1007","level_db":-6.5}]}`, nil, &lastSeq)
	if len(out) != 0 {
		t.Fatalf("legacy levels UDP should be suppressed when VSP realtime is enabled: %s", string(out))
	}
	select {
	case payload := <-b.realtimePublishCh:
		frames, ok := payload["frames"].([]map[string]any)
		if !ok || len(frames) == 0 {
			t.Fatalf("expected VSP realtime publish payload, got %#v", payload)
		}
	default:
		t.Fatalf("expected VSP realtime payload despite UDP suppression")
	}
}

func TestNormalizeTelemetrySuppressesLegacyTransportUDPWhenVSPRealtimeEnabled(t *testing.T) {
	t.Setenv("VIT_BRIDGE_KEEP_LEGACY_REALTIME_UDP", "0")
	b := New(Config{FileReplyDir: t.TempDir(), VSPHubURL: "http://127.0.0.1:8787/vsp"}, nil, nil, nil)
	var lastSeq int64
	out := b.normalizeTelemetry(`{"topic":"transport","is_playing":true,"position_seconds":8.25}`, nil, &lastSeq)
	if len(out) != 0 {
		t.Fatalf("legacy transport UDP should be suppressed when VSP realtime is enabled: %s", string(out))
	}
	select {
	case payload := <-b.realtimePublishCh:
		frames, ok := payload["frames"].([]map[string]any)
		if !ok || len(frames) != 1 {
			t.Fatalf("expected one VSP transport frame, got %#v", payload["frames"])
		}
		if got := fmt.Sprint(frames[0]["stream"]); got != "transport.playhead" {
			t.Fatalf("stream = %q", got)
		}
	default:
		t.Fatalf("expected VSP realtime transport payload despite UDP suppression")
	}
}

func TestNormalizeTelemetrySuppressesLegacyDeltaUDPWhenVSPHubEnabled(t *testing.T) {
	t.Setenv("VIT_BRIDGE_KEEP_LEGACY_REALTIME_UDP", "0")
	t.Setenv("VIT_BRIDGE_KEEP_LEGACY_DELTA_UDP", "0")
	b := New(Config{FileReplyDir: t.TempDir(), VSPHubURL: "http://127.0.0.1:8787/vsp"}, nil, nil, nil)
	var lastSeq int64
	out := b.normalizeTelemetry(`{"type":"delta_update","seq_id":42,"action":"node_added","target_uid":"track_1007","value":{"id":"1007"}}`, nil, &lastSeq)
	if len(out) != 0 {
		t.Fatalf("legacy delta UDP should be suppressed when VSP hub is enabled: %s", string(out))
	}
	if lastSeq != 42 {
		t.Fatalf("lastSeq = %d, want 42", lastSeq)
	}
	select {
	case payload := <-b.realtimePublishCh:
		t.Fatalf("delta_update should be delivered by hub event relay, not realtime publisher: %#v", payload)
	default:
	}
}

func TestNormalizeTelemetryKeepsLegacyDeltaUDPWithoutVSPHub(t *testing.T) {
	b := New(Config{FileReplyDir: t.TempDir()}, nil, nil, nil)
	var lastSeq int64
	out := b.normalizeTelemetry(`{"type":"delta_update","seq_id":7,"action":"node_added","target_uid":"track_1007"}`, nil, &lastSeq)
	if len(out) == 0 {
		t.Fatalf("legacy delta UDP should stay enabled when VSP hub is absent")
	}
	if !strings.Contains(string(out), `"delta_update"`) {
		t.Fatalf("unexpected delta output: %s", string(out))
	}
	if lastSeq != 7 {
		t.Fatalf("lastSeq = %d, want 7", lastSeq)
	}
}

func TestVSPRealtimePublisherDisabledWithoutHubURL(t *testing.T) {
	b := New(Config{FileReplyDir: t.TempDir()}, nil, nil, nil)
	if b.realtimePublishCh != nil {
		t.Fatalf("realtime publisher should be disabled when VSPHubURL is empty")
	}
	var lastSeq int64
	b.normalizeTelemetry(`{"topic":"transport","is_playing":true}`, nil, &lastSeq)
	if b.realtimePublishCh != nil {
		t.Fatalf("realtime publisher unexpectedly enabled")
	}
}

func TestVSPRealtimePublisherSendsFramesToHubCache(t *testing.T) {
	hub := vsphub.New(vsphub.Config{HTTPAddr: "127.0.0.1:0"}, &bridgeTestKernel{}, nil)
	server := httptest.NewServer(hub.Routes())
	defer server.Close()

	client := vspclient.New(server.URL+"/vsp", vspRealtimeClientID, "agent", vspRealtimeClientName, vspRealtimeClientVersion, 2*time.Second)
	b := New(Config{FileReplyDir: t.TempDir()}, nil, nil, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := b.publishVSPRealtimePayload(ctx, client, map[string]any{
		"frames": []map[string]any{
			{
				"stream":      "transport.playhead",
				"frame_index": 99,
				"data": map[string]any{
					"source":           "engine_telemetry",
					"position_seconds": 9.5,
					"is_playing":       true,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("publishVSPRealtimePayload returned error: %v", err)
	}

	status := hub.Status()
	realtime := status["realtime"].(map[string]any)
	if realtime["published_frames"].(int64) != 1 {
		t.Fatalf("hub realtime diagnostics = %#v", realtime)
	}

	reqBody := `{"vsp_version":"1.0","schema":"vsp.realtime.frame_request.v1","message_id":"msg_test_frame","session_id":"sess_gui","client_id":"godot.gui","role":"gui","channel":"realtime","type":"realtime.frame_request","created_at":"2026-07-03T00:00:00Z","payload":{"hub_cache_only":true,"stream":"transport.playhead"}}`
	rec := httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(reqBody)))
	if rec.Code != http.StatusOK {
		t.Fatalf("frame_request status=%d body=%s", rec.Code, rec.Body.String())
	}
	var reply map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &reply); err != nil {
		t.Fatalf("decode frame reply: %v", err)
	}
	if reply["type"] != "realtime.frame" {
		t.Fatalf("unexpected frame reply: %#v", reply)
	}
	payload := reply["payload"].(map[string]any)
	if payload["stream"] != "transport.playhead" || payload["frame_index"].(float64) != 99 {
		t.Fatalf("unexpected frame payload: %#v", payload)
	}
}
