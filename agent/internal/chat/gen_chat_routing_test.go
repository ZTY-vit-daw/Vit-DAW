package chat

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/orchestrationruntime"
	agentruntime "vit-daw-agent/internal/runtime"
)

// GEN-CHAT-ROUTING-1 paired tests, replaying the 278 run2/run3 defect shapes
// offline (negative anchors: coord/runs/GEN-YUE-DIALECT-1/run_20261107_193711
// and run_20261107_194419). Double-shape pin: on the pre-fix tree these tests
// FAIL (run2 message hijacked by the pan_layout canary; generation-flavored
// goal text gets a narrowed toolset without gen.generate); on the fixed tree
// they PASS.

// --- Defect 1: short capability codes embedded in structured text must not
// claim the turn (ABC notation / code / hex / JSON params), while explicit
// bounded codes and natural phrases keep routing.

func TestCapabilityShortCodeMarkersIgnoreStructuredText(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
	}{
		{"abc score tail with b32 (run2 shape)", "V: Ins\ne'8c'8a6g6e'4-|e'4c'8a8g12|b32|Z|"},
		{"abc bar containing a4b4", `"A7"g2a2z16g4a4c'4-|"Em7"b12z8g4a4b4-|`},
		{"code identifier a4b4", "const level = compute(a4b4, 0.3)"},
		{"snake_case identifier track_b2", `{"track_b2": "vocal", "gain": -2.5}`},
		{"hex bytes d1ff c2e0 d2f0", "checksum d1ffc2e0d2f0 mismatch"},
		{"hex c3po style token", "device id c3po-1138 offline"},
		{"version string b5xx", "rollback to build b5xx failed"},
		{"b2 inside token", "base32 key ab2cd9 matched"},
		{"d1 inside token", "sha1 d1a7f3 truncated"},
		{"d3 inside token", "seed d3ad42 replayed"},
		{"c1 inside token", "color c1b0ff sampled"},
		{"b3 inside token", "row b32 col 7 empty"},
		{"b4 inside token", "cell a4b4 pinned"},
		{"c2 inside token", "c2ef checkpoint reached"},
	} {
		if got := inferProjectAwareCapability(tc.text); got != "" {
			t.Fatalf("%s: structured text hijacked into %q", tc.name, got)
		}
	}
	// The explicit-marker helpers share the token face: an embedded code must
	// not count as an explicit B4/C1 invocation either.
	if explicitB4InvocationMarker("把 a4b4 单元格的值改成 0.3") {
		t.Fatal("explicitB4InvocationMarker must not fire on an embedded a4b4 token")
	}
	if explicitB4InvocationMarker("hash 00b4ff prefix") {
		t.Fatal("explicitB4InvocationMarker must not fire on an embedded 00b4 token")
	}
	if explicitC1InvocationMarker("颜色 c1b0ff 的十六进制采样") {
		t.Fatal("explicitC1InvocationMarker must not fire on an embedded c1b0 token")
	}
}

func TestCapabilityExplicitCodesAndNaturalPhrasesStillMatch(t *testing.T) {
	for _, tc := range []struct {
		text string
		want string
	}{
		{"请做 B2 静态平衡", staticBalanceCapabilityID},
		{"b2 静态平衡", staticBalanceCapabilityID},
		{"b2静态平衡", staticBalanceCapabilityID}, // CJK rune is a token boundary
		{"B5 核心元素定位", staticBalanceCapabilityID},
		{"请先做整首工程的静态平衡", staticBalanceCapabilityID}, // natural phrase, unchanged
		{"b3 声像布局", panLayoutCapabilityID},
		{"重新安排所有轨道的声像布局", panLayoutCapabilityID}, // natural phrase, unchanged
		{"b4 低频关系", lowEndRelationCapabilityID},
		{"分析一下当前工程的低频关系", lowEndRelationCapabilityID}, // B4-ABLATION-1 pair fixture
		{"c1 频段清理", frequencyCleanupCapabilityID},
		{"run C1 frequency cleanup for the full project", frequencyCleanupCapabilityID},
		{"c2 动态控制", dynamicControlCapabilityID},
		{"run C2 dynamic control", dynamicControlCapabilityID},
		{"c3 观察空间与深度", spaceDepthCapabilityID},
		{"d1 母带前响度峰值观察", premasterObservationCapabilityID},
		{"D1", premasterObservationCapabilityID},
		{"d2 翻译检查", premasterObservationCapabilityID},
		{"pre_master.observation d2", premasterObservationCapabilityID},
		{"d3 参考曲对比观察", premasterObservationCapabilityID},
		{"全局响度检查", premasterObservationCapabilityID}, // AF-D1-LOUDNESS-SETTLE-1 family
	} {
		if got := inferProjectAwareCapability(tc.text); got != tc.want {
			t.Fatalf("inferProjectAwareCapability(%q)=%q want %q", tc.text, got, tc.want)
		}
	}
	// Explicit-marker helpers keep the standalone-code semantics.
	if !explicitB4InvocationMarker("运行 B4") {
		t.Fatal("explicitB4InvocationMarker must fire on the standalone B4 code")
	}
	if explicitB4InvocationMarker("把所选 EQ 的低频调整一下") {
		t.Fatal("explicitB4InvocationMarker must stay quiet for an ordinary EQ request")
	}
	if !explicitC1InvocationMarker("run c1") {
		t.Fatal("explicitC1InvocationMarker must fire on the standalone c1 code")
	}
}

// --- Defect 1 replay (run2 shape): the verbatim 278 run2 chat message must
// not be claimed by the pan_layout canary. Pre-fix: inferProjectAwareCapability
// returned static_mix.pan_layout.v0 (dumped on the pristine base tree) and
// handleCapabilityRuntimeCanary took the turn over (run2 artifact:
// workflow=capability_runtime_v1, goal_status=failed).

