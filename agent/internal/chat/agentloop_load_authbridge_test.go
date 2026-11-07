package chat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/planner"
	"vit-daw-agent/internal/processorattestation"
)

// ARRLOOP-AUTHBRIDGE-1: the capability-wheel executor bridge produces the
// user-in-the-loop confirmation phase for processor loads (确认卡→用户确认→
// 盖章) with the same Authorize* stamping semantics as the load workflow
// confirm phase. The paired directions pinned here: an unconfirmed load never
// reaches the Kernel (it parks as needs_confirmation), and a confirmed load
// passes the unchanged PCA gate via the server-minted stamp.

type authBridgeFakeKernel struct {
	commands []map[string]any
}

func (f *authBridgeFakeKernel) SendCommand(_ context.Context, cmd map[string]any) (map[string]any, string, error) {
	f.commands = append(f.commands, cmd)
	if strings.EqualFold(strings.TrimSpace(firstStringFromMap(cmd, "cmd")), "rack_add_node") {
		return map[string]any{"status": "ok", "plugin_id": "bridge-node-1"}, "", nil
	}
	return map[string]any{"status": "ok"}, "", nil
}

func (f *authBridgeFakeKernel) rackAdds() []map[string]any {
	out := make([]map[string]any, 0, len(f.commands))
	for _, cmd := range f.commands {
		if strings.EqualFold(strings.TrimSpace(firstStringFromMap(cmd, "cmd")), "rack_add_node") {
			out = append(out, cmd)
		}
	}
	return out
}

