package chat

// LEGAME-AUTOINTRO-1（315）自动开场/恢复播报单测：binding×会话态矩阵四象限+幂等
//（开场工程级一次性/播报会话级一次性）+F5 竞态回归（281-B2 教训：注入必须观察
// activation+restore 的最终态，pre-activation 窗口不得写会话表或 binding）。
//
// 内核面沿用 eduJamKernelSendOverride 假内核（真实内核轨树形状，281 修复腿 2 后
// 惯例）；binding sidecar 落 t.TempDir 伪工程旁（.vit_agent/legame_binding.json，
// 与生产同一条 loadLegameBinding 代码路径）。

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// eduJamIntroProject 建一个伪工程路径（t.TempDir 下 proj.vit）。
func eduJamIntroProject(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "proj.vit")
}

// eduJamWriteIntroBinding 在伪工程旁写 binding sidecar（openedAt/openedConv 空
// 串=写 null）。
func eduJamWriteIntroBinding(t *testing.T, projectPath, schemaVersion, levelID, openedAt, openedConv string) {
	t.Helper()
	binding := legameBinding{
		SchemaVersion: schemaVersion,
		LevelID:       levelID,
		BoundAt:       time.Now().UTC().Format(time.RFC3339),
		BoundSource:   "placement_script",
	}
	if openedAt != "" {
		binding.OpenedAt = &openedAt
	}
	if openedConv != "" {
		binding.OpenedConversationID = &openedConv
	}
	path := legameBindingPath(projectPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir binding dir: %v", err)
	}
	data, err := json.MarshalIndent(binding, "", "  ")
	if err != nil {
		t.Fatalf("marshal binding: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write binding: %v", err)
	}
}

// eduJamIntroServer 建带伪工程绑定工作区的测试 Server（env 轴由
// newEduJamTestServer 统一指到仓库夹具）。
func eduJamIntroServer(t *testing.T, fake *eduJamFakeKernel, projectPath string) *Server {
	t.Helper()
	server := newEduJamTestServer(t, fake)
	server.mu.Lock()
	server.activeWorkspacePath = projectPath
	server.mu.Unlock()
	return server
}

