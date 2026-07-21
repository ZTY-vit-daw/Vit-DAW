package vsphub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type recordingKernel struct {
	mu      sync.Mutex
	raw     []string
	replies []string
	err     error
}

func (k *recordingKernel) SendRaw(_ context.Context, payload string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.raw = append(k.raw, payload)
	if k.err != nil {
		return "", k.err
	}
	if len(k.replies) == 0 {
		return `{"vsp_version":"1.0","channel":"session","type":"session.heartbeat","status":"ok"}`, nil
	}
	reply := k.replies[0]
	k.replies = k.replies[1:]
	return reply, nil
}

func (k *recordingKernel) Calls() []string {
	k.mu.Lock()
	defer k.mu.Unlock()
	return append([]string(nil), k.raw...)
}

func testEnvelopeBody(t *testing.T, sessionID, clientID, role, channel, messageType string, payload map[string]any) string {
	t.Helper()
	if payload == nil {
		payload = map[string]any{}
	}
	body := map[string]any{
		"vsp_version": VSPVersion,
		"schema":      "vsp." + messageType + ".v1",
		"message_id":  "msg_test_" + strings.ReplaceAll(messageType, ".", "_"),
		"session_id":  sessionID,
		"client_id":   clientID,
		"role":        role,
		"channel":     channel,
		"type":        messageType,
		"created_at":  "2026-07-03T00:00:00Z",
		"trace_id":    "trace_test",
		"payload":     payload,
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal test envelope: %v", err)
	}
	return string(data)
}

func decodeMap(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode JSON body: %v body=%s", err, string(body))
	}
	return out
}

func listContainsText(value any, want string) bool {
	rows, ok := value.([]any)
	if !ok {
		return false
	}
	for _, row := range rows {
		text, ok := row.(string)
		if ok && strings.TrimSpace(text) == want {
			return true
		}
	}
	return false
}

func TestHTTPVSPForwardsRawEnvelopeAndRegistersSession(t *testing.T) {
	kernel := &recordingKernel{replies: []string{
		`{"vsp_version":"1.0","channel":"session","type":"session.hello_ack","session_id":"sess_kernel_http","feature_flags":{"state.delta":true},"payload":{"status":"ok"}}`,
	}}
	hub := New(Config{HTTPAddr: "127.0.0.1:0"}, kernel, nil)
	body := testEnvelopeBody(t, "session_pending", "godot.gui.test", "gui", "session", "session.hello", map[string]any{
		"client_name":        "GUI Test",
		"client_version":     "test",
		"wants":              []string{"state.snapshot"},
		"transport_bindings": []string{"vsp.hub.http"},
	})

	req := httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(body))
	rec := httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Vit-VSP-Transport"); got != TransportHTTP {
		t.Fatalf("transport header = %q", got)
	}
	calls := kernel.Calls()
	if len(calls) != 1 || calls[0] != body {
		t.Fatalf("kernel calls = %#v", calls)
	}
	var reply map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &reply); err != nil {
		t.Fatalf("decode reply: %v body=%s", err, rec.Body.String())
	}
	if reply["type"] != "session.hello_ack" || reply["session_id"] != "sess_kernel_http" {
		t.Fatalf("unexpected hello reply: %#v", reply)
	}
	if hubInfo := mapFromAny(reply["hub"]); hubInfo["transport_binding"] != TransportHTTP {
		t.Fatalf("missing hub info: %#v", reply["hub"])
	}
	if hub.sessions.Count() != 1 {
		t.Fatalf("session count = %d", hub.sessions.Count())
	}
}

func TestHealthAndStatusExposeDiagnosticsContract(t *testing.T) {
	kernel := &recordingKernel{}
	hub := New(Config{
		HTTPAddr:        "127.0.0.1:9876",
		KernelReqURL:    "tcp://127.0.0.1:6001",
		KernelSubURL:    "tcp://127.0.0.1:6002",
		EnableTelemetry: true,
		HubName:         "Test VSP Hub",
		HubVersion:      "test-hub-version",
	}, kernel, nil)

	rec := httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("health status = %d body=%s", rec.Code, rec.Body.String())
	}
	health := decodeMap(t, rec.Body.Bytes())
	if health["status"] != "ok" || health["service"] != "VspHub" || health["diagnostics_version"] != diagnosticsVersion {
		t.Fatalf("unexpected health body: %#v", health)
	}
	if health["hub_name"] != "Test VSP Hub" || health["hub_version"] != "test-hub-version" {
		t.Fatalf("missing hub identity in health: %#v", health)
	}
	if pid, ok := health["pid"].(float64); !ok || pid <= 0 {
		t.Fatalf("missing pid in health: %#v", health["pid"])
	}
	if !listContainsText(health["transports"], TransportHTTP) || !listContainsText(health["transports"], TransportWebSocket) {
		t.Fatalf("missing transports in health: %#v", health["transports"])
	}
	kernelDiag := mapFromAny(health["kernel"])
	if kernelDiag["status"] != "configured" || kernelDiag["req_url"] != "tcp://127.0.0.1:6001" || kernelDiag["telemetry_enabled"] != true {
		t.Fatalf("unexpected kernel diagnostics: %#v", kernelDiag)
	}

	register := testEnvelopeBody(t, "sess_diag_ext", "third.party.diag", "extension", "extension", "extension.register", map[string]any{
		"name": "Diagnostics Extension",
	})
	rec = httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(register)))
	if rec.Code != http.StatusOK {
		t.Fatalf("register status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/vsp/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d body=%s", rec.Code, rec.Body.String())
	}
	status := decodeMap(t, rec.Body.Bytes())
	if status["diagnostics_version"] != diagnosticsVersion || status["kernel_req"] != "tcp://127.0.0.1:6001" {
		t.Fatalf("unexpected status body: %#v", status)
	}
	if count, ok := status["session_count"].(float64); !ok || count != 1 {
		t.Fatalf("session_count = %#v", status["session_count"])
	}
	bindings, ok := status["transport_bindings"].([]any)
	if !ok || len(bindings) != 2 {
		t.Fatalf("transport_bindings = %#v", status["transport_bindings"])
	}
	httpBinding := mapFromAny(bindings[0])
	if httpBinding["name"] != TransportHTTP || httpBinding["endpoint"] != "http://127.0.0.1:9876/vsp" {
		t.Fatalf("unexpected HTTP binding diagnostics: %#v", httpBinding)
	}
	capabilities := mapFromAny(status["capabilities"])
	if !listContainsText(capabilities["extension"], "state.snapshot") || listContainsText(capabilities["extension"], "command.request") {
		t.Fatalf("unexpected extension capabilities: %#v", capabilities["extension"])
	}
	sessions, ok := status["sessions"].([]any)
	if !ok || len(sessions) != 1 {
		t.Fatalf("sessions = %#v", status["sessions"])
	}
	session := mapFromAny(sessions[0])
	if session["session_id"] != "sess_diag_ext" || session["client_id"] != "third.party.diag" || session["role"] != "extension" || session["transport"] != TransportHTTP {
		t.Fatalf("unexpected diagnostic session: %#v", session)
	}
	if !listContainsText(session["capabilities"], "event.poll") {
		t.Fatalf("session capabilities missing event.poll: %#v", session["capabilities"])
	}
}

