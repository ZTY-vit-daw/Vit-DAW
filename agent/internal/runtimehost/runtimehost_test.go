package runtimehost

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 委派门：仅 VitShell 显式设置 env 时启用（旧链零行为）。
func TestEnabledFromEnv(t *testing.T) {
	cases := []struct {
		value string
		want  bool
	}{
		{"", false},
		{"0", false},
		{"no", false},
		{"1", true},
		{"true", true},
		{"ON", true},
		{"Yes", true},
	}
	for _, c := range cases {
		t.Setenv(spawnRuntimeEnv, c.value)
		if got := EnabledFromEnv(); got != c.want {
			t.Fatalf("EnabledFromEnv(%q)=%v want %v", c.value, got, c.want)
		}
	}
}

// 候选布局镜像 start_page.gd：dev 根相对路径 + freshness 降序 + 存在优先；
// build 优先于 build_release；canonical 回退只在 root 不同时追加。
func TestKernelAndHubCandidates(t *testing.T) {
	root := t.TempDir()
	build := filepath.Join(root, "VitApp", "build", "VitApp_artefacts", "Release", KernelExeName())
	buildRelease := filepath.Join(root, "VitApp", "build_release", "VitApp_artefacts", "Release", KernelExeName())
	hubBin := filepath.Join(root, "agent", "bin", HubExeName())
	for _, p := range []string{build, buildRelease, hubBin} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	kc := KernelCandidates(root)
	fwd := func(p string) string { return strings.ReplaceAll(p, "\\", "/") }
	if len(kc) < 2 || kc[0] != fwd(build) || kc[1] != fwd(buildRelease) {
		t.Fatalf("kernel candidates order: %v", kc)
	}
	hc := HubCandidates(root)
	if len(hc) < 1 || firstExisting(hc) != fwd(hubBin) {
		t.Fatalf("hub candidates: %v", hc)
	}
	if KernelCandidates("") != nil || HubCandidates("") != nil {
		t.Fatal("empty root must yield no candidates (probe-only)")
	}
}

// 排序语义：存在者优先于不存在；新 mtime 优先；同刻按字典序稳定。
func TestSortByFreshness(t *testing.T) {
	dir := t.TempDir()
	older := filepath.Join(dir, "older.exe")
	newer := filepath.Join(dir, "newer.exe")
	missing := filepath.Join(dir, "missing.exe")
	for _, p := range []string{older, newer} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(older, past, past); err != nil {
		t.Fatal(err)
	}
	got := sortByFreshness([]string{missing, older, newer})
	if len(got) != 3 || got[0] != newer || got[1] != older || got[2] != missing {
		t.Fatalf("freshness order: %v", got)
	}
}

// hub 探测：status ok + service VspHub 才算在线；其余诚实报错。
func TestProbeHub(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","service":"VspHub"}`))
	}))
	defer ok.Close()
	if err := ProbeHub(context.Background(), ok.URL, time.Second); err != nil {
		t.Fatalf("healthy hub probe: %v", err)
	}

	wrongService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","service":"other"}`))
	}))
	defer wrongService.Close()
	if err := ProbeHub(context.Background(), wrongService.URL, time.Second); err == nil {
		t.Fatal("wrong service must fail probe")
	}

	if err := ProbeHub(context.Background(), "http://127.0.0.1:1/health", 500*time.Millisecond); err == nil {
		t.Fatal("dead endpoint must fail probe")
	}
}

// 内核探测：无内核监听的端点必须失败（探测先行语义的前提——探测不通过
// 才允许 spawn；不能把死端点误判为在线）。
func TestProbeKernelDeadEndpointFails(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := ProbeKernel(ctx, "tcp://127.0.0.1:1", 800*time.Millisecond); err == nil {
		t.Fatal("dead ZMQ endpoint must fail kernel probe")
	}
}

// Supervisor 门与等待语义：未启用 no-op；gave-up 后 WaitKernelReady 立即
// false（诚实快速失败，不空等）；ready 后立即 true。
func TestSupervisorGatingAndWaitSemantics(t *testing.T) {
	s := New(Config{Enabled: false})
	s.Start(context.Background()) // no-op，不得 spawn 任何东西
	if len(s.procs) != 0 {
		t.Fatal("disabled supervisor must not spawn")
	}
	if s.WaitKernelReady(context.Background(), 50*time.Millisecond) {
		t.Fatal("disabled supervisor wait must be false")
	}

	on := New(Config{Enabled: true})
	on.markKernelGaveUp()
	started := time.Now()
	if on.WaitKernelReady(context.Background(), 5*time.Second) {
		t.Fatal("gave-up wait must be false")
	}
	if time.Since(started) > time.Second {
		t.Fatal("gave-up wait must fail fast (no busy waiting)")
	}

	ready := New(Config{Enabled: true})
	ready.markKernelReady()
	if !ready.WaitKernelReady(context.Background(), 50*time.Millisecond) {
		t.Fatal("ready wait must be true immediately")
	}
}

// WaitKernelReady 引导中超时：返回 false（调用方走诚实降级）。
func TestSupervisorWaitBootTimeout(t *testing.T) {
	s := New(Config{Enabled: true})
	if s.WaitKernelReady(context.Background(), 30*time.Millisecond) {
		t.Fatal("booting state must time out to false")
	}
}