// newAuthBridgeFixture 在隔离临时目录内造一件已晋升 PCA 准入的插件（含真实
// 二进制文件，供装载门指纹校验），stores 经 t.Setenv 定向到临时路径。
func newAuthBridgeFixture(t *testing.T, identifier string) (path string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_PATH", filepath.Join(root, "pca-v1.json"))
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_V2_PATH", filepath.Join(root, "pca-v2.json"))
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", filepath.Join(root, "missing_plugin_semantics.json"))

	bundle := filepath.Join(root, "AuthBridge Piano.vst3")
	if err := os.MkdirAll(filepath.Join(bundle, "Contents", "x86_64-win"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "Contents", "x86_64-win", "AuthBridge Piano.vst3"), []byte("authbridge-binary-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	subject := processorattestation.Subject{Name: "AuthBridge Piano", Manufacturer: "Vendor", Format: "VST3", Identifier: identifier, InstalledPath: bundle}
	fingerprint, err := processorattestation.FingerprintPath(bundle)
	if err != nil {
		t.Fatal(err)
	}
	store, err := processorattestation.NewStore("")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.PromoteCurrent(processorattestation.IssueSpec{
		Subject: subject, BinaryFingerprint: fingerprint, ProcessorFamily: processorattestation.FamilyStaticEQ,
		Coverage: []processorattestation.Coverage{{Action: "upsert", Shape: "bell"}},
		Evidence: []processorattestation.EvidenceRef{{ReceiptID: "authbridge-test", Kind: "test", SHA256: "sha256:" + strings.Repeat("a", 64), ObservedAt: time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)}},
	}, "authbridge-test"); err != nil {
		t.Fatal(err)
	}
	return bundle
}

func authBridgeServerAndExecutor(kernel *authBridgeFakeKernel) (*Server, pluginGrabberWorkflowExecutor) {
	server := &Server{harness: harness.NewWithSender(kernel, nil, nil)}
	return server, pluginGrabberWorkflowExecutor{server: server, base: executorpkg.New(server.harness)}
}

func authBridgeLoadInput(identifier, pluginPath string, confirmed bool) executorpkg.Input {
	return executorpkg.Input{
		GoalID: "goal-authbridge",
		ToolCall: planner.ToolCall{
			ID:   "radd-1",
			Tool: "rack.add_node",
			Args: map[string]any{
				"track_id":          "1032",
				"plugin_path":       pluginPath,
				"plugin_identifier": identifier,
				"zone_id":           "Z2",
			},
		},
		Context:   map[string]any{"user_message": "给这条 MIDI 轨加一个钢琴音源"},
		Confirmed: confirmed,
	}
}

// 方向一（未确认→仍拦）：未确认装载不派发、不执行，停泊确认卡（用户在环），
// 确认卡指名精确身份。
func TestAgentLoopLoadBridgeParksUnconfirmedLoadForUserConfirmation(t *testing.T) {
	const identifier = "VST3-AuthBridge-Chat-1"
	path := newAuthBridgeFixture(t, identifier)
	kernel := &authBridgeFakeKernel{}
	_, executor := authBridgeServerAndExecutor(kernel)

	result, err := executor.RunToolCall(context.Background(), authBridgeLoadInput(identifier, path, false))
	if err != nil {
		t.Fatalf("unconfirmed load should pause, not fail: %v", err)
	}
	if result.Status != "needs_confirmation" || !result.RequiresConfirmation {
		t.Fatalf("unconfirmed load must park as needs_confirmation, got %+v", result)
	}
	if result.Result["mutation_performed"] != false {
		t.Fatalf("paused load must not mutate: %+v", result.Result)
	}
	if !strings.Contains(result.Preview, identifier) || !strings.Contains(result.Preview, "1032") {
		t.Fatalf("confirmation card must name the exact identity: %q", result.Preview)
	}
	if adds := kernel.rackAdds(); len(adds) != 0 {
		t.Fatalf("unconfirmed load reached Kernel: %+v", adds)
	}
}

// 方向二（确认→盖章→过门→executed）：用户确认后的恢复执行经服务端盖章
// （AuthorizeConfirmedAgentLoopLoad 同族 Authorize* 语义）通过零改动 PCA 装
// 载门派发到内核；模型路径分隔符变体被钉到已晋升安装路径。
func TestAgentLoopLoadBridgeStampedConfirmedLoadPassesGate(t *testing.T) {
	const identifier = "VST3-AuthBridge-Chat-2"
	path := newAuthBridgeFixture(t, identifier)
	kernel := &authBridgeFakeKernel{}
	_, executor := authBridgeServerAndExecutor(kernel)

	result, err := executor.RunToolCall(context.Background(), authBridgeLoadInput(identifier, filepath.ToSlash(path), true))
	if err != nil || result.Status != "ok" {
		t.Fatalf("confirmed stamped load should pass the gate, result=%+v err=%v", result, err)
	}
	adds := kernel.rackAdds()
	if len(adds) != 1 {
		t.Fatalf("confirmed load did not dispatch exactly once: %+v", kernel.commands)
	}
	if got := strings.TrimSpace(firstStringFromMap(adds[0], "plugin_path")); got != path {
		t.Fatalf("dispatched plugin_path = %q, want the admitted path %q", got, path)
	}
	if got := strings.TrimSpace(firstStringFromMap(adds[0], "plugin_identifier")); got != identifier {
		t.Fatalf("dispatched plugin_identifier = %q, want %q", got, identifier)
	}
	if got := strings.TrimSpace(firstStringFromMap(adds[0], "track_id")); got != "1032" {
		t.Fatalf("dispatched track_id = %q, want 1032", got)
	}
}

// fail-closed：无可晋升准入的身份，确认卡不出、确认后也不放行——两个方向
// 都如实报错，内核零派发。
func TestAgentLoopLoadBridgeFailsClosedWithoutPromotedAdmission(t *testing.T) {
	const identifier = "VST3-AuthBridge-Chat-Unadmitted"
	path := newAuthBridgeFixture(t, "VST3-AuthBridge-Chat-Other")
	for _, confirmed := range []bool{false, true} {
		kernel := &authBridgeFakeKernel{}
		_, executor := authBridgeServerAndExecutor(kernel)
		result, err := executor.RunToolCall(context.Background(), authBridgeLoadInput(identifier, path, confirmed))
		if err != nil || result.Status != "error" || !strings.Contains(result.Error, "装载授权解析失败") || !strings.Contains(result.Error, "no current promoted admission") {
			t.Fatalf("unadmitted selection must fail closed (confirmed=%t): result=%+v err=%v", confirmed, result, err)
		}
		if result.Result["mutation_performed"] != false {
			t.Fatalf("failed authorization must not mutate: %+v", result.Result)
		}
		if adds := kernel.rackAdds(); len(adds) != 0 {
			t.Fatalf("unadmitted load reached Kernel (confirmed=%t): %+v", confirmed, adds)
		}
	}
}

// full access 面不受桥影响：显式 full-project-access 语境下桥不拦不停不盖，
// 装载门自身的 full-access 回落照旧解析派发（分支序=semanticTopologyLoad
// Authorization）。
func TestAgentLoopLoadBridgeLeavesExplicitFullAccessPathUnchanged(t *testing.T) {
	const identifier = "VST3-AuthBridge-Chat-3"
	path := newAuthBridgeFixture(t, identifier)
	kernel := &authBridgeFakeKernel{}
	_, executor := authBridgeServerAndExecutor(kernel)

	in := authBridgeLoadInput(identifier, path, false)
	in.Context["authority_mode"] = "full_project_access"
	in.Context["authority_mode_explicit"] = true
	result, err := executor.RunToolCall(context.Background(), in)
	if err != nil || result.Status != "ok" {
		t.Fatalf("full-access load must keep its existing gate-fallback path, result=%+v err=%v", result, err)
	}
	if adds := kernel.rackAdds(); len(adds) != 1 {
		t.Fatalf("full-access admitted load did not dispatch: %+v", kernel.commands)
	}
}

// 形态边界：非装载工具、身份不全的装载、free-state 语境不走桥（既有链路
// 原样）。
func TestAgentLoopLoadBridgeLeavesNonManagedShapesToExistingPaths(t *testing.T) {
	const identifier = "VST3-AuthBridge-Chat-4"
	path := newAuthBridgeFixture(t, identifier)
	kernel := &authBridgeFakeKernel{}
	_, executor := authBridgeServerAndExecutor(kernel)

	// 身份不全（无 identifier）：交回 harness 既有校验如实拒绝，桥不代答。
	incomplete := authBridgeLoadInput(identifier, path, false)
	delete(incomplete.ToolCall.Args, "plugin_identifier")
	result, _ := executor.RunToolCall(context.Background(), incomplete)
	if result.Status != "error" || !strings.Contains(result.Error, "pca_load_gate") {
		t.Fatalf("incomplete load must fail via existing validators, result=%+v", result)
	}
	if adds := kernel.rackAdds(); len(adds) != 0 {
		t.Fatalf("incomplete load reached Kernel: %+v", adds)
	}

	// 非装载工具（只读 semantic search）不经桥。
	kernel2 := &authBridgeFakeKernel{}
	_, executor2 := authBridgeServerAndExecutor(kernel2)
	readOnly := executorpkg.Input{
		ToolCall: planner.ToolCall{ID: "search-1", Tool: "plugin.semantic_search", Args: map[string]any{"query": "piano", "type": "instrument"}},
		Context:  map[string]any{"user_message": "找一个钢琴音源"},
	}
	if _, err := executor2.RunToolCall(context.Background(), readOnly); err != nil {
		t.Fatalf("read-only search should not be bridged: %v", err)
	}
}