func TestExtensionRegisterIsHandledLocally(t *testing.T) {
	kernel := &recordingKernel{}
	hub := New(Config{HTTPAddr: "127.0.0.1:0"}, kernel, nil)
	body := testEnvelopeBody(t, "sess_ext", "third.party.ext", "extension", "extension", "extension.register", map[string]any{
		"name":    "Third Party",
		"version": "0.1.0",
	})

	req := httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(body))
	rec := httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if calls := kernel.Calls(); len(calls) != 0 {
		t.Fatalf("extension.register should not hit kernel: %#v", calls)
	}
	var reply map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &reply); err != nil {
		t.Fatalf("decode reply: %v", err)
	}
	if reply["type"] != "extension.register_ack" {
		t.Fatalf("unexpected reply: %#v", reply)
	}
	session, ok := hub.sessions.Get("sess_ext")
	if !ok {
		t.Fatalf("extension session was not registered")
	}
	if session.ClientName != "Third Party" || session.ClientVersion != "0.1.0" {
		t.Fatalf("unexpected extension session: %#v", session)
	}
}

func TestExtensionRegisterRequiresExtensionRole(t *testing.T) {
	kernel := &recordingKernel{}
	hub := New(Config{HTTPAddr: "127.0.0.1:0"}, kernel, nil)
	body := testEnvelopeBody(t, "sess_gui", "godot.gui", "gui", "extension", "extension.register", map[string]any{
		"name": "Not Extension",
	})

	req := httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(body))
	rec := httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if calls := kernel.Calls(); len(calls) != 0 {
		t.Fatalf("denied extension.register should not hit kernel: %#v", calls)
	}
}

func TestExtensionUnregisterIsHandledLocally(t *testing.T) {
	kernel := &recordingKernel{}
	hub := New(Config{HTTPAddr: "127.0.0.1:0"}, kernel, nil)
	register := testEnvelopeBody(t, "sess_ext", "third.party.ext", "extension", "extension", "extension.register", map[string]any{
		"name": "Third Party",
	})
	unregister := testEnvelopeBody(t, "sess_ext", "third.party.ext", "extension", "extension", "extension.unregister", map[string]any{})

	rec := httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(register)))
	if rec.Code != http.StatusOK {
		t.Fatalf("register status = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(unregister)))

	if rec.Code != http.StatusOK {
		t.Fatalf("unregister status = %d body=%s", rec.Code, rec.Body.String())
	}
	if _, ok := hub.sessions.Get("sess_ext"); ok {
		t.Fatalf("extension session was not removed")
	}
	if calls := kernel.Calls(); len(calls) != 0 {
		t.Fatalf("extension unregister should not hit kernel: %#v", calls)
	}
	var reply map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &reply); err != nil {
		t.Fatalf("decode reply: %v", err)
	}
	if reply["type"] != "extension.unregister_ack" {
		t.Fatalf("unexpected reply: %#v", reply)
	}
}

func TestExtensionCommandIsDeniedByPermissionGate(t *testing.T) {
	kernel := &recordingKernel{}
	hub := New(Config{HTTPAddr: "127.0.0.1:0"}, kernel, nil)
	body := testEnvelopeBody(t, "sess_ext", "third.party.ext", "extension", "command", "command.request", map[string]any{
		"command": "transport.play",
	})

	req := httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(body))
	rec := httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if calls := kernel.Calls(); len(calls) != 0 {
		t.Fatalf("denied extension command should not hit kernel: %#v", calls)
	}
	reply := decodeMap(t, rec.Body.Bytes())
	if reply["type"] != "command.error" {
		t.Fatalf("permission denial should be a command.error envelope: %#v", reply)
	}
	if errObj := mapFromAny(reply["error"]); errObj["code"] != "permission_denied" {
		t.Fatalf("unexpected permission error envelope: %#v", reply)
	}
}

func TestSessionRoleOverridesEnvelopeRoleForPermissions(t *testing.T) {
	kernel := &recordingKernel{}
	hub := New(Config{HTTPAddr: "127.0.0.1:0"}, kernel, nil)
	register := testEnvelopeBody(t, "sess_ext", "third.party.ext", "extension", "extension", "extension.register", map[string]any{
		"name": "Third Party",
	})
	rec := httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(register)))
	if rec.Code != http.StatusOK {
		t.Fatalf("register status = %d body=%s", rec.Code, rec.Body.String())
	}

	spoofedCommand := testEnvelopeBody(t, "sess_ext", "third.party.ext", "gui", "command", "command.request", map[string]any{
		"command": "transport.play",
	})
	rec = httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(spoofedCommand)))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("spoofed command status = %d body=%s", rec.Code, rec.Body.String())
	}
	if calls := kernel.Calls(); len(calls) != 0 {
		t.Fatalf("role-spoofed extension command should not hit kernel: %#v", calls)
	}
	reply := decodeMap(t, rec.Body.Bytes())
	if errObj := mapFromAny(reply["error"]); errObj["code"] != "permission_denied" {
		t.Fatalf("unexpected role override error: %#v", reply)
	}
}

