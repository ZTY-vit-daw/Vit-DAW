// Package runtimehost — SHELL-4 内核随窗就绪（用户裁定 2026-11-15：
// kernel-ready-at-shell-start）。
//
// 语义：VitShell 拉起的 agent 在启动时即拉起 headless 内核（VitApp.exe）
// 与 VspHub.exe 并完成连接——内核/装载不再挂在「打开工程」动作上。
//
// 所有权与门（锚定领取节结论）：
//   - 现链中内核/hub 由 Godot start_page 在直启动作时拉起、随 Godot 退出被
//     VitIpcClient 清理（关 DAW=内核死，手测 v6 18:35:17 证据）。本包把 spawn
//     所有权移交 agent：仅当 VitShell 显式委派（env VIT_AGENT_SPAWN_RUNTIME=1，
//     AgentManager 设置）时启用；旧链（Godot-first/dev_agent_smoke）不设该 env
//     → 零行为变化。
//   - 探测先行，不双拉：spawn 前先探测（内核 ZMQ ping / hub /health），已在线
//     即附着（agent 重启、外部已拉起等场景不出双内核）。Godot 直启侧的
//     「IPC 已就绪→跳过自启」是既有行为（start_page.gd:844-847），内核先行
//     在线时自然附着——Godot 零改动、装载协议零变更。
//   - 生命周期：spawn 的 children 挂在 agent 进程树内（VitShell job
//     KILL_ON_JOB_CLOSE 收编）→ 内核仅随 VitShell 主窗退出而退出；关 DAW
//     （Godot 退出）后内核留存。agent 自身退出时兜底 kill 已 spawn 的 children
//     （防 agent 单独死亡时内核裸留）。
//   - 诚实失败：找不到二进制/spawn 失败/就绪超时只记日志不崩——启动台由
//     greeting/列表的内核不可达诚实呈现面承接（launcher_flow 三态）。
//
// 二进制候选与 Godot 侧同一 dev 布局（start_page.gd
// _dev_kernel_candidates_by_freshness / _dev_vsp_hub_candidates_by_freshness
// 镜像）：VIT_DAW_DEV_ROOT / VIT_DAW_ROOT env（VitShell AgentManager 设置），
// 缺省从 cwd 向上走 agent/go.mod + VitApp 启发（telemetry.go 同款）。
// 内核 Workspace 根解析=exe 目录回溯（VitPaths.h getProjectRootDirectory：
// cwd 回溯失败后按 exe 回溯），agent 链继承 agent cwd（非 VitApp 根）→ 与
// Godot 链解析到同一 <dev_root>/VitApp。
package runtimehost

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/logx"
)

// 委派 env（VitShell AgentManager 设置；旧链不设）。
const spawnRuntimeEnv = "VIT_AGENT_SPAWN_RUNTIME"

// 默认时限（冷启动内核 ZMQ 绑定晚于进程存在——audio/project init 之后）。
const (
	defaultKernelReadyTimeout = 90 * time.Second
	defaultHubReadyTimeout    = 20 * time.Second
	defaultProbeTimeout       = 1500 * time.Millisecond
	probeInterval             = 250 * time.Millisecond
)

// Config 监督器配置（Supervisor 构造后只读）。
type Config struct {
	// Enabled=false 时 Start 为 no-op（旧链零行为）。
	Enabled bool
	// DevRoot dev 根（env 解析或 cwd 启发）；空=只探测不 spawn。
	DevRoot string
	// KernelReqURL 内核 ZMQ REQ 端点（与 agent 主客户端同一端点）。
	KernelReqURL string
	// HubHealthURL VspHub /health 探测端点。
	HubHealthURL string
	Logger       *logx.Logger

	KernelReadyTimeout time.Duration
	HubReadyTimeout    time.Duration
	ProbeTimeout       time.Duration
}

func (c *Config) kernelReadyTimeout() time.Duration {
	if c.KernelReadyTimeout > 0 {
		return c.KernelReadyTimeout
	}
	return defaultKernelReadyTimeout
}

func (c *Config) hubReadyTimeout() time.Duration {
	if c.HubReadyTimeout > 0 {
		return c.HubReadyTimeout
	}
	return defaultHubReadyTimeout
}

func (c *Config) probeTimeout() time.Duration {
	if c.ProbeTimeout > 0 {
		return c.ProbeTimeout
	}
	return defaultProbeTimeout
}

// EnabledFromEnv 报告 VitShell 是否委派了 runtime spawn 所有权。
func EnabledFromEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(spawnRuntimeEnv))) {
	case "1", "true", "on", "yes":
		return true
	}
	return false
}

