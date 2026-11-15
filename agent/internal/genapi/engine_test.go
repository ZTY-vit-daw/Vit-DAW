package genapi

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// GEN-YUE-WIRE-1 item 6: minimal engine supervision. External engines are
// reused untouched; self-started processes come up on demand and are the
// only ones the supervisor stops.

// TestEngineHelperProcess is not a real test: it is the fake engine child.
// When GO_ENGINE_PORT is set it serves a minimal engine HTTP surface on that
// port (delayed by GO_ENGINE_DELAY_MS), then hangs until killed — a stand-in
// for the real yue-server.exe lifecycle.
func TestEngineHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	if d := os.Getenv("GO_ENGINE_DELAY_MS"); d != "" {
		ms, _ := strconv.Atoi(d)
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
	if port := os.Getenv("GO_ENGINE_PORT"); port != "" {
		mux := http.NewServeMux()
		mux.HandleFunc("/job", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"status":"running"}`))
		})
		go func() { _ = http.ListenAndServe("127.0.0.1:"+port, mux) }()
	}
	for {
		time.Sleep(time.Hour)
	}
}

// TestEngineHelperProcessExit is the early-exit fake engine child.
func TestEngineHelperProcessExit(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	os.Exit(3)
}

// freeLocalPort reserves a port and releases it (the helper child binds it
// shortly after).
func freeLocalPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

func TestProbeYueEngine(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound) // any HTTP answer counts as up
	}))
	defer srv.Close()
	if !ProbeYueEngine(t.Context(), srv.URL, 0) {
		t.Fatal("running engine must probe true")
	}
	if ProbeYueEngine(t.Context(), "http://127.0.0.1:1", 0) {
		t.Fatal("refused port must probe false")
	}
}

func TestEnsureYueEngineExternalReuse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	cfg := ProviderConfig{ID: "yue-probe-external", Dialect: "yue_local", BaseURL: srv.URL}
	h, err := EnsureYueEngine(t.Context(), cfg)
	if err != nil {
		t.Fatalf("external engine must be reused: %v", err)
	}
	if !h.External || h.PID != 0 {
		t.Fatalf("expected external handle: %#v", h)
	}
	// Stopping an external engine is a no-op.
	stopped, err := EngineStop(cfg.ID)
	if err != nil || stopped {
		t.Fatalf("external engine must not be stoppable: stopped=%v err=%v", stopped, err)
	}
}

func TestEnsureYueEngineNoConfig(t *testing.T) {
	cfg := ProviderConfig{ID: "yue-probe-noconf", Dialect: "yue_local", BaseURL: "http://127.0.0.1:1"}
	_, err := EnsureYueEngine(t.Context(), cfg)
	if err == nil {
		t.Fatal("unreachable engine without engine config must fail")
	}
	ge := AsError(err)
	if ge.Class != ClassConfigError {
		t.Fatalf("expected config_error, got %s: %v", ge.Class, err)
	}
}

func TestEnsureYueEngineStartsAndStops(t *testing.T) {
	port := freeLocalPort(t)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	cfg := ProviderConfig{
		ID:      "yue-probe-start",
		Dialect: "yue_local",
		BaseURL: baseURL,
		Engine: &EngineConfig{
			Command:             os.Args[0],
			Args:                []string{"-test.run=TestEngineHelperProcess", "--"},
			Env:                 map[string]string{"GO_WANT_HELPER_PROCESS": "1", "GO_ENGINE_PORT": fmt.Sprint(port), "GO_ENGINE_DELAY_MS": "400"},
			ReadyTimeoutSeconds: 20,
			PollIntervalSeconds: 0.2,
		},
	}
	h, err := EnsureYueEngine(t.Context(), cfg)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if h.External || h.PID == 0 {
		t.Fatalf("expected self-owned handle: %#v", h)
	}
	// The child really serves the engine surface.
	if !ProbeYueEngine(t.Context(), baseURL, 0) {
		t.Fatal("self-started engine surface must be reachable")
	}
	// Status reports the owned pid.
	st := EngineStatus(cfg.ID, baseURL)
	if st["owned_pid"] != h.PID {
		t.Fatalf("status must report owned pid: %#v", st)
	}
	stopped, serr := EngineStop(cfg.ID)
	if serr != nil || !stopped {
		t.Fatalf("self-started engine must stop: stopped=%v err=%v", stopped, serr)
	}
	// The helper child (and its listener) is really gone.
	deadline := time.Now().Add(5 * time.Second)
	for ProbeYueEngine(t.Context(), baseURL, 0) && time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
	}
	if ProbeYueEngine(t.Context(), baseURL, 0) {
		t.Fatal("engine surface must be down after stop")
	}
	st2 := EngineStatus(cfg.ID, baseURL)
	if st2["owned_pid"] != 0 {
		t.Fatalf("stop must clear ownership: %#v", st2)
	}
}

func TestEnsureYueEngineEarlyExit(t *testing.T) {
	cfg := ProviderConfig{
		ID:      "yue-probe-earlyexit",
		Dialect: "yue_local",
		BaseURL: "http://127.0.0.1:1",
		Engine: &EngineConfig{
			Command:             os.Args[0],
			Args:                []string{"-test.run=TestEngineHelperProcessExit", "--"},
			Env:                 map[string]string{"GO_WANT_HELPER_PROCESS": "1"},
			ReadyTimeoutSeconds: 10,
			PollIntervalSeconds: 0.2,
		},
	}
	_, err := EnsureYueEngine(t.Context(), cfg)
	if err == nil {
		t.Fatal("early-exiting engine must fail")
	}
	if !strings.Contains(err.Error(), "exited before becoming ready") && !strings.Contains(err.Error(), "exited early") {
		t.Fatalf("expected early-exit classification, got: %v", err)
	}
}