func TestExtensionReadOnlyCapabilitiesAreForwarded(t *testing.T) {
	kernel := &recordingKernel{replies: []string{
		`{"vsp_version":"1.0","channel":"state","type":"state.snapshot","session_id":"sess_ext","payload":{"status":"ok","scope":"project.timeline"}}`,
		`{"vsp_version":"1.0","channel":"event","type":"event.notification","session_id":"sess_ext","payload":{"status":"ok","events":[]}}`,
	}}
	hub := New(Config{HTTPAddr: "127.0.0.1:0"}, kernel, nil)
	register := testEnvelopeBody(t, "sess_ext", "third.party.ext", "extension", "extension", "extension.register", map[string]any{
		"name": "Third Party",
	})
	rec := httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(register)))
	if rec.Code != http.StatusOK {
		t.Fatalf("register status = %d body=%s", rec.Code, rec.Body.String())
	}

	snapshot := testEnvelopeBody(t, "sess_ext", "third.party.ext", "extension", "state", "state.snapshot_request", map[string]any{
		"scope": "project.timeline",
	})
	rec = httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(snapshot)))
	if rec.Code != http.StatusOK {
		t.Fatalf("state.snapshot status = %d body=%s", rec.Code, rec.Body.String())
	}
	if reply := decodeMap(t, rec.Body.Bytes()); reply["type"] != "state.snapshot" {
		t.Fatalf("unexpected snapshot reply: %#v", reply)
	}

	poll := testEnvelopeBody(t, "sess_ext", "third.party.ext", "extension", "event", "event.poll", map[string]any{
		"topics": []string{"project"},
	})
	rec = httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(poll)))
	if rec.Code != http.StatusOK {
		t.Fatalf("event.poll status = %d body=%s", rec.Code, rec.Body.String())
	}
	if reply := decodeMap(t, rec.Body.Bytes()); reply["type"] != "event.notification" {
		t.Fatalf("unexpected event poll reply: %#v", reply)
	}
	if calls := kernel.Calls(); len(calls) != 2 {
		t.Fatalf("allowed read-only extension calls should hit kernel twice: %#v", calls)
	}
}

func TestAssetMaterializeRequestIsServedFromHubCache(t *testing.T) {
	kernel := &recordingKernel{}
	hub := New(Config{HTTPAddr: "127.0.0.1:0"}, kernel, nil)
	hub.assets.storeTile(AudioFeatureTile{
		ClipID:           "1011",
		KernelTrackID:    "1007",
		Kind:             "waveform_peak",
		FeatureType:      "waveform_envelope",
		TileIndex:        0,
		TileStartSeconds: 0,
		TileDuration:     5,
		Data:             []float32{0.1, -0.1, 0.2, -0.2},
		Metadata: map[string]any{
			"feature_type": "waveform_envelope",
			"tile_index":   0,
		},
	})
	body := testEnvelopeBody(t, "sess_gui", "godot.gui.test", "gui", "asset", "asset.materialize_request", map[string]any{
		"clip_id":         "1011",
		"kernel_track_id": "1007",
		"kind":            "waveform_peak",
		"feature_type":    "waveform_envelope",
	})

	rec := httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	reply := decodeMap(t, rec.Body.Bytes())
	if reply["type"] != "asset.materialized" {
		t.Fatalf("type = %#v", reply["type"])
	}
	payload := mapFromAny(reply["payload"])
	if payload["status"] != "ok" || payload["cached"] != true {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	if int(payload["tile_count"].(float64)) != 1 || int(payload["float_count"].(float64)) != 4 {
		t.Fatalf("unexpected tile/float counts: %#v", payload)
	}
	if calls := kernel.Calls(); len(calls) != 0 {
		t.Fatalf("materialize request should not hit kernel: %#v", calls)
	}
}

func TestRealtimePublishIsHandledLocally(t *testing.T) {
	kernel := &recordingKernel{}
	hub := New(Config{HTTPAddr: "127.0.0.1:0"}, kernel, nil)
	body := testEnvelopeBody(t, "sess_kernel", "vit.kernel", "kernel", "realtime", "realtime.publish", map[string]any{
		"frame": map[string]any{
			"stream":      "transport.playhead",
			"frame_index": 7,
			"data": map[string]any{
				"position_seconds": 12.5,
				"playing":          true,
			},
		},
	})

	rec := httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("publish status = %d body=%s", rec.Code, rec.Body.String())
	}
	if calls := kernel.Calls(); len(calls) != 0 {
		t.Fatalf("realtime.publish should not hit kernel: %#v", calls)
	}
	reply := decodeMap(t, rec.Body.Bytes())
	if reply["type"] != "realtime.publish_ack" {
		t.Fatalf("unexpected publish reply: %#v", reply)
	}
	payload := mapFromAny(reply["payload"])
	if payload["delivery"] != "hub_cache" || payload["accepted_frames"].(float64) != 1 {
		t.Fatalf("unexpected publish payload: %#v", payload)
	}
	if snapshot := hub.realtime.Snapshot(); snapshot["published_frames"].(int64) != 1 {
		t.Fatalf("unexpected realtime snapshot: %#v", snapshot)
	}
}