// defaultDevRoot 与 start_page.gd _DEFAULT_DEV_ROOT 同值（canonical 回退用）。
const defaultDevRoot = `D:/Vit_DAW`

// DevRootFromEnv 解析 dev 根：VIT_DAW_DEV_ROOT → VIT_DAW_ROOT → cwd 向上走
// （agent/go.mod + VitApp 目录同在=仓库根；telemetry.go 同款启发）。空=未解析。
func DevRootFromEnv() string {
	for _, key := range []string{"VIT_DAW_DEV_ROOT", "VIT_DAW_ROOT"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return filepath.Clean(strings.ReplaceAll(v, "\\", "/"))
		}
	}
	if root := climbFromCwd(); root != "" {
		return root
	}
	return ""
}

// climbFromCwd 从工作目录向上找仓库根（agent/go.mod 与 VitApp/ 同在）。
func climbFromCwd() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	dir = filepath.Clean(strings.ReplaceAll(dir, "\\", "/"))
	for {
		if fileExists(filepath.Join(dir, "agent", "go.mod")) && dirExists(filepath.Join(dir, "VitApp")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// KernelExeName / HubExeName 平台命名（PORT-B1：Mac 无 .exe）。
func KernelExeName() string {
	if isMac() {
		return "VitApp"
	}
	return "VitApp.exe"
}

func HubExeName() string {
	if isMac() {
		return "vsphub"
	}
	return "VspHub.exe"
}

// KernelCandidates 内核候选（freshness 降序；root 为空返回 nil）——
// start_page.gd _dev_kernel_candidates_by_freshness 镜像（Release 优先）。
func KernelCandidates(root string) []string {
	if strings.TrimSpace(root) == "" {
		return nil
	}
	roots := []string{strings.TrimSpace(root)}
	if filepath.Clean(strings.ReplaceAll(defaultDevRoot, "\\", "/")) != filepath.Clean(strings.ReplaceAll(root, "\\", "/")) {
		roots = append(roots, defaultDevRoot)
	}
	name := KernelExeName()
	var out []string
	for _, r := range roots {
		r = strings.ReplaceAll(r, "\\", "/")
		if isMac() {
			// Mac cmake 布局：Release→Debug（start_page.gd:965-967 同序）。
			for _, config := range []string{"Release", "Debug"} {
				out = append(out,
					join(r, "VitApp", "build", "VitApp_artefacts", config, name),
					join(r, "VitApp", "build_release", "VitApp_artefacts", config, name))
			}
		} else {
			out = append(out,
				join(r, "VitApp", "build", "VitApp_artefacts", "Release", name),
				join(r, "VitApp", "build_release", "VitApp_artefacts", "Release", name))
		}
	}
	return sortByFreshness(out)
}

// HubCandidates hub 候选（freshness 降序；root 为空返回 nil）——
// start_page.gd _dev_vsp_hub_candidates_by_freshness 镜像（bin 优先）。
func HubCandidates(root string) []string {
	if strings.TrimSpace(root) == "" {
		return nil
	}
	roots := []string{strings.TrimSpace(root)}
	if filepath.Clean(strings.ReplaceAll(defaultDevRoot, "\\", "/")) != filepath.Clean(strings.ReplaceAll(root, "\\", "/")) {
		roots = append(roots, defaultDevRoot)
	}
	name := HubExeName()
	var out []string
	for _, r := range roots {
		r = strings.ReplaceAll(r, "\\", "/")
		if isMac() {
			out = append(out, join(r, "agent", "bin", name), join(r, "agent", name))
		} else {
			out = append(out, join(r, "agent", "bin", name), join(r, "agent", name))
		}
	}
	return sortByFreshness(out)
}

func join(parts ...string) string {
	return strings.Join(parts, "/")
}

// sortByFreshness mtime 降序，不存在排尾；同刻按字典序稳定（start_page.gd
// _compare_executable_candidate_freshness 同语义）。
func sortByFreshness(paths []string) []string {
	type entry struct {
		path  string
		mtime int64
		exist bool
	}
	entries := make([]entry, 0, len(paths))
	seen := map[string]bool{}
	for _, p := range paths {
		if seen[p] {
			continue
		}
		seen[p] = true
		info, err := os.Stat(p)
		e := entry{path: p}
		if err == nil && !info.IsDir() {
			e.exist = true
			e.mtime = info.ModTime().Unix()
		}
		entries = append(entries, e)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].exist != entries[j].exist {
			return entries[i].exist
		}
		if entries[i].mtime != entries[j].mtime {
			return entries[i].mtime > entries[j].mtime
		}
		return entries[i].path < entries[j].path
	})
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.path)
	}
	return out
}

// ── 探测面 ───────────────────────────────────────────────────────────────────

