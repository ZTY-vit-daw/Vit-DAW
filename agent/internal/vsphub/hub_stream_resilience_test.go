package vsphub

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// dialStream connects a websocket client and performs session.hello. The
// supplied kernel must answer the hello reply.
func dialStream(t *testing.T, wsURL, clientID string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket %s: %v", clientID, err)
	}
	t.Cleanup(func() { conn.Close() })
	hello := testEnvelopeBody(t, "session_pending", clientID, "gui", "session", "session.hello", map[string]any{
		"client_name":        clientID,
		"transport_bindings": []string{TransportWebSocket},
	})
	if err := conn.WriteMessage(websocket.TextMessage, []byte(hello)); err != nil {
		t.Fatalf("write hello %s: %v", clientID, err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf("read hello reply %s: %v", clientID, err)
	}
	return conn
}

func readJSONWithDeadline(t *testing.T, conn *websocket.Conn, timeout time.Duration) map[string]any {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	_, body, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read websocket message: %v", err)
	}
	return decodeMap(t, body)
}

// subscribeEvents performs a hub-local event.subscribe so telemetry
// broadcasts are delivered to this client.
func subscribeEvents(t *testing.T, conn *websocket.Conn, sessionID, clientID string) {
	t.Helper()
	body := testEnvelopeBody(t, sessionID, clientID, "gui", "event", "event.subscribe", map[string]any{
		"topics": []string{"levels"},
	})
	if err := conn.WriteMessage(websocket.TextMessage, []byte(body)); err != nil {
		t.Fatalf("write event.subscribe: %v", err)
	}
	// The kernel-registered reply arrives via ws; consume it.
	readJSONWithDeadline(t, conn, 2*time.Second)
}

// A client whose outbox overflows is evicted fail-fast and Write never
// blocks the shared callers (root cause R1/R2 contract).
func TestOutboxOverflowEvictsClientWithoutBlocking(t *testing.T) {
	origCap := streamQueueCapacity
	streamQueueCapacity = 4
	t.Cleanup(func() { streamQueueCapacity = origCap })

	kernel := &recordingKernel{replies: []string{
		`{"vsp_version":"1.0","channel":"session","type":"session.hello_ack","session_id":"sess_overflow","payload":{"status":"ok"}}`,
	}}
	hub := New(Config{HTTPAddr: "127.0.0.1:0"}, kernel, nil)
	server := httptest.NewServer(hub.Routes())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/vsp/stream"
	_ = dialStream(t, wsURL, "ws.overflow")

	stream := hub.streams.Clients()[0]

	start := time.Now()
	for i := 0; i < 50; i++ {
		stream.Write([]byte(fmt.Sprintf(`{"topic":"levels","seq":%d}`, i)))
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("stream.Write blocked behind full outbox: %v", elapsed)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if stream.SnapshotInfo()["evicted"].(bool) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("overflowing stream was not evicted: %v", stream.SnapshotInfo())
}

// A stalled client that stops reading must not starve telemetry delivery to
// a healthy client sharing the same relay (root cause R2 regression).
func TestStalledClientDoesNotStarveHealthyClient(t *testing.T) {
	kernel := &recordingKernel{replies: []string{
		`{"vsp_version":"1.0","channel":"session","type":"session.hello_ack","session_id":"sess_stalled","payload":{"status":"ok"}}`,
		`{"vsp_version":"1.0","channel":"session","type":"session.hello_ack","session_id":"sess_healthy","payload":{"status":"ok"}}`,
		`{"vsp_version":"1.0","channel":"event","type":"event.notification","session_id":"sess_stalled","payload":{"status":"subscribed","subscription_id":"evt_stalled","topics":["levels"]}}`,
		`{"vsp_version":"1.0","channel":"event","type":"event.notification","session_id":"sess_healthy","payload":{"status":"subscribed","subscription_id":"evt_healthy","topics":["levels"]}}`,
	}}
	hub := New(Config{HTTPAddr: "127.0.0.1:0"}, kernel, nil)
	server := httptest.NewServer(hub.Routes())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/vsp/stream"
	stalled := dialStream(t, wsURL, "ws.stalled")
	healthy := dialStream(t, wsURL, "ws.healthy")
	subscribeEvents(t, stalled, "sess_stalled", "ws.stalled")
	subscribeEvents(t, healthy, "sess_healthy", "ws.healthy")

	// The stalled client never reads from here on; the healthy client must
	// keep receiving every telemetry broadcast.
	for i := 0; i < 200; i++ {
		hub.broadcastTelemetry(fmt.Sprintf(`{"topic":"levels","seq":%d}`, i))
	}
	for i := 0; i < 50; i++ {
		event := readJSONWithDeadline(t, healthy, 2*time.Second)
		if fmt.Sprint(event["type"]) != "event.notification" {
			t.Fatalf("healthy client received unexpected frame: %#v", event)
		}
	}
}

// The realtime publish HTTP path must stay responsive even while a wedged
// subscribed websocket client never reads (root cause R3 regression: agent
// POST "context deadline exceeded").
func TestRealtimePublishHTTPNotBlockedByWedgedStream(t *testing.T) {
	kernel := &recordingKernel{replies: []string{
		`{"vsp_version":"1.0","channel":"session","type":"session.hello_ack","session_id":"sess_wedged","payload":{"status":"ok"}}`,
	}}
	hub := New(Config{HTTPAddr: "127.0.0.1:0"}, kernel, nil)
	server := httptest.NewServer(hub.Routes())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/vsp/stream"
	wedged := dialStream(t, wsURL, "ws.wedged")
	// Hub-local realtime subscribe binds the meters stream to the wedged
	// client exactly like the Godot adapter does.
	sub := testEnvelopeBody(t, "", "ws.wedged", "gui", "realtime", "realtime.subscribe", map[string]any{
		"streams": []map[string]any{{
			"stream":  "meters.visible_tracks",
			"max_hz":  30,
			"mode":    "latest_only",
		}},
		"hub_cache_only": true,
	})
	if err := wedged.WriteMessage(websocket.TextMessage, []byte(sub)); err != nil {
		t.Fatalf("write realtime.subscribe: %v", err)
	}
	readJSONWithDeadline(t, wedged, 2*time.Second)
	// wedged never reads again.

	bigFrames := make([]map[string]any, 64)
	for i := range bigFrames {
		bigFrames[i] = map[string]any{
			"stream":      "meters.visible_tracks",
			"frame_index": i + 1,
			"data":        map[string]any{"blob": strings.Repeat("x", 4096)},
		}
	}
	body, err := json.Marshal(map[string]any{
		"vsp_version": VSPVersion,
		"schema":      "vsp.realtime.publish.v1",
		"message_id":  "msg_test_publish",
		"session_id":  "sess_agent_pub",
		"client_id":   "vit.agent.realtime_publisher",
		"role":        "agent",
		"channel":     "realtime",
		"type":        "realtime.publish",
		"payload":     map[string]any{"frames": bigFrames},
	})
	if err != nil {
		t.Fatalf("marshal publish envelope: %v", err)
	}

	client := &http.Client{Timeout: 2 * time.Second}
	for i := 0; i < 20; i++ {
		start := time.Now()
		resp, err := client.Post(server.URL+"/vsp", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("publish POST %d failed after %v: %v", i, time.Since(start), err)
		}
		resp.Body.Close()
		if elapsed := time.Since(start); elapsed > 1500*time.Millisecond {
			t.Fatalf("publish POST %d blocked behind wedged stream: %v", i, elapsed)
		}
	}
}

// Idle but healthy clients receive the hub heartbeat so GUI watchdogs can
// tell "quiet" apart from a supply cut (root cause R5 fallback).
func TestHeartbeatReachesIdleClient(t *testing.T) {
	origPing := streamPingInterval
	streamPingInterval = 50 * time.Millisecond
	t.Cleanup(func() { streamPingInterval = origPing })

	kernel := &recordingKernel{replies: []string{
		`{"vsp_version":"1.0","channel":"session","type":"session.hello_ack","session_id":"sess_ping","payload":{"status":"ok"}}`,
	}}
	hub := New(Config{HTTPAddr: "127.0.0.1:0"}, kernel, nil)
	server := httptest.NewServer(hub.Routes())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/vsp/stream"
	conn := dialStream(t, wsURL, "ws.ping")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		event := readJSONWithDeadline(t, conn, 2*time.Second)
		if fmt.Sprint(event["type"]) == "hub.heartbeat" {
			return
		}
	}
	t.Fatalf("idle client never received hub.heartbeat within deadline")
}