func TestRealtimeCacheOnlySubscribeAndFrameRequestStayLocal(t *testing.T) {
	kernel := &recordingKernel{}
	hub := New(Config{HTTPAddr: "127.0.0.1:0"}, kernel, nil)
	subscribe := testEnvelopeBody(t, "sess_gui", "godot.gui", "gui", "realtime", "realtime.subscribe", map[string]any{
		"hub_cache_only": true,
		"streams": []map[string]any{
			{
				"stream":    "meters.visible_tracks",
				"track_ids": []string{"1007", "1008"},
				"max_hz":    60,
			},
		},
	})

	rec := httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(subscribe)))
	if rec.Code != http.StatusOK {
		t.Fatalf("subscribe status = %d body=%s", rec.Code, rec.Body.String())
	}
	reply := decodeMap(t, rec.Body.Bytes())
	if reply["type"] != "realtime.stream_status" {
		t.Fatalf("unexpected subscribe reply: %#v", reply)
	}
	payload := mapFromAny(reply["payload"])
	if payload["delivery"] != "hub_cache" {
		t.Fatalf("unexpected subscribe delivery: %#v", payload)
	}
	subID := payload["subscription_id"].(string)
	streams := payload["streams"].([]any)
	stream := mapFromAny(streams[0])
	streamID := stream["stream_id"].(string)

	publish := testEnvelopeBody(t, "sess_kernel", "vit.kernel", "kernel", "realtime", "realtime.publish", map[string]any{
		"frame": map[string]any{
			"stream":      "meters.visible_tracks",
			"frame_index": 11,
			"data": map[string]any{
				"tracks": []map[string]any{
					{"track_id": "1007", "peak": -10.0},
					{"track_id": "1008", "peak": -14.0},
				},
			},
		},
	})
	rec = httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(publish)))
	if rec.Code != http.StatusOK {
		t.Fatalf("publish status = %d body=%s", rec.Code, rec.Body.String())
	}

	frameRequest := testEnvelopeBody(t, "sess_gui", "godot.gui", "gui", "realtime", "realtime.frame_request", map[string]any{
		"hub_cache_only":  true,
		"subscription_id": subID,
		"stream_id":       streamID,
	})
	rec = httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(frameRequest)))
	if rec.Code != http.StatusOK {
		t.Fatalf("frame_request status = %d body=%s", rec.Code, rec.Body.String())
	}
	reply = decodeMap(t, rec.Body.Bytes())
	if reply["type"] != "realtime.frame" {
		t.Fatalf("unexpected frame reply: %#v", reply)
	}
	frame := mapFromAny(reply["payload"])
	if frame["stream"] != "meters.visible_tracks" || frame["subscription_id"] != subID || frame["stream_id"] != streamID {
		t.Fatalf("unexpected cached frame payload: %#v", frame)
	}
	if calls := kernel.Calls(); len(calls) != 0 {
		t.Fatalf("cache-only realtime flow should not hit kernel: %#v", calls)
	}
}

func TestRealtimeCacheHitFiltersVisibleTracksFromPublishedFullFrame(t *testing.T) {
	kernel := &recordingKernel{}
	hub := New(Config{HTTPAddr: "127.0.0.1:0"}, kernel, nil)
	subscribe := testEnvelopeBody(t, "sess_gui", "godot.gui", "gui", "realtime", "realtime.subscribe", map[string]any{
		"hub_cache_only": true,
		"streams": []map[string]any{
			{
				"stream":    "meters.visible_tracks",
				"track_ids": []string{"1008", "1009"},
				"max_hz":    60,
			},
			{
				"stream":    "spectrum.visible_tracks",
				"track_ids": []string{"1008", "1009"},
				"max_hz":    12,
			},
		},
	})

	rec := httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(subscribe)))
	if rec.Code != http.StatusOK {
		t.Fatalf("subscribe status = %d body=%s", rec.Code, rec.Body.String())
	}
	reply := decodeMap(t, rec.Body.Bytes())
	payload := mapFromAny(reply["payload"])
	subID := payload["subscription_id"].(string)
	streamIDs := map[string]string{}
	for _, rawStream := range payload["streams"].([]any) {
		stream := mapFromAny(rawStream)
		streamIDs[fmt.Sprint(stream["stream"])] = fmt.Sprint(stream["stream_id"])
	}

	publish := testEnvelopeBody(t, "sess_kernel", "vit.kernel", "kernel", "realtime", "realtime.publish", map[string]any{
		"frames": []map[string]any{
			{
				"stream":      "meters.visible_tracks",
				"track_ids":   []string{"1007", "1008", "1009"},
				"frame_index": 31,
				"data": map[string]any{
					"tracks": []map[string]any{
						{"track_id": "1007", "peak_db": -30.0},
						{"track_id": "1008", "peak_db": -12.0},
						{"track_id": "1009", "peak_db": -9.0},
					},
					"visible_track_count": 3,
				},
			},
			{
				"stream":      "spectrum.visible_tracks",
				"track_ids":   []string{"1007", "1008", "1009"},
				"frame_index": 32,
				"data": map[string]any{
					"asset_refs": []map[string]any{
						{"track_id": "1007", "uri": "vit-cache://tracks/1007/spectrum/latest"},
						{"track_id": "1008", "uri": "vit-cache://tracks/1008/spectrum/latest"},
						{"track_id": "1009", "uri": "vit-cache://tracks/1009/spectrum/latest"},
					},
					"visible_track_count": 3,
				},
			},
		},
	})
	rec = httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(publish)))
	if rec.Code != http.StatusOK {
		t.Fatalf("publish status = %d body=%s", rec.Code, rec.Body.String())
	}

	meterRequest := testEnvelopeBody(t, "sess_gui", "godot.gui", "gui", "realtime", "realtime.frame_request", map[string]any{
		"hub_cache_only":  true,
		"subscription_id": subID,
		"stream_id":       streamIDs["meters.visible_tracks"],
	})
	rec = httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(meterRequest)))
	if rec.Code != http.StatusOK {
		t.Fatalf("meter frame_request status = %d body=%s", rec.Code, rec.Body.String())
	}
	frame := mapFromAny(decodeMap(t, rec.Body.Bytes())["payload"])
	data := mapFromAny(frame["data"])
	tracks := data["tracks"].([]any)
	if len(tracks) != 2 || fmt.Sprint(mapFromAny(tracks[0])["track_id"]) != "1008" || fmt.Sprint(mapFromAny(tracks[1])["track_id"]) != "1009" {
		t.Fatalf("meter tracks were not filtered in subscription order: %#v", tracks)
	}
	if data["visible_track_count"].(float64) != 2 {
		t.Fatalf("meter visible_track_count was not filtered: %#v", data)
	}

	spectrumRequest := testEnvelopeBody(t, "sess_gui", "godot.gui", "gui", "realtime", "realtime.frame_request", map[string]any{
		"hub_cache_only":  true,
		"subscription_id": subID,
		"stream_id":       streamIDs["spectrum.visible_tracks"],
	})
	rec = httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(spectrumRequest)))
	if rec.Code != http.StatusOK {
		t.Fatalf("spectrum frame_request status = %d body=%s", rec.Code, rec.Body.String())
	}
	frame = mapFromAny(decodeMap(t, rec.Body.Bytes())["payload"])
	data = mapFromAny(frame["data"])
	refs := data["asset_refs"].([]any)
	if len(refs) != 2 || fmt.Sprint(mapFromAny(refs[0])["track_id"]) != "1008" || fmt.Sprint(mapFromAny(refs[1])["track_id"]) != "1009" {
		t.Fatalf("spectrum refs were not filtered in subscription order: %#v", refs)
	}
	if data["visible_track_count"].(float64) != 2 {
		t.Fatalf("spectrum visible_track_count was not filtered: %#v", data)
	}
	if calls := kernel.Calls(); len(calls) != 0 {
		t.Fatalf("filtered cache-only realtime flow should not hit kernel: %#v", calls)
	}
}