// TestEduJamAutoIntroBindingSessionMatrix binding×会话态矩阵（设计稿 §2.2 验收
// 口径：注入开场/恢复播报/静默/零差异四象限全断言）。
func TestEduJamAutoIntroBindingSessionMatrix(t *testing.T) {
	ctx := context.Background()
	newFake := func() *eduJamFakeKernel {
		return &eduJamFakeKernel{
			tracks: eduJamRealShapeTracks(eduJamFakeTrack("t1"), eduJamFakeTrack("t2"), eduJamFakeTrack("t3")),
			notes:  map[string][]any{},
		}
	}
	seedActive := func(t *testing.T, server *Server, ownerConv string) {
		t.Helper()
		resp, handled := server.eduJamEnter(ctx, ownerConv, "chat", "")
		if !handled || resp.StopReason != "edu_jam_entered" {
			t.Fatalf("seed active jam failed: stop=%s reply=%s", resp.StopReason, resp.Reply)
		}
	}
	seedTerminal := func(t *testing.T, server *Server, ownerConv string) {
		t.Helper()
		seedActive(t, server, ownerConv)
		resp, handled := server.eduJamExit(ctx, ownerConv, "chat")
		if !handled || resp.StopReason != "edu_jam_exited" {
			t.Fatalf("seed terminal jam failed: stop=%s", resp.StopReason)
		}
	}

	cases := []struct {
		name       string
		binding    string // "" | "fresh" | "opened" | "bad-schema"
		session    string // "" | "active" | "terminal"
		wantServe  bool
		wantStop   string
		checkReply func(t *testing.T, reply string)
	}{
		{name: "no binding no session = zero diff", binding: "", session: "", wantServe: false},
		{name: "no binding active session = zero diff (broadcast is binding-gated)", binding: "", session: "active", wantServe: false},
		{name: "bad schema binding = treated as absent", binding: "bad-schema", session: "", wantServe: false},
		{name: "fresh binding no session = auto intro", binding: "fresh", session: "", wantServe: true, wantStop: "edu_jam_entered",
			checkReply: func(t *testing.T, reply string) {
				if !strings.Contains(reply, "乐局开始") {
					t.Fatalf("intro reply missing enter narrative: %s", reply)
				}
			}},
		{name: "fresh binding active jam = recovery broadcast", binding: "fresh", session: "active", wantServe: true, wantStop: "edu_jam_recovery_broadcast",
			checkReply: func(t *testing.T, reply string) {
				if !strings.Contains(reply, "接着上局") {
					t.Fatalf("broadcast reply missing recovery lead: %s", reply)
				}
			}},
		{name: "fresh binding terminal jam = intro again (terminal does not block)", binding: "fresh", session: "terminal", wantServe: true, wantStop: "edu_jam_entered"},
		{name: "opened binding active jam = recovery broadcast", binding: "opened", session: "active", wantServe: true, wantStop: "edu_jam_recovery_broadcast"},
		{name: "opened binding terminal jam = silent", binding: "opened", session: "terminal", wantServe: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := eduJamIntroServer(t, newFake(), eduJamIntroProject(t))
			switch tc.binding {
			case "fresh":
				eduJamWriteIntroBinding(t, server.activeWorkspacePath, legameBindingSchemaVersion, "first-run-twinkle", "", "")
			case "opened":
				eduJamWriteIntroBinding(t, server.activeWorkspacePath, legameBindingSchemaVersion, "first-run-twinkle", "2026-11-08T12:00:00Z", "conv-earlier")
			case "bad-schema":
				eduJamWriteIntroBinding(t, server.activeWorkspacePath, "vit.legame_binding.v9", "first-run-twinkle", "", "")
			}
			switch tc.session {
			case "active":
				seedActive(t, server, "conv-owner")
			case "terminal":
				seedTerminal(t, server, "conv-owner")
			}

			resp, served := server.eduJamAutoIntroCheckpoint(ctx, "conv-intro", "chat", "在吗")
			if served != tc.wantServe {
				t.Fatalf("served=%v want %v (resp=%+v)", served, tc.wantServe, resp)
			}
			if served {
				if resp.StopReason != tc.wantStop {
					t.Fatalf("stop=%s want %s reply=%s", resp.StopReason, tc.wantStop, resp.Reply)
				}
				if tc.checkReply != nil {
					tc.checkReply(t, resp.Reply)
				}
			}
			if tc.binding == "fresh" && tc.session == "" && served {
				// 开场后 binding 回写断言：opened_at 非空+会话 id=本轮会话。
				binding, ok := loadLegameBinding(server.activeWorkspacePath)
				if !ok || binding.OpenedAt == nil || strings.TrimSpace(*binding.OpenedAt) == "" {
					t.Fatalf("binding opened_at must be written after intro: %+v", binding)
				}
				if binding.OpenedConversationID == nil || *binding.OpenedConversationID != "conv-intro" {
					t.Fatalf("binding opened_conversation_id wrong: %+v", binding)
				}
				if session, ok := server.eduJamSessionFor("conv-intro"); !ok || session.Status != eduJamStatusActive {
					t.Fatalf("intro must leave an active session: ok=%v session=%+v", ok, session)
				}
			}
		})
	}
}

// TestEduJamAutoIntroIdempotency 幂等语义：开场工程级一次性（同会话第二轮不再注
// 入；第二会话不重开场）+恢复播报会话级一次性（同会话只播一次）。
func TestEduJamAutoIntroIdempotency(t *testing.T) {
	ctx := context.Background()
	fake := &eduJamFakeKernel{
		tracks: eduJamRealShapeTracks(eduJamFakeTrack("t1"), eduJamFakeTrack("t2"), eduJamFakeTrack("t3")),
		notes:  map[string][]any{},
	}
	project := eduJamIntroProject(t)
	server := eduJamIntroServer(t, fake, project)
	eduJamWriteIntroBinding(t, project, legameBindingSchemaVersion, "first-run-twinkle", "", "")

	// conv-1 第一轮白话 → 开场。
	resp, served := server.eduJamAutoIntroCheckpoint(ctx, "conv-1", "chat", "你好呀")
	if !served || resp.StopReason != "edu_jam_entered" {
		t.Fatalf("first turn must auto-intro: served=%v stop=%s reply=%s", served, resp.StopReason, resp.Reply)
	}
	// conv-1 第二轮白话 → 不再注入（会话级标记），流向普通会话轮。
	if _, served := server.eduJamAutoIntroCheckpoint(ctx, "conv-1", "chat", "继续聊"); served {
		t.Fatalf("second turn in the same conversation must not re-inject")
	}
	// conv-2（同工程新会话）→ 恢复播报，非重开场。
	resp, served = server.eduJamAutoIntroCheckpoint(ctx, "conv-2", "chat", "嗨")
	if !served || resp.StopReason != "edu_jam_recovery_broadcast" {
		t.Fatalf("second conversation must get recovery broadcast: served=%v stop=%s reply=%s", served, resp.StopReason, resp.Reply)
	}
	if !strings.Contains(resp.Reply, "接着上局") {
		t.Fatalf("broadcast reply wrong: %s", resp.Reply)
	}
	// conv-2 第二轮 → 不再播。
	if _, served := server.eduJamAutoIntroCheckpoint(ctx, "conv-2", "chat", "嗯嗯"); served {
		t.Fatalf("broadcast must fire once per conversation per process")
	}
	// binding 的 opened_conversation_id 仍是 conv-1（工程级一次性）。
	binding, ok := loadLegameBinding(project)
	if !ok || binding.OpenedConversationID == nil || *binding.OpenedConversationID != "conv-1" {
		t.Fatalf("opened_conversation_id must stay the intro conversation: %+v", binding)
	}
}