// Evicting a stream unwedges the server read loop so the client is removed
// from the registry (cleanup wiring for fail-fast eviction).
func TestEvictedStreamIsRemovedFromRegistry(t *testing.T) {
	kernel := &recordingKernel{replies: []string{
		`{"vsp_version":"1.0","channel":"session","type":"session.hello_ack","session_id":"sess_evict","payload":{"status":"ok"}}`,
	}}
	hub := New(Config{HTTPAddr: "127.0.0.1:0"}, kernel, nil)
	server := httptest.NewServer(hub.Routes())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/vsp/stream"
	_ = dialStream(t, wsURL, "ws.evict")

	if got := hub.streams.Count(); got != 1 {
		t.Fatalf("stream count after connect = %d, want 1", got)
	}
	hub.streams.Clients()[0].Evict("test_evict")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if hub.streams.Count() == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("evicted stream still registered: %d", hub.streams.Count())
}

// The health surface exposes relay liveness and per-stream delivery state so
// the resilience injection leg can quantify the outage window.
func TestHealthExposesRelayAndStreamDiagnostics(t *testing.T) {
	kernel := &recordingKernel{replies: []string{}}
	hub := New(Config{HTTPAddr: "127.0.0.1:0"}, kernel, nil)

	health := hub.Health()
	if _, ok := health["relay"]; !ok {
		t.Fatalf("health missing relay diagnostics: %#v", health)
	}
	if _, ok := health["streams"]; !ok {
		t.Fatalf("health missing streams diagnostics: %#v", health)
	}
	hub.relay.recvCount.Add(3)
	hub.relay.lastRecvUnixMs.Store(time.Now().UTC().UnixMilli())
	relay := hub.relaySnapshot()
	if relay["recv_count"].(int64) != 3 {
		t.Fatalf("relay snapshot recv_count = %v, want 3", relay["recv_count"])
	}
	if _, ok := relay["last_recv_at"]; !ok {
		t.Fatalf("relay snapshot missing last_recv_at: %#v", relay)
	}
}