func TestWebSocketRealtimeBroadcastFiltersBySessionSubscription(t *testing.T) {
	kernel := &recordingKernel{replies: []string{
		`{"vsp_version":"1.0","channel":"session","type":"session.hello_ack","session_id":"sess_gui_ws","feature_flags":{"realtime.visible_tracks":true},"payload":{"status":"ok"}}`,
	}}
	hub := New(Config{HTTPAddr: "127.0.0.1:0"}, kernel, nil)
	server := httptest.NewServer(hub.Routes())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/vsp/stream"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	hello := testEnvelopeBody(t, "session_pending", "godot.gui.ws", "gui", "session", "session.hello", map[string]any{
		"client_name":        "GUI WS",
		"client_version":     "test",
		"wants":              []string{"realtime.subscribe"},
		"transport_bindings": []string{"vsp.hub.websocket"},
	})
	if err := conn.WriteMessage(websocket.TextMessage, []byte(hello)); err != nil {
		t.Fatalf("write websocket hello: %v", err)
	}
	_, replyBody, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read websocket hello: %v", err)
	}
	if reply := decodeMap(t, replyBody); reply["type"] != "session.hello_ack" || reply["session_id"] != "sess_gui_ws" {
		t.Fatalf("unexpected websocket hello reply: %#v", reply)
	}

	subscribe := testEnvelopeBody(t, "sess_gui_ws", "godot.gui.ws", "gui", "realtime", "realtime.subscribe", map[string]any{
		"hub_cache_only": true,
		"streams": []map[string]any{
			{
				"stream":    "meters.visible_tracks",
				"track_ids": []string{"1008", "1009"},
				"max_hz":    60,
			},
		},
	})
	if err := conn.WriteMessage(websocket.TextMessage, []byte(subscribe)); err != nil {
		t.Fatalf("write websocket subscribe: %v", err)
	}
	_, replyBody, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("read websocket subscribe: %v", err)
	}
	subscribeReply := decodeMap(t, replyBody)
	if subscribeReply["type"] != "realtime.stream_status" {
		t.Fatalf("unexpected websocket subscribe reply: %#v", subscribeReply)
	}
	subPayload := mapFromAny(subscribeReply["payload"])
	subID := fmt.Sprint(subPayload["subscription_id"])
	streamRows := subPayload["streams"].([]any)
	stream := mapFromAny(streamRows[0])
	streamID := fmt.Sprint(stream["stream_id"])

	publish := testEnvelopeBody(t, "sess_kernel", "vit.kernel", "kernel", "realtime", "realtime.publish", map[string]any{
		"frame": map[string]any{
			"stream":      "meters.visible_tracks",
			"track_ids":   []string{"1007", "1008", "1009"},
			"frame_index": 41,
			"data": map[string]any{
				"tracks": []map[string]any{
					{"track_id": "1007", "peak_db": -30.0},
					{"track_id": "1008", "peak_db": -12.0},
					{"track_id": "1009", "peak_db": -9.0},
				},
				"visible_track_count": 3,
			},
		},
	})
	rec := httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(publish)))
	if rec.Code != http.StatusOK {
		t.Fatalf("publish status = %d body=%s", rec.Code, rec.Body.String())
	}

	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	_, replyBody, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("read websocket realtime frame: %v", err)
	}
	reply := decodeMap(t, replyBody)
	if reply["type"] != "realtime.frame" || reply["session_id"] != "sess_gui_ws" {
		t.Fatalf("unexpected websocket realtime reply: %#v", reply)
	}
	frame := mapFromAny(reply["payload"])
	if frame["subscription_id"] != subID || frame["stream_id"] != streamID || frame["delivery"] != "hub_websocket" {
		t.Fatalf("unexpected websocket frame routing fields: %#v", frame)
	}
	data := mapFromAny(frame["data"])
	tracks := data["tracks"].([]any)
	if len(tracks) != 2 || fmt.Sprint(mapFromAny(tracks[0])["track_id"]) != "1008" || fmt.Sprint(mapFromAny(tracks[1])["track_id"]) != "1009" {
		t.Fatalf("websocket tracks were not filtered in subscription order: %#v", tracks)
	}
	if data["visible_track_count"].(float64) != 2 {
		t.Fatalf("websocket visible_track_count was not filtered: %#v", data)
	}
	if calls := kernel.Calls(); len(calls) != 1 {
		t.Fatalf("only websocket hello should hit kernel: %#v", calls)
	}
}

