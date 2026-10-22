package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeHub simulates the VSP hub HTTP surface with a controllable outage: while
// down is set, requests hang past the publisher's 2s client timeout.
type fakeHub struct {
	mu        sync.Mutex
	down      atomic.Bool
	hellos    int
	publishes []string
	session   int
}

func (f *fakeHub) handler(w http.ResponseWriter, r *http.Request) {
	if f.down.Load() {
		time.Sleep(4 * time.Second)
		http.Error(w, "hub down", http.StatusServiceUnavailable)
		return
	}
	var env map[string]any
	if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
		http.Error(w, "bad envelope", http.StatusBadRequest)
		return
	}
	messageType := fmt.Sprint(env["type"])
	f.mu.Lock()
	switch messageType {
	case "session.hello":
		f.hellos++
		f.session++
		fmt.Fprintf(w, `{"vsp_version":"1.0","channel":"session","type":"session.hello_ack","session_id":"sess_fake_%d","payload":{"status":"ok"}}`, f.session)
	case "realtime.publish":
		payload, _ := env["payload"].(map[string]any)
		marker := ""
		if frames, ok := payload["frames"].([]any); ok && len(frames) > 0 {
			if frame, ok := frames[0].(map[string]any); ok {
				marker = fmt.Sprint(frame["marker"])
			}
		}
		f.publishes = append(f.publishes, marker)
		fmt.Fprint(w, `{"vsp_version":"1.0","channel":"realtime","type":"realtime.publish_ack","session_id":"`+fmt.Sprint(env["session_id"])+`","payload":{"status":"ok","accepted_frames":1}}`)
	default:
		http.Error(w, "unsupported", http.StatusBadRequest)
	}
	f.mu.Unlock()
}

func (f *fakeHub) publishedMarkers() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.publishes...)
}

func (f *fakeHub) helloCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hellos
}

// A frame that fails to publish during a hub outage is retained and delivered
// after recovery (zero loss inside the retry budget), and the publisher
// re-registers with a fresh session once its session was cleared by failure.
func TestPublisherRetainsFrameAcrossHubOutageAndReRegisters(t *testing.T) {
	hub := &fakeHub{}
	server := httptest.NewServer(http.HandlerFunc(hub.handler))
	defer server.Close()

	b := New(Config{FileReplyDir: t.TempDir(), VSPHubURL: server.URL + "/vsp"}, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.runVSPRealtimeChannelPublisher(ctx, b.realtimePublishCh, vspRealtimeClientID, vspRealtimeClientName)

	sendFrame := func(marker string) {
		b.realtimePublishCh <- map[string]any{
			"frames": []map[string]any{{"stream": "transport.playhead", "marker": marker}},
		}
	}

	// Pre-outage publish succeeds and establishes a session.
	sendFrame("m0")
	waitFor(t, 3*time.Second, func() bool {
		markers := hub.publishedMarkers()
		return len(markers) == 1 && markers[0] == "m0"
	})
	hellosBefore := hub.helloCount()
	if hellosBefore != 1 {
		t.Fatalf("expected one hello before outage, got %d", hellosBefore)
	}

	// Outage: hub hangs past the 2s client timeout; the frame must be
	// retained, not dropped.
	hub.down.Store(true)
	sendFrame("m1")
	time.Sleep(3 * time.Second)
	if markers := hub.publishedMarkers(); len(markers) != 1 {
		t.Fatalf("unexpected publishes during outage: %v", markers)
	}

	// Recovery: the retained frame must arrive without a new frame being sent.
	hub.down.Store(false)
	waitFor(t, 10*time.Second, func() bool {
		markers := hub.publishedMarkers()
		return len(markers) == 2 && markers[1] == "m1"
	})

	// Failure cleared the session, so recovery required a fresh hello.
	if hellos := hub.helloCount(); hellos < hellosBefore+1 {
		t.Fatalf("publisher did not re-register after failure: hellos=%d want >=%d", hellos, hellosBefore+1)
	}

	// Post-recovery frames flow immediately.
	sendFrame("m2")
	waitFor(t, 3*time.Second, func() bool {
		markers := hub.publishedMarkers()
		return len(markers) == 3 && markers[2] == "m2"
	})
}

// A fresher frame replaces the retained one while retrying (latest-only
// semantics for live frames).
func TestPublisherLatestOnlyDuringOutage(t *testing.T) {
	hub := &fakeHub{}
	server := httptest.NewServer(http.HandlerFunc(hub.handler))
	defer server.Close()

	b := New(Config{FileReplyDir: t.TempDir(), VSPHubURL: server.URL + "/vsp"}, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.runVSPRealtimeChannelPublisher(ctx, b.realtimePublishCh, vspTransportClientID, vspTransportClientName)

	hub.down.Store(true)
	b.realtimePublishCh <- map[string]any{
		"frames": []map[string]any{{"stream": "transport.playhead", "marker": "old"}},
	}
	time.Sleep(2500 * time.Millisecond) // first attempt fails; backoff armed
	b.realtimePublishCh <- map[string]any{
		"frames": []map[string]any{{"stream": "transport.playhead", "marker": "new"}},
	}
	time.Sleep(500 * time.Millisecond) // replaces retained frame before retry
	hub.down.Store(false)

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		markers := hub.publishedMarkers()
		for _, marker := range markers {
			if marker == "old" {
				t.Fatalf("stale frame published after replacement: %v", markers)
			}
		}
		if len(markers) > 0 && markers[len(markers)-1] == "new" {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("fresh frame never published after recovery: %v", hub.publishedMarkers())
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("condition not met within %v", timeout)
}