// TestEduJamAutoIntroBroadcastContent 播报内容（确定性模板，零 LLM）：n0 已过后
// 当前节点 n1（第 2/7 关）——「接着上局：读谱纠错（2/7）——继续或问我都行」。
func TestEduJamAutoIntroBroadcastContent(t *testing.T) {
	ctx := context.Background()
	theme := eduJamThemeNotes(t)
	fake := &eduJamFakeKernel{
		tracks: eduJamRealShapeTracks(
			eduJamFakeTrack("t1", eduJamFakeClip("c1", 26.0)),
			eduJamFakeTrack("t2"),
			eduJamFakeTrack("t3"),
		),
		notes: map[string][]any{"c1": eduJamNoteRows(theme)},
	}
	project := eduJamIntroProject(t)
	server := eduJamIntroServer(t, fake, project)
	eduJamWriteIntroBinding(t, project, legameBindingSchemaVersion, "first-run-twinkle", "2026-11-08T12:00:00Z", "conv-owner")

	// 局主会话推进过 n0（当前节点=n1_read_and_fix，第 2/7 关）。
	if _, handled := server.handleEduJamChat(ctx, "conv-owner", "chat", "开始乐局", eduJamChatCtx()); !handled {
		t.Fatalf("owner enter not handled")
	}
	if _, handled := server.handleEduJamChat(ctx, "conv-owner", "chat", "写好了", eduJamChatCtx()); !handled {
		t.Fatalf("n0 advance not handled")
	}

	resp, served := server.eduJamAutoIntroCheckpoint(ctx, "conv-new", "chat", "接着来")
	if !served || resp.StopReason != "edu_jam_recovery_broadcast" {
		t.Fatalf("broadcast expected: served=%v stop=%s", served, resp.StopReason)
	}
	for _, needle := range []string{"接着上局", "读谱纠错", "2/7"} {
		if !strings.Contains(resp.Reply, needle) {
			t.Fatalf("broadcast reply missing %q: %s", needle, resp.Reply)
		}
	}
	data := resp.WorkflowData
	if got := data["node_index"]; got != 2 {
		t.Fatalf("broadcast node_index=%v want 2", got)
	}
	if got := data["node_total"]; got != 7 {
		t.Fatalf("broadcast node_total=%v want 7", got)
	}
	if got := data["owner_conversation_id"]; got != "conv-owner" {
		t.Fatalf("owner_conversation_id=%v", got)
	}
	if got := data["current_node"]; got != "n1_read_and_fix" {
		t.Fatalf("current_node=%v", got)
	}
}

// TestEduJamAutoIntroCommandWordBypass 命令词/斜杠消息不经检查点（归既有命令面，
// 自动开场与「开始乐局」同一 enter 内核、语义单源）。
func TestEduJamAutoIntroCommandWordBypass(t *testing.T) {
	ctx := context.Background()
	fake := &eduJamFakeKernel{
		tracks: eduJamRealShapeTracks(eduJamFakeTrack("t1"), eduJamFakeTrack("t2"), eduJamFakeTrack("t3")),
		notes:  map[string][]any{},
	}
	project := eduJamIntroProject(t)
	server := eduJamIntroServer(t, fake, project)
	eduJamWriteIntroBinding(t, project, legameBindingSchemaVersion, "first-run-twinkle", "", "")

	for _, msg := range []string{"开始乐局", "乐局状态", "写好了", "/reset"} {
		if _, served := server.eduJamAutoIntroCheckpoint(ctx, "conv-cmd", "chat", msg); served {
			t.Fatalf("%q must bypass the auto-intro checkpoint (command faces own it)", msg)
		}
	}
	// 命令词本体照常工作（同一 enter 内核）。
	resp, handled := server.handleEduJamChat(ctx, "conv-cmd", "chat", "开始乐局", eduJamChatCtx())
	if !handled || resp.StopReason != "edu_jam_entered" {
		t.Fatalf("manual enter must still work: stop=%s", resp.StopReason)
	}
}