func TestRealtimeBroadcastSkipsVisibleFramesWithoutSubscribedRows(t *testing.T) {
	store := NewRealtimeStore()
	subscribeBody := testEnvelopeBody(t, "sess_gui", "godot.gui", "gui", "realtime", "realtime.subscribe", map[string]any{
		"hub_cache_only": true,
		"streams": []map[string]any{
			{
				"stream":    "meters.visible_tracks",
				"track_ids": []string{"1008", "1009"},
				"max_hz":    60,
			},
		},
	})
	env, err := ParseEnvelopeBytes([]byte(subscribeBody))
	if err != nil {
		t.Fatalf("parse subscribe envelope: %v", err)
	}
	status := store.Subscribe(env, true)
	streams := status["streams"].([]map[string]any)
	streamID := fmt.Sprint(streams[0]["stream_id"])

	emptyDeliveries := store.FramesForSessionBroadcast("sess_gui", map[string]any{
		"stream":      "meters.visible_tracks",
		"track_ids":   []string{"1007"},
		"frame_index": 1,
		"data": map[string]any{
			"tracks": []map[string]any{
				{"track_id": "1007", "peak_db": -30.0},
			},
		},
	})
	if len(emptyDeliveries) != 0 {
		t.Fatalf("non-matching visible frame should not be delivered or throttle subscription: %#v", emptyDeliveries)
	}

	deliveries := store.FramesForSessionBroadcast("sess_gui", map[string]any{
		"stream":      "meters.visible_tracks",
		"track_ids":   []string{"1007", "1008", "1009"},
		"frame_index": 2,
		"data": map[string]any{
			"tracks": []map[string]any{
				{"track_id": "1007", "peak_db": -30.0},
				{"track_id": "1008", "peak_db": -12.0},
				{"track_id": "1009", "peak_db": -9.0},
			},
		},
	})
	if len(deliveries) != 1 {
		t.Fatalf("matching visible frame should be delivered once: %#v", deliveries)
	}
	payload := deliveries[0].Payload
	if payload["stream_id"] != streamID {
		t.Fatalf("delivery did not preserve stream_id: %#v", payload)
	}
	data := mapFromAny(payload["data"])
	tracks := data["tracks"].([]any)
	if len(tracks) != 2 || fmt.Sprint(mapFromAny(tracks[0])["track_id"]) != "1008" || fmt.Sprint(mapFromAny(tracks[1])["track_id"]) != "1009" {
		t.Fatalf("matching visible tracks were not filtered in subscription order: %#v", tracks)
	}
}

func TestTelemetryBroadcastRequiresEventSubscription(t *testing.T) {
	kernel := &recordingKernel{replies: []string{
		`{"vsp_version":"1.0","channel":"session","type":"session.hello_ack","session_id":"sess_without_events","payload":{"status":"ok"}}`,
		`{"vsp_version":"1.0","channel":"session","type":"session.hello_ack","session_id":"sess_with_events","payload":{"status":"ok"}}`,
		`{"vsp_version":"1.0","channel":"event","type":"event.notification","session_id":"sess_with_events","payload":{"status":"subscribed","subscription_id":"evt_sub","topics":["levels"]}}`,
	}}
	hub := New(Config{HTTPAddr: "127.0.0.1:0"}, kernel, nil)
	server := httptest.NewServer(hub.Routes())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/vsp/stream"
	withoutEvents, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket without events: %v", err)
	}
	defer withoutEvents.Close()
	withEvents, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket with events: %v", err)
	}
	defer withEvents.Close()

	sendWS := func(conn *websocket.Conn, body string) map[string]any {
		t.Helper()
		if err := conn.WriteMessage(websocket.TextMessage, []byte(body)); err != nil {
			t.Fatalf("write websocket: %v", err)
		}
		if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		_, replyBody, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read websocket reply: %v", err)
		}
		return decodeMap(t, replyBody)
	}

	helloWithout := testEnvelopeBody(t, "session_pending", "ws.without.events", "gui", "session", "session.hello", map[string]any{
		"client_name":        "Without Events",
		"transport_bindings": []string{"vsp.hub.websocket"},
	})
	if reply := sendWS(withoutEvents, helloWithout); reply["session_id"] != "sess_without_events" {
		t.Fatalf("unexpected hello without events reply: %#v", reply)
	}
	helloWith := testEnvelopeBody(t, "session_pending", "ws.with.events", "gui", "session", "session.hello", map[string]any{
		"client_name":        "With Events",
		"transport_bindings": []string{"vsp.hub.websocket"},
	})
	if reply := sendWS(withEvents, helloWith); reply["session_id"] != "sess_with_events" {
		t.Fatalf("unexpected hello with events reply: %#v", reply)
	}
	subscribe := testEnvelopeBody(t, "sess_with_events", "ws.with.events", "gui", "event", "event.subscribe", map[string]any{
		"topics": []string{"levels"},
	})
	if reply := sendWS(withEvents, subscribe); reply["type"] != "event.notification" {
		t.Fatalf("unexpected event subscribe reply: %#v", reply)
	}

	hub.broadcastTelemetry(`{"topic":"levels","peak_db":-9}`)

	if err := withEvents.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set withEvents read deadline: %v", err)
	}
	_, body, err := withEvents.ReadMessage()
	if err != nil {
		t.Fatalf("subscribed websocket did not receive telemetry event: %v", err)
	}
	if reply := decodeMap(t, body); reply["type"] != "event.notification" {
		t.Fatalf("unexpected telemetry event: %#v", reply)
	}
	if err := withoutEvents.SetReadDeadline(time.Now().Add(150 * time.Millisecond)); err != nil {
		t.Fatalf("set withoutEvents read deadline: %v", err)
	}
	if _, body, err := withoutEvents.ReadMessage(); err == nil {
		t.Fatalf("unsubscribed websocket received telemetry event: %s", string(body))
	}
}