func TestGenChatRoutingRun2MessageNotClaimedByCanary(t *testing.T) {
	if got := inferProjectAwareCapability(genChatRoutingRunMessage); got != "" {
		t.Fatalf("run2 message still hijacked into %q (b32 inside the ABC score)", got)
	}
	server := &Server{orchestrationRuntime: orchestrationruntime.New()}
	response, handled := server.handleCapabilityRuntimeCanary(context.Background(), "conv-303-run2-replay", ChatRequest{
		Message: genChatRoutingRunMessage,
		Context: map[string]any{"agent_mode": agentModeDefault},
	}, agentruntime.Goal{})
	if handled {
		t.Fatalf("canary must not take over the run2 shape: workflow=%q reply=%.80s", response.Workflow, response.Reply)
	}
}

// --- Defect 2 replay (run3 shape): the same message behind "/goal " (prefix
// stripped before the goalrunner sees it) must disclose gen.generate in BOTH
// the narrowed allowlist and the catalog summary, while the mix/midi/transport
// packs it naturally hits keep their own tools. Pre-fix the model honestly
// reported the tool was absent (run3 artifact reply).

func TestGenChatRoutingRun3GoalMessageDisclosesGenerationTools(t *testing.T) {
	server := &Server{harness: harness.New(nil, nil, nil)}
	caps := agentLoopCapabilityNames(genChatRoutingRunMessage, nil)
	sawGeneration, sawMix, sawMidi, sawTransport := false, false, false, false
	for _, capability := range caps {
		switch capability {
		case "generation":
			sawGeneration = true
		case "mix":
			sawMix = true
		case "midi":
			sawMidi = true
		case "transport":
			sawTransport = true
		}
	}
	if !sawGeneration {
		t.Fatalf("generation capability missing for the run3 shape: %+v", caps)
	}
	// Existing packs stay armed exactly as pre-fix (dumped [mix midi transport] order-free).
	if !sawMix || !sawMidi || !sawTransport {
		t.Fatalf("existing packs must keep firing for the run3 shape: %+v", caps)
	}
	ctx := server.agentLoopToolContext(agentModeDefault, genChatRoutingRunMessage, nil)
	for _, want := range []string{"gen.generate", "gen.providers", "mix.request_observation"} {
		if !containsToolName(ctx.AllowedTools, want) {
			t.Fatalf("%s missing from narrowed toolset: %+v", want, ctx.AllowedTools)
		}
	}
	if !strings.Contains(ctx.CatalogSummary, "tool=gen.generate") {
		t.Fatalf("catalog summary (prompt disclosure face) missing gen.generate:\n%s", ctx.CatalogSummary)
	}
}

func TestGenChatRoutingGenerationIntentCoversUtteranceFamily(t *testing.T) {
	for _, text := range []string{
		"请用 YuE2 把下面这段谱唱出来",
		"用 AI 翻唱这首歌",
		"帮我生成一段 108 BPM 的鼓伴奏",
		"把这段旋律唱一段给我听",
		"generate audio for this progression",
		"make an ai cover of the chorus",
	} {
		if !agentLoopGenerationIntent(text) {
			t.Fatalf("generation intent must fire for %q", text)
		}
	}
}

// --- Defect 2 zero-change pin: utterances WITHOUT generation vocabulary keep
// their pre-fix capability sets (literal lists dumped from the pristine base
// tree at 5c5ef23e) and their toolsets stay free of gen.* tools. Bare
// vocal/drums/bpm mix words are NOT generation triggers.

func TestGenChatRoutingExistingPacksUnchangedWithoutGenerationIntent(t *testing.T) {
	server := &Server{harness: harness.New(nil, nil, nil)}
	battery := []struct {
		msg  string
		ctx  map[string]any
		want []string
	}{
		{"把当前主唱轨道往前一点", map[string]any{"selected_track_id": "track_1"}, []string{"mix"}},
		{"vocal 有点大，压低一点", nil, []string{"mix"}},
		{"把鼓组的节奏量化一下", nil, []string{"midi"}},
		{"工程 BPM 是多少", nil, []string{"transport"}},
		{"新建一条轨道", nil, []string{"track"}},
		{"输入 8 个 MIDI 音符", nil, []string{"midi"}},
		{"使用当前加载的均衡器调节 3400Hz 频段增益 3dB", map[string]any{"selected_track_id": "1007", "selected_plugin_id": "1013"}, []string{"mix", "plugin"}},
		{"分析一下当前工程的低频关系", nil, []string{"mix"}},
		{"把片段裁剪一下", nil, []string{"clip"}},
	}
	join := func(values []string) string {
		return strings.Join(values, " ")
	}
	for _, b := range battery {
		caps := agentLoopCapabilityNames(b.msg, b.ctx)
		if got := join(caps); got != join(b.want) {
			t.Fatalf("capability set drifted for %q: got %q want %q (pre-fix dump)", b.msg, got, join(b.want))
		}
		for _, capability := range caps {
			if capability == "generation" {
				t.Fatalf("generation capability leaked into a non-generation turn: %q -> %+v", b.msg, caps)
			}
		}
		tools := toolNamesForAgentLoopCapabilities(server.harness, agentModeDefault, caps)
		for _, forbidden := range []string{"gen.generate", "gen.providers"} {
			if containsToolName(tools, forbidden) {
				t.Fatalf("%s leaked into toolset for %q (capabilities %+v)", forbidden, b.msg, caps)
			}
		}
	}
}