// TestEduJamAutoIntroF5ActivationRace F5 竞态回归（281-B2 教训入卡，两方向）：
//
//	方向 1（pre-activation 窗口）：内核已持有目标工程而工作区仍绑旧工程（旧工程
//	  旁也有 fresh binding）→ 检查点不得注入、不得写会话表、不得写 binding；此刻
//	  惰性激活的空态整表覆盖发生也无会话可丢。
//	方向 2（激活完成）：工作区绑定追上内核+空态 restore → 白话触发自动开场，会话
//	  绑定激活工作区 uuid，binding opened_at 落盘；随后新会话得恢复播报。
func TestEduJamAutoIntroF5ActivationRace(t *testing.T) {
	ctx := context.Background()
	theme := eduJamThemeNotes(t)
	fake := &eduJamFakeKernel{ // 内核真值 project_uuid=uuid-edu-jam-test（假内核固定）
		tracks: eduJamRealShapeTracks(
			eduJamFakeTrack("t1", eduJamFakeClip("c1", 26.0)),
			eduJamFakeTrack("t2"),
			eduJamFakeTrack("t3"),
		),
		notes: map[string][]any{"c1": eduJamNoteRows(theme)},
	}
	staleProject := eduJamIntroProject(t)
	realProject := eduJamIntroProject(t)
	server := newEduJamTestServer(t, fake)
	eduJamWriteIntroBinding(t, staleProject, legameBindingSchemaVersion, "first-run-twinkle", "", "")
	eduJamWriteIntroBinding(t, realProject, legameBindingSchemaVersion, "first-run-twinkle", "", "")

	// 时序起点（F5 形态）：内核已切到目标工程，工作区仍绑切换前旧草稿工程。
	server.mu.Lock()
	server.activeWorkspaceUUID = "uuid-stale-draft"
	server.activeWorkspacePath = staleProject
	server.mu.Unlock()

	// 方向 1：白话不注入；会话表空；stale binding 不被写。
	resp, served := server.eduJamAutoIntroCheckpoint(ctx, "conv-race", "chat", "在吗")
	if served {
		t.Fatalf("pre-activation window must not inject: %+v", resp)
	}
	if _, exists := server.eduJamSessionFor("conv-race"); exists {
		t.Fatalf("pre-activation window must not write the session table")
	}
	if binding, ok := loadLegameBinding(staleProject); !ok || binding.OpenedAt != nil {
		t.Fatalf("pre-activation window must not write binding: %+v ok=%v", binding, ok)
	}
	// F5 杀手锏复刻：此刻空态整表覆盖发生，无会话可丢。
	server.mu.Lock()
	server.eduJamSessions = server.restoreEduJamSessionsLocked(nil)
	server.mu.Unlock()

	// 方向 2：激活完成（绑定追上内核）+空态 restore → 自动开场生效。
	server.mu.Lock()
	server.activeWorkspaceUUID = "uuid-edu-jam-test"
	server.activeWorkspacePath = realProject
	server.eduJamSessions = server.restoreEduJamSessionsLocked(nil)
	server.mu.Unlock()
	resp, served = server.eduJamAutoIntroCheckpoint(ctx, "conv-race", "chat", "在吗")
	if !served || resp.StopReason != "edu_jam_entered" {
		t.Fatalf("post-activation white-talk must auto-intro: served=%v stop=%s reply=%s", served, resp.StopReason, resp.Reply)
	}
	session, ok := server.eduJamSessionFor("conv-race")
	if !ok || session.ProjectUUID != "uuid-edu-jam-test" {
		t.Fatalf("intro session must bind the activated workspace uuid: ok=%v session=%+v", ok, session)
	}
	if binding, ok := loadLegameBinding(realProject); !ok || binding.OpenedAt == nil || *binding.OpenedConversationID != "conv-race" {
		t.Fatalf("binding opened markers must be written to the activated project: %+v ok=%v", binding, ok)
	}
	// 新会话 → 恢复播报（非重开场）。
	resp, served = server.eduJamAutoIntroCheckpoint(ctx, "conv-race-2", "chat", "嗨")
	if !served || resp.StopReason != "edu_jam_recovery_broadcast" {
		t.Fatalf("second conversation must get recovery broadcast: served=%v stop=%s", served, resp.StopReason)
	}
}