func TestEventSubscribeReplaysCachedAudioFeatureTelemetry(t *testing.T) {
	kernel := &recordingKernel{replies: []string{
		`{"vsp_version":"1.0","channel":"session","type":"session.hello_ack","session_id":"sess_replay","payload":{"status":"ok"}}`,
		`{"vsp_version":"1.0","channel":"event","type":"event.notification","session_id":"sess_replay","payload":{"status":"subscribed","subscription_id":"evt_replay","topics":["audio_feature_data_ready"]}}`,
	}}
	hub := New(Config{HTTPAddr: "127.0.0.1:0"}, kernel, nil)
	server := httptest.NewServer(hub.Routes())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/vsp/stream"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	sendWS := func(body string) map[string]any {
		t.Helper()
		if err := conn.WriteMessage(websocket.TextMessage, []byte(body)); err != nil {
			t.Fatalf("write websocket: %v", err)
		}
		if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		_, replyBody, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read websocket reply: %v", err)
		}
		return decodeMap(t, replyBody)
	}

	hello := testEnvelopeBody(t, "session_pending", "ws.replay", "gui", "session", "session.hello", map[string]any{
		"client_name":        "Replay Client",
		"transport_bindings": []string{"vsp.hub.websocket"},
	})
	if reply := sendWS(hello); reply["session_id"] != "sess_replay" {
		t.Fatalf("unexpected hello reply: %#v", reply)
	}

	hub.broadcastTelemetry(`{"command":"audio_feature_data_ready","clip_id":"1011","track_id":"1007","shared_memory":"Vit_AudioFeature_waveform_1011_g1_0","float_count":3072}`)
	hub.broadcastTelemetry(`{"type":"delta_update","seq_id":9,"action":"node_added","target_uid":"track_1007"}`)

	subscribe := testEnvelopeBody(t, "sess_replay", "ws.replay", "gui", "event", "event.subscribe", map[string]any{
		"topics": []string{"audio_feature_data_ready", "delta_update"},
	})
	if reply := sendWS(subscribe); reply["type"] != "event.notification" {
		t.Fatalf("unexpected event subscribe reply: %#v", reply)
	}

	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set replay read deadline: %v", err)
	}
	_, body, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("cached telemetry replay was not delivered: %v", err)
	}
	replay := decodeMap(t, body)
	payload := mapFromAny(replay["payload"])
	if payload["replay"] != true {
		t.Fatalf("cached telemetry was not marked as replay: %#v", payload)
	}
	telemetry := mapFromAny(payload["telemetry"])
	if telemetry["command"] != "audio_feature_data_ready" || telemetry["clip_id"] != "1011" {
		t.Fatalf("unexpected replay telemetry: %#v", telemetry)
	}
	if err := conn.SetReadDeadline(time.Now().Add(150 * time.Millisecond)); err != nil {
		t.Fatalf("set extra replay read deadline: %v", err)
	}
	if _, body, err := conn.ReadMessage(); err == nil {
		t.Fatalf("delta_update should not be replay cached: %s", string(body))
	}
}

func TestRealtimeKernelSubscribeRegistersShadowForCacheHits(t *testing.T) {
	kernel := &recordingKernel{replies: []string{
		`{"vsp_version":"1.0","channel":"realtime","type":"realtime.stream_status","session_id":"sess_gui","payload":{"status":"subscribed","subscription_id":"sub_kernel","streams":[{"stream":"transport.playhead","stream_id":"stream_playhead","max_hz":30,"latest_only":true,"drop_old":true}]}}`,
	}}
	hub := New(Config{HTTPAddr: "127.0.0.1:0"}, kernel, nil)
	subscribe := testEnvelopeBody(t, "sess_gui", "godot.gui", "gui", "realtime", "realtime.subscribe", map[string]any{
		"streams": []map[string]any{
			{"stream": "transport.playhead", "max_hz": 30},
		},
	})

	rec := httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(subscribe)))
	if rec.Code != http.StatusOK {
		t.Fatalf("subscribe status = %d body=%s", rec.Code, rec.Body.String())
	}
	if calls := kernel.Calls(); len(calls) != 1 {
		t.Fatalf("non-cache-only subscribe should hit kernel once: %#v", calls)
	}

	publish := testEnvelopeBody(t, "sess_kernel", "vit.kernel", "kernel", "realtime", "realtime.publish", map[string]any{
		"frame": map[string]any{
			"stream":      "transport.playhead",
			"frame_index": 18,
			"data": map[string]any{
				"position_seconds": 33.25,
				"playing":          true,
			},
		},
	})
	rec = httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(publish)))
	if rec.Code != http.StatusOK {
		t.Fatalf("publish status = %d body=%s", rec.Code, rec.Body.String())
	}

	frameRequest := testEnvelopeBody(t, "sess_gui", "godot.gui", "gui", "realtime", "realtime.frame_request", map[string]any{
		"subscription_id": "sub_kernel",
		"stream_id":       "stream_playhead",
	})
	rec = httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(frameRequest)))
	if rec.Code != http.StatusOK {
		t.Fatalf("frame_request status = %d body=%s", rec.Code, rec.Body.String())
	}
	reply := decodeMap(t, rec.Body.Bytes())
	if reply["type"] != "realtime.frame" {
		t.Fatalf("unexpected cached frame reply: %#v", reply)
	}
	frame := mapFromAny(reply["payload"])
	if frame["stream"] != "transport.playhead" || frame["subscription_id"] != "sub_kernel" || frame["stream_id"] != "stream_playhead" {
		t.Fatalf("unexpected cached frame payload: %#v", frame)
	}
	if calls := kernel.Calls(); len(calls) != 1 {
		t.Fatalf("cached frame_request should not add kernel calls: %#v", calls)
	}
}

func TestRealtimeDiagnosticsExposeCacheState(t *testing.T) {
	kernel := &recordingKernel{}
	hub := New(Config{HTTPAddr: "127.0.0.1:0"}, kernel, nil)
	publish := testEnvelopeBody(t, "sess_kernel", "vit.kernel", "kernel", "realtime", "realtime.publish", map[string]any{
		"frame": map[string]any{
			"stream":      "transport.playhead",
			"frame_index": 1,
			"data":        map[string]any{"playing": false},
		},
	})
	rec := httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(publish)))
	if rec.Code != http.StatusOK {
		t.Fatalf("publish status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/vsp/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d body=%s", rec.Code, rec.Body.String())
	}
	status := decodeMap(t, rec.Body.Bytes())
	realtime := mapFromAny(status["realtime"])
	if realtime["cache_first_frame_request"] != true || realtime["published_frames"].(float64) != 1 {
		t.Fatalf("unexpected realtime diagnostics: %#v", realtime)
	}
	capabilities := mapFromAny(status["capabilities"])
	if !listContainsText(capabilities["kernel"], "realtime.publish") {
		t.Fatalf("kernel capabilities missing realtime.publish: %#v", capabilities["kernel"])
	}
}