// ProbeKernel 内核探测：ZMQ REQ {"cmd":"ping"} → status ok / message pong。
func ProbeKernel(ctx context.Context, reqURL string, timeout time.Duration) error {
	client := kernel.New(reqURL, timeout)
	reply, _, err := client.SendCommand(ctx, map[string]any{"cmd": "ping"})
	if err != nil {
		return err
	}
	status := strings.ToLower(strings.TrimSpace(fmt.Sprint(reply["status"])))
	message := strings.ToLower(strings.TrimSpace(fmt.Sprint(reply["message"])))
	if status == "ok" || status == "success" || message == "pong" {
		return nil
	}
	return fmt.Errorf("kernel ping reply status=%s message=%s", status, message)
}

// ProbeHub hub 探测：GET /health → {"status":"ok","service":"VspHub"}。
func ProbeHub(ctx context.Context, healthURL string, timeout time.Duration) error {
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, healthURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("hub health http %d", resp.StatusCode)
	}
	var body struct {
		Status  string `json:"status"`
		Service string `json:"service"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&body); err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(body.Status), "ok") && strings.TrimSpace(body.Service) == "VspHub" {
		return nil
	}
	return fmt.Errorf("hub health reply status=%s service=%s", body.Status, body.Service)
}

// ── Supervisor ───────────────────────────────────────────────────────────────

// 内核引导状态（Supervisor.state；atomic 读写）。
const (
	kernelStateBooting int32 = 0
	kernelStateReady   int32 = 1
	kernelStateGaveUp  int32 = 2
)

// Supervisor 管理本 agent 会话内 spawn 的内核/hub children（Start 幂等，
// Shutdown 只 kill 自己 spawn 的进程——探测附着的外部进程不动）。
type Supervisor struct {
	cfg     Config
	mu      sync.Mutex
	procs   []*os.Process
	started bool

	readyOnce sync.Once
	ready     chan struct{}
	// state：0=引导中，1=内核就绪，2=已放弃（无候选/spawn 失败/就绪超时）。
	// WaitKernelReady 对 2 快速失败——调用方（greeting/列表）立即走内核
	// 不可达诚实呈现，不为已知无望的引导空等。
	state atomic.Int32
}

// New 构造监督器（cfg.Enabled=false 时 Start/Shutdown 均 no-op）。
func New(cfg Config) *Supervisor {
	if strings.TrimSpace(cfg.KernelReqURL) == "" {
		cfg.KernelReqURL = "tcp://127.0.0.1:5555"
	}
	if strings.TrimSpace(cfg.HubHealthURL) == "" {
		cfg.HubHealthURL = "http://127.0.0.1:8787/health"
	}
	return &Supervisor{cfg: cfg, ready: make(chan struct{})}
}

// Start 启动就绪保障循环（goroutine）：hub/内核探测先行，缺则 spawn 并等
// 就绪（bounded）；内核就绪即关闭 ready 通道。幂等（二次调用 no-op）。
func (s *Supervisor) Start(ctx context.Context) {
	if s == nil || !s.cfg.Enabled {
		return
	}
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.mu.Unlock()

	go s.run(ctx)
}

func (s *Supervisor) run(ctx context.Context) {
	log := s.cfg.Logger
	if log != nil {
		log.Info("runtimehost: enabled dev_root=%s kernel=%s hub_health=%s", s.cfg.DevRoot, s.cfg.KernelReqURL, s.cfg.HubHealthURL)
	}
	// hub 先保障（快）：缺失 spawn、bounded 等 /health。
	s.ensureHub(ctx)
	// 内核后保障（慢启动：ZMQ 绑定晚于 audio/project init）。
	s.ensureKernel(ctx)
}

func (s *Supervisor) ensureHub(ctx context.Context) {
	log := s.cfg.Logger
	if err := ProbeHub(ctx, s.cfg.HubHealthURL, s.cfg.probeTimeout()); err == nil {
		if log != nil {
			log.Info("runtimehost: VspHub already online (attach, no spawn)")
		}
		return
	} else if log != nil {
		log.Info("runtimehost: VspHub not reachable (%v) -> spawn path", err)
	}
	exe := firstExisting(HubCandidates(s.cfg.DevRoot))
	if exe == "" {
		if log != nil {
			log.Warn("runtimehost: no VspHub candidate under dev_root=%s (probe-only mode; launcher will present kernel-unavailable honestly)", s.cfg.DevRoot)
		}
		return
	}
	if pid := s.spawnChild(exe, "hub"); pid <= 0 {
		return
	}
	deadline := time.Now().Add(s.cfg.hubReadyTimeout())
	for {
		if err := ProbeHub(ctx, s.cfg.HubHealthURL, s.cfg.probeTimeout()); err == nil {
			if log != nil {
				log.Info("runtimehost: VspHub ready (%s)", exe)
			}
			return
		}
		if ctx.Err() != nil || time.Now().After(deadline) {
			if log != nil {
				log.Warn("runtimehost: VspHub not ready after %s (%s)", s.cfg.hubReadyTimeout(), exe)
			}
			return
		}
		time.Sleep(probeInterval)
	}
}

func (s *Supervisor) ensureKernel(ctx context.Context) {
	log := s.cfg.Logger
	if err := ProbeKernel(ctx, s.cfg.KernelReqURL, s.cfg.probeTimeout()); err == nil {
		if log != nil {
			log.Info("runtimehost: kernel already online (attach, no spawn)")
		}
		s.markKernelReady()
		return
	} else if log != nil {
		log.Info("runtimehost: kernel not reachable (%v) -> spawn path", err)
	}
	exe := firstExisting(KernelCandidates(s.cfg.DevRoot))
	if exe == "" {
		if log != nil {
			log.Warn("runtimehost: no VitApp candidate under dev_root=%s (probe-only mode; launcher will present kernel-unavailable honestly)", s.cfg.DevRoot)
		}
		s.markKernelGaveUp()
		return
	}
	if pid := s.spawnChild(exe, "kernel"); pid <= 0 {
		s.markKernelGaveUp()
		return
	}
	deadline := time.Now().Add(s.cfg.kernelReadyTimeout())
	for {
		if err := ProbeKernel(ctx, s.cfg.KernelReqURL, s.cfg.probeTimeout()); err == nil {
			if log != nil {
				log.Info("runtimehost: kernel ready (%s)", exe)
			}
			s.markKernelReady()
			return
		}
		if ctx.Err() != nil || time.Now().After(deadline) {
			if log != nil {
				log.Warn("runtimehost: kernel not ready after %s (%s); launcher will present kernel-unavailable honestly", s.cfg.kernelReadyTimeout(), exe)
			}
			s.markKernelGaveUp()
			return
		}
		time.Sleep(probeInterval)
	}
}

func (s *Supervisor) markKernelReady() {
	s.state.Store(kernelStateReady)
	s.readyOnce.Do(func() { close(s.ready) })
}

func (s *Supervisor) markKernelGaveUp() {
	s.state.CompareAndSwap(kernelStateBooting, kernelStateGaveUp)
}

// WaitKernelReady 有界等待内核就绪（greeting/列表的启动窗口桥接用）：
// 已就绪立即 true；已放弃（无候选/失败/超时）立即 false（诚实快速失败）；
// 引导中等到就绪/放弃/超时/ctx 取消。
func (s *Supervisor) WaitKernelReady(ctx context.Context, timeout time.Duration) bool {
	if s == nil {
		return false
	}
	switch s.state.Load() {
	case kernelStateReady:
		return true
	case kernelStateGaveUp:
		return false
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-s.ready:
		return true
	case <-ctx.Done():
		return false
	case <-timer.C:
		return false
	}
}

// Shutdown kill 自己 spawn 的 children（兜底；VitShell job 收编是主保障）。
func (s *Supervisor) Shutdown() {
	if s == nil {
		return
	}
	s.mu.Lock()
	procs := append([]*os.Process(nil), s.procs...)
	s.procs = nil
	s.mu.Unlock()
	for _, p := range procs {
		if p == nil {
			continue
		}
		if err := p.Kill(); err != nil && s.cfg.Logger != nil {
			s.cfg.Logger.Warn("runtimehost: child kill: %v", err)
		}
	}
}

// spawnChild 启动子进程（CREATE_NO_WINDOW：headless 链无窗；cwd 不设=继承
// agent cwd，内核 Workspace 根按 exe 回溯解析，与 Godot 链同一结果）。
func (s *Supervisor) spawnChild(exe, label string) int {
	log := s.cfg.Logger
	cmd := execCommand(exe)
	if err := cmd.Start(); err != nil {
		if log != nil {
			log.Error("runtimehost: spawn %s failed (%s): %v", label, exe, err)
		}
		return 0
	}
	s.mu.Lock()
	s.procs = append(s.procs, cmd.Process)
	s.mu.Unlock()
	if log != nil {
		log.Info("runtimehost: spawned %s pid=%d (%s)", label, cmd.Process.Pid, exe)
	}
	return cmd.Process.Pid
}

func firstExisting(candidates []string) string {
	for _, c := range candidates {
		if fileExists(c) {
			return c
		}
	}
	return ""
}

func isMac() bool {
	return runtime.GOOS == "darwin"
}