func TestEnvelopeValidationRequiresVSPIdentityFields(t *testing.T) {
	required := []string{"vsp_version", "schema", "message_id", "session_id", "client_id", "role", "channel", "type", "created_at", "payload"}
	for _, field := range required {
		t.Run("missing_"+field, func(t *testing.T) {
			body := map[string]any{
				"vsp_version": VSPVersion,
				"schema":      "vsp.session.hello.v1",
				"message_id":  "msg_required",
				"session_id":  "session_pending",
				"client_id":   "agent.required",
				"role":        "agent",
				"channel":     "session",
				"type":        "session.hello",
				"created_at":  "2026-07-03T00:00:00Z",
				"payload":     map[string]any{},
			}
			delete(body, field)
			data, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("marshal validation fixture: %v", err)
			}
			_, err = ParseEnvelopeBytes(data)
			if err == nil || !strings.Contains(err.Error(), field) {
				t.Fatalf("expected missing %s validation error, got %v", field, err)
			}
		})
	}

	_, err := ParseEnvelopeBytes([]byte(`{"vsp_version":"2.0","schema":"vsp.session.hello.v1","message_id":"msg_bad_version","session_id":"session_pending","client_id":"agent.required","role":"agent","channel":"session","type":"session.hello","created_at":"2026-07-03T00:00:00Z","payload":{}}`))
	if err == nil || !strings.Contains(err.Error(), "unsupported VSP version") {
		t.Fatalf("expected unsupported version error, got %v", err)
	}
	_, err = ParseEnvelopeBytes([]byte(`{"vsp_version":"1.0","schema":"vsp.session.hello.v1","message_id":"msg_bad_payload","session_id":"session_pending","client_id":"agent.required","role":"agent","channel":"session","type":"session.hello","created_at":"2026-07-03T00:00:00Z","payload":[]}`))
	if err == nil || !strings.Contains(err.Error(), "payload must be a JSON object") {
		t.Fatalf("expected payload object error, got %v", err)
	}
}

func TestHTTPVSPRejectsInvalidRequestsWithContractErrors(t *testing.T) {
	kernel := &recordingKernel{}
	hub := New(Config{HTTPAddr: "127.0.0.1:0"}, kernel, nil)

	rec := httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/vsp", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /vsp status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader("{")))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Vit-VSP-Transport"); got != TransportHTTP {
		t.Fatalf("invalid JSON transport header = %q", got)
	}
	reply := decodeMap(t, rec.Body.Bytes())
	if reply["vsp_version"] != VSPVersion || reply["type"] != "session.close" {
		t.Fatalf("invalid JSON did not return VSP error envelope: %#v", reply)
	}
	if errObj := mapFromAny(reply["error"]); errObj["code"] != "validation_error" {
		t.Fatalf("invalid JSON error = %#v", reply)
	}

	missingClient := `{"vsp_version":"1.0","schema":"vsp.session.hello.v1","message_id":"msg_missing_client","session_id":"session_pending","role":"agent","channel":"session","type":"session.hello","created_at":"2026-07-03T00:00:00Z","payload":{}}`
	rec = httptest.NewRecorder()
	hub.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vsp", strings.NewReader(missingClient)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing client_id status = %d body=%s", rec.Code, rec.Body.String())
	}
	reply = decodeMap(t, rec.Body.Bytes())
	if errObj := mapFromAny(reply["error"]); errObj["code"] != "validation_error" || !strings.Contains(errObj["message"].(string), "client_id") {
		t.Fatalf("missing client_id error = %#v", reply)
	}
	if calls := kernel.Calls(); len(calls) != 0 {
		t.Fatalf("invalid requests should not hit kernel: %#v", calls)
	}
}

func TestWebSocketStreamDispatchesEnvelope(t *testing.T) {
	kernel := &recordingKernel{replies: []string{
		`{"vsp_version":"1.0","channel":"session","type":"session.hello_ack","session_id":"sess_ws","payload":{"status":"ok"}}`,
	}}
	hub := New(Config{HTTPAddr: "127.0.0.1:0"}, kernel, nil)
	server := httptest.NewServer(hub.Routes())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/vsp/stream"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	body := testEnvelopeBody(t, "session_pending", "agent.test", "agent", "session", "session.hello", map[string]any{
		"client_name":        "Agent Test",
		"transport_bindings": []string{"vsp.hub.websocket"},
	})
	if err := conn.WriteMessage(websocket.TextMessage, []byte(body)); err != nil {
		t.Fatalf("write websocket: %v", err)
	}
	_, replyBody, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read websocket: %v", err)
	}
	var reply map[string]any
	if err := json.Unmarshal(replyBody, &reply); err != nil {
		t.Fatalf("decode websocket reply: %v body=%s", err, string(replyBody))
	}
	if reply["type"] != "session.hello_ack" {
		t.Fatalf("unexpected websocket reply: %#v", reply)
	}
	if hubInfo := mapFromAny(reply["hub"]); hubInfo["transport_binding"] != TransportWebSocket {
		t.Fatalf("missing websocket hub info: %#v", reply["hub"])
	}
	if len(kernel.Calls()) != 1 {
		t.Fatalf("kernel calls = %#v", kernel.Calls())
	}
}

func TestWebSocketInvalidEnvelopeReturnsStructuredError(t *testing.T) {
	kernel := &recordingKernel{}
	hub := New(Config{HTTPAddr: "127.0.0.1:0"}, kernel, nil)
	server := httptest.NewServer(hub.Routes())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/vsp/stream"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"vsp_version":"1.0","channel":"session","type":"session.hello"}`)); err != nil {
		t.Fatalf("write invalid websocket envelope: %v", err)
	}
	_, replyBody, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read websocket validation error: %v", err)
	}
	reply := decodeMap(t, replyBody)
	if reply["vsp_version"] != VSPVersion || reply["type"] != "session.close" {
		t.Fatalf("websocket validation did not return VSP error envelope: %#v", reply)
	}
	if ack := mapFromAny(reply["ack"]); ack["stage"] != "rejected" {
		t.Fatalf("websocket validation ack = %#v", reply)
	}
	if errObj := mapFromAny(reply["error"]); errObj["code"] != "validation_error" {
		t.Fatalf("websocket validation error = %#v", reply)
	}
	if calls := kernel.Calls(); len(calls) != 0 {
		t.Fatalf("invalid websocket envelope should not hit kernel: %#v", calls)
	}
}
