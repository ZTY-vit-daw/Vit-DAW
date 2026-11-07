package chat

// EDU-LEVEL-RUNTIME-B（乐局 3b）挂载面单测：以 279 定稿乐局卡
// coord/levels/first-run-twinkle/ 为夹具（经 VIT_EDU_LEVELS_DIR 指到仓库夹具，
// 与 edu 包测试同型只读用法），内核面用 eduJamKernelSendOverride 假内核——
// 判定链全确定性零 LLM，断言落在回复文本、WorkflowData 与会话态上。

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"sync"
	"testing"

	"vit-daw-agent/internal/edu"
)

const eduJamTestLevelsDir = "../../../coord/levels"

// eduJamFakeKernel 是乐局读面的假内核：固定工程态 + clip→notes 表。
type eduJamFakeKernel struct {
	mu     sync.Mutex
	tracks []any
	notes  map[string][]any
}

func (f *eduJamFakeKernel) send(_ context.Context, cmd map[string]any) (map[string]any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch cmd["cmd"] {
	case "get_project_state":
		return map[string]any{
			"status":       "ok",
			"project_uuid": "uuid-edu-jam-test",
			"tracks":       f.tracks,
			"music_context": map[string]any{
				"tempo_bpm": 100.0, "tempo_setting_count": 1, "time_signature": "4/4",
			},
		}, nil
	case "get_midi_clip_notes":
		clipID, _ := cmd["clip_id"].(string)
		rows, ok := f.notes[clipID]
		if !ok {
			return map[string]any{"status": "error", "message": "clip not found: " + clipID}, nil
		}
		return map[string]any{"status": "ok", "notes": rows}, nil
	}
	return map[string]any{"status": "error", "message": "unknown cmd"}, nil
}

func eduJamFakeClip(clipID string, endSeconds float64) map[string]any {
	return map[string]any{"id": clipID, "clip_type": "midi", "start_seconds": 0.0, "end_seconds": endSeconds}
}

// eduJamFakeTrack 造一条**真实内核用户轨形状**的假轨行（shadow_test.go:132-136
// 同形状：track_type=hybrid + is_audio_track=true——真实内核用户轨必带正判标记；
// 修复腿 2 后槽寻址先过 eduJamUserTrackRows 过滤，无标记的合成行会被当服务轨
// 滤掉，故假用户轨必须携带真实形状标记）。
func eduJamFakeTrack(trackID string, clips ...map[string]any) map[string]any {
	rows := make([]any, 0, len(clips))
	for _, c := range clips {
		rows = append(rows, c)
	}
	return map[string]any{"track_id": trackID, "track_type": "hybrid", "is_audio_track": true, "clips": rows}
}

// eduJamServiceTracks 造真实内核轨树前部固定存在的 5 条服务轨
// （shadow_test.go:127-131 同形状钉死；fixture 工程实证
// edu_jam_fixture_project.vit:40-48：Arranger 1002/Chord 1003/Marker 1004/
// Tempo 1005/Master 1006 前置于用户轨）。274 教训：夹具形状必须反钉真实内核，
// 合成形状与真实不符曾掩盖本寻址缺陷面（smokerun2 取证）。
func eduJamServiceTracks() []any {
	return []any{
		map[string]any{"track_id": "1002", "track_name": "Arranger", "track_type": "track", "is_audio_track": false},
		map[string]any{"track_id": "1003", "track_name": "Chord", "track_type": "track", "is_audio_track": false},
		map[string]any{"track_id": "1004", "track_name": "Marker", "track_type": "track", "is_audio_track": false},
		map[string]any{"track_id": "1005", "track_name": "Tempo", "track_type": "track", "is_audio_track": false},
		map[string]any{"track_id": "1006", "track_name": "Master", "track_type": "master", "is_audio_track": false},
	}
}

// eduJamRealShapeTracks 以真实内核轨树形状组装 get_project_state 的 tracks：
// 5 条服务轨前缀 + 用户轨。零用户轨（空工程）=eduJamRealShapeTracks()。
func eduJamRealShapeTracks(userTracks ...map[string]any) []any {
	out := eduJamServiceTracks()
	for _, track := range userTracks {
		out = append(out, track)
	}
	return out
}

// eduJamNoteRows 把 edu.Note 投影成内核 get_midi_clip_notes 的 notes 行。
func eduJamNoteRows(notes []edu.Note) []any {
	rows := make([]any, 0, len(notes))
	for _, n := range notes {
		rows = append(rows, map[string]any{"id": n.ID, "pitch": n.Pitch, "start": n.Start, "length": n.Length})
	}
	return rows
}

// newEduJamTestServer 建一个带假内核与激活工作区绑定的测试 Server，并加载
// 乐局卡夹具。theme/corrupted 由调用方选择注入哪些轨。
func newEduJamTestServer(t *testing.T, fake *eduJamFakeKernel) *Server {
	t.Helper()
	t.Setenv("VIT_EDU_LEVELS_DIR", eduJamTestLevelsDir)
	server := newChatServerForTest(t, nil, nil, nil)
	server.eduJamKernelSendOverride = fake.send
	server.mu.Lock()
	server.activeWorkspaceUUID = "uuid-edu-jam-test"
	server.mu.Unlock()
	return server
}

// eduJamThemeNotes 取夹具卡 theme 资产的可判定音符（42 音全表）。
func eduJamThemeNotes(t *testing.T) []edu.Note {
	t.Helper()
	card, err := edu.LoadLevelCard(eduJamTestLevelsDir + "/first-run-twinkle/level.yaml")
	if err != nil {
		t.Fatalf("load fixture level card: %v", err)
	}
	notes, ok := card.AssetNotes("theme")
	if !ok || len(notes) == 0 {
		t.Fatalf("fixture theme notes unavailable")
	}
	return notes
}

// eduJamCorruptedNotes 取 theme 全表并把 37 拍处 69 改 68（279 卡 §2.2 损坏点）。
func eduJamCorruptedNotes(t *testing.T) []edu.Note {
	t.Helper()
	notes := append([]edu.Note(nil), eduJamThemeNotes(t)...)
	hit := false
	for i := range notes {
		if notes[i].Start == 37.0 && notes[i].Pitch == 69 {
			notes[i].Pitch = 68
			hit = true
		}
	}
	if !hit {
		t.Fatalf("corruption anchor (beat 37 pitch 69) not found in theme fixture")
	}
	return notes
}

func eduJamChatCtx() map[string]any { return map[string]any{} }

func TestEduJamEnterStatusIdempotentReject(t *testing.T) {
	fake := &eduJamFakeKernel{
		tracks: eduJamRealShapeTracks(eduJamFakeTrack("t1"), eduJamFakeTrack("t2"), eduJamFakeTrack("t3")),
		notes:  map[string][]any{},
	}
	server := newEduJamTestServer(t, fake)
	ctx := context.Background()

	resp, handled := server.handleEduJamChat(ctx, "conv-edu-1", "chat", "开始乐局", eduJamChatCtx())
	if !handled {
		t.Fatalf("enter not handled")
	}
	if resp.StopReason != "edu_jam_entered" {
		t.Fatalf("enter stop_reason=%s reply=%s", resp.StopReason, resp.Reply)
	}
	if !strings.Contains(resp.Reply, "乐局开始") || !strings.Contains(resp.Reply, "拍谱开局") {
		t.Fatalf("enter reply missing story/first node: %s", resp.Reply)
	}
	if got := resp.WorkflowData["level_id"]; got != "first-run-twinkle" {
		t.Fatalf("level_id=%v", got)
	}
	if got := resp.WorkflowData["current_node"]; got != "n0_photo_score_open" {
		t.Fatalf("current_node=%v", got)
	}

	resp, handled = server.handleEduJamChat(ctx, "conv-edu-1", "chat", "开始乐局", eduJamChatCtx())
	if !handled || resp.StopReason != "edu_jam_enter_idempotent_rejected" {
		t.Fatalf("duplicate enter not idempotently rejected: handled=%v stop=%s", handled, resp.StopReason)
	}

	resp, handled = server.handleEduJamChat(ctx, "conv-edu-1", "chat", "现在第几关", eduJamChatCtx())
	if !handled || !strings.Contains(resp.Reply, "第 1 关") || !strings.Contains(resp.Reply, "进行中") {
		t.Fatalf("status reply unexpected: handled=%v reply=%s", handled, resp.Reply)
	}
}

func TestEduJamAdvanceThroughN0JudgeN1(t *testing.T) {
	theme := eduJamThemeNotes(t)
	fake := &eduJamFakeKernel{
		tracks: eduJamRealShapeTracks(
			eduJamFakeTrack("t1", eduJamFakeClip("c1", 26.0)),
			eduJamFakeTrack("t2", eduJamFakeClip("c2", 26.0)),
			eduJamFakeTrack("t3"),
		),
		notes: map[string][]any{
			"c1": eduJamNoteRows(theme),
			"c2": eduJamNoteRows(eduJamCorruptedNotes(t)),
		},
	}
	server := newEduJamTestServer(t, fake)
	ctx := context.Background()

	if _, handled := server.handleEduJamChat(ctx, "conv-edu-2", "chat", "开始乐局", eduJamChatCtx()); !handled {
		t.Fatalf("enter not handled")
	}
	// n0：clip_exists(melody_imported) + human_replied（检查消息本身）→ done。
	resp, handled := server.handleEduJamChat(ctx, "conv-edu-2", "chat", "写好了", eduJamChatCtx())
	if !handled || resp.StopReason != "edu_jam_node_done" {
		t.Fatalf("n0 advance unexpected: handled=%v stop=%s reply=%s", handled, resp.StopReason, resp.Reply)
	}
	if got := resp.WorkflowData["node_done"]; got != "n0_photo_score_open" {
		t.Fatalf("n0 node_done=%v", got)
	}
	if !strings.Contains(resp.Reply, "读谱纠错") {
		t.Fatalf("n0 reply missing next node goal: %s", resp.Reply)
	}
	// n1：judge note_sequence_match melody_imported vs theme（逐音一致）→ done。
	resp, handled = server.handleEduJamChat(ctx, "conv-edu-2", "chat", "写好了", eduJamChatCtx())
	if !handled || resp.WorkflowData["node_done"] != "n1_read_and_fix" {
		t.Fatalf("n1 judge pass unexpected: stop=%s data=%v reply=%s", resp.StopReason, resp.WorkflowData, resp.Reply)
	}
	if !strings.Contains(resp.Reply, "定速度") {
		t.Fatalf("n1 reply missing n2 goal: %s", resp.Reply)
	}
}

func TestEduJamJudgeFailAttemptFeedbackAndPullback(t *testing.T) {
	fake := &eduJamFakeKernel{
		tracks: eduJamRealShapeTracks(
			eduJamFakeTrack("t1", eduJamFakeClip("c1", 26.0)),
			eduJamFakeTrack("t2"),
			eduJamFakeTrack("t3"),
		),
		notes: map[string][]any{
			"c1": eduJamNoteRows(eduJamCorruptedNotes(t)), // n1 判定输入=损坏表 → fail
		},
	}
	server := newEduJamTestServer(t, fake)
	ctx := context.Background()
	if _, handled := server.handleEduJamChat(ctx, "conv-edu-3", "chat", "开始乐局", eduJamChatCtx()); !handled {
		t.Fatalf("enter not handled")
	}
	if _, handled := server.handleEduJamChat(ctx, "conv-edu-3", "chat", "写好了", eduJamChatCtx()); !handled {
		t.Fatalf("n0 advance not handled")
	}
	// n1 判定失败：attempt=1 + 缺口反馈 + 提示阶梯首级。
	resp, handled := server.handleEduJamChat(ctx, "conv-edu-3", "chat", "写好了", eduJamChatCtx())
	if !handled || resp.StopReason != "edu_jam_attempt_failed" {
		t.Fatalf("n1 judge fail unexpected: stop=%s reply=%s", resp.StopReason, resp.Reply)
	}
	if got := resp.WorkflowData["attempt"]; got != 1 {
		t.Fatalf("attempt=%v data=%v", got, resp.WorkflowData)
	}
	if !strings.Contains(resp.Reply, "第 1 次尝试") || !strings.Contains(resp.Reply, "✗ judge:note_sequence_match") {
		t.Fatalf("attempt feedback missing verdict row: %s", resp.Reply)
	}
	if !strings.Contains(resp.Reply, "先让听全曲") {
		t.Fatalf("attempt feedback missing hint rung: %s", resp.Reply)
	}

	// 拉回：pullback_after_turns=4（279 卡 guidance）。判定失败那轮已计 1 次未推进，
	// 故第 3 条普通轮到阈值：前 2 条放行且不注入，第 3 条放行但注入当前节点目标，
	// 随后计数归零。
	for i := 0; i < 2; i++ {
		chatCtx := eduJamChatCtx()
		_, handled := server.handleEduJamChat(ctx, "conv-edu-3", "chat", "为什么这样写？", chatCtx)
		if handled {
			t.Fatalf("ordinary turn %d stolen by jam face", i)
		}
		if _, injected := chatCtx[eduJamPullbackContextKey]; injected {
			t.Fatalf("pullback injected early at turn %d", i)
		}
	}
	chatCtx := eduJamChatCtx()
	if _, handled := server.handleEduJamChat(ctx, "conv-edu-3", "chat", "嗯嗯", chatCtx); handled {
		t.Fatalf("pullback turn must still flow to the normal loop")
	}
	injection, ok := chatCtx[eduJamPullbackContextKey].(map[string]any)
	if !ok {
		t.Fatalf("pullback not injected at threshold turn: %v", chatCtx)
	}
	if injection["node_id"] != "n1_read_and_fix" || strings.TrimSpace(injection["goal"].(string)) == "" {
		t.Fatalf("pullback injection content wrong: %v", injection)
	}
	// 注入后计数归零：再 3 条普通轮不注入。
	for i := 0; i < 3; i++ {
		next := eduJamChatCtx()
		_, _ = server.handleEduJamChat(ctx, "conv-edu-3", "chat", "继续聊", next)
		if _, injected := next[eduJamPullbackContextKey]; injected {
			t.Fatalf("pullback re-injected before next window (%d)", i)
		}
	}
}

func TestEduJamClosureConfirmTerminalEventAndSettleHooks(t *testing.T) {
	theme := eduJamThemeNotes(t)
	fake := &eduJamFakeKernel{
		tracks: eduJamRealShapeTracks(
			eduJamFakeTrack("t1", eduJamFakeClip("c1", 26.0)),
			eduJamFakeTrack("t2", eduJamFakeClip("c2", 26.0)), // 学生已改对（=theme）
			eduJamFakeTrack("t3", eduJamFakeClip("c3", 26.0)), // 伴奏已填（closure 前提）
		),
		notes: map[string][]any{
			"c1": eduJamNoteRows(theme),
			"c2": eduJamNoteRows(theme),
			"c3": eduJamNoteRows(theme[:7]),
		},
	}
	server := newEduJamTestServer(t, fake)
	ctx := context.Background()
	conv := "conv-edu-4"
	say := func(msg string) (ChatResponse, bool) {
		return server.handleEduJamChat(ctx, conv, "chat", msg, eduJamChatCtx())
	}
	mustHandled := func(msg, wantStop string) ChatResponse {
		t.Helper()
		resp, handled := say(msg)
		if !handled {
			t.Fatalf("%q not handled", msg)
		}
		if wantStop != "" && resp.StopReason != wantStop {
			t.Fatalf("%q stop=%s want=%s reply=%s", msg, resp.StopReason, wantStop, resp.Reply)
		}
		return resp
	}

	mustHandled("开始乐局", "edu_jam_entered")
	mustHandled("写好了", "edu_jam_node_done") // n0
	mustHandled("写好了", "edu_jam_node_done") // n1（judge 过）
	// 未知事件 fail-closed 不记录。
	if resp := mustHandled("乐局事件 bogus_event", "edu_jam_unknown_event"); !strings.Contains(resp.Reply, "fail-closed") {
		t.Fatalf("unknown event reply missing fail-closed wording: %s", resp.Reply)
	}
	mustHandled("乐局事件 user_confirmed tempo", "edu_jam_event_recorded")
	mustHandled("写好了", "edu_jam_node_done") // n2（event+tempo_set）
	mustHandled("写好了", "edu_jam_node_done") // n3（judge melody_corrupted 已改对）
	mustHandled("乐局事件 user_chose variation_direction", "edu_jam_event_recorded")
	mustHandled("写好了", "edu_jam_node_done") // n4
	mustHandled("乐局事件 user_feedback_given", "edu_jam_event_recorded")
	mustHandled("乐局事件 user_confirmed final", "edu_jam_event_recorded")
	mustHandled("写好了", "edu_jam_node_done") // n5
	mustHandled("乐局事件 export_completed", "edu_jam_event_recorded")
	// n6 完成 → 全节点 done → 闭合判定通过 → 问询。
	resp := mustHandled("写好了", "edu_jam_closure_inquiry")
	if !strings.Contains(resp.Reply, "closure_inquiry") {
		t.Fatalf("closure inquiry reply missing semantic template marker: %s", resp.Reply)
	}
	if got := resp.WorkflowData["semantic_template"]; got != "closure_inquiry" {
		t.Fatalf("semantic_template=%v", got)
	}
	// 确认收尾 → 终态 + 沉淀钩子位 + trajectory 终局面。
	resp = mustHandled("确认收尾", "edu_jam_completed")
	if got := resp.WorkflowData["end_reason"]; got != "closure_confirmed" {
		t.Fatalf("end_reason=%v", got)
	}
	if mem, ok := resp.WorkflowData["memory_writes"].([]string); !ok || len(mem) == 0 {
		t.Fatalf("memory_writes hook missing: %v", resp.WorkflowData["memory_writes"])
	}
	settled := false
	server.mu.Lock()
	for _, ev := range server.events[conv] {
		if ev.Type == "trajectory.settled" && ev.Status == "completed" && ev.ItemID == "edu_jam:first-run-twinkle" {
			settled = true
		}
	}
	server.mu.Unlock()
	if !settled {
		t.Fatalf("terminal trajectory.settled event not emitted for %s", conv)
	}
	// 终态后再推进/再退出=幂等说明，不再改状态。
	if resp := mustHandled("写好了", "edu_jam_terminal"); !strings.Contains(resp.Reply, "不再推进") {
		t.Fatalf("terminal advance reply unexpected: %s", resp.Reply)
	}
	mustHandled("结束乐局", "edu_jam_terminal")
}

func TestEduJamClosureBlockedOnPlaceholderBlank(t *testing.T) {
	theme := eduJamThemeNotes(t)
	fake := &eduJamFakeKernel{
		tracks: eduJamRealShapeTracks(
			eduJamFakeTrack("t1", eduJamFakeClip("c1", 26.0)),
			eduJamFakeTrack("t2", eduJamFakeClip("c2", 26.0)),
			eduJamFakeTrack("t3"), // 伴奏轨留白 → no_placeholder_blanks 缺口
		),
		notes: map[string][]any{
			"c1": eduJamNoteRows(theme),
			"c2": eduJamNoteRows(theme),
		},
	}
	server := newEduJamTestServer(t, fake)
	ctx := context.Background()
	conv := "conv-edu-5"
	for _, msg := range []string{"开始乐局", "写好了", "写好了",
		"乐局事件 user_confirmed tempo", "写好了", "写好了",
		"乐局事件 user_chose variation_direction", "写好了",
		"乐局事件 user_feedback_given", "乐局事件 user_confirmed final", "写好了",
		"乐局事件 export_completed"} {
		if _, handled := server.handleEduJamChat(ctx, conv, "chat", msg, eduJamChatCtx()); !handled {
			t.Fatalf("%q not handled", msg)
		}
	}
	resp, handled := server.handleEduJamChat(ctx, conv, "chat", "写好了", eduJamChatCtx())
	if !handled || resp.StopReason != "edu_jam_closure_blocked" {
		t.Fatalf("closure should be blocked by placeholder blank: stop=%s reply=%s", resp.StopReason, resp.Reply)
	}
	if !strings.Contains(resp.Reply, "accompaniment_blank") {
		t.Fatalf("closure gap must name the blank slot: %s", resp.Reply)
	}
	// 缺口未补时确认收尾被回绝。
	resp, handled = server.handleEduJamChat(ctx, conv, "chat", "确认收尾", eduJamChatCtx())
	if !handled || resp.StopReason != "edu_jam_closure_preconditions" {
		t.Fatalf("confirm before closure pass must be rejected: stop=%s", resp.StopReason)
	}
}

func TestEduJamExitFromActiveAndTrajectoryTerminalFace(t *testing.T) {
	fake := &eduJamFakeKernel{
		tracks: eduJamRealShapeTracks(eduJamFakeTrack("t1"), eduJamFakeTrack("t2"), eduJamFakeTrack("t3")),
		notes:  map[string][]any{},
	}
	server := newEduJamTestServer(t, fake)
	ctx := context.Background()
	conv := "conv-edu-6"
	if _, handled := server.handleEduJamChat(ctx, conv, "chat", "开始乐局", eduJamChatCtx()); !handled {
		t.Fatalf("enter not handled")
	}
	resp, handled := server.handleEduJamChat(ctx, conv, "chat", "结束乐局", eduJamChatCtx())
	if !handled || resp.StopReason != "edu_jam_exited" {
		t.Fatalf("exit unexpected: stop=%s reply=%s", resp.StopReason, resp.Reply)
	}
	stopped := false
	server.mu.Lock()
	for _, ev := range server.events[conv] {
		if ev.Type == "trajectory.settled" && ev.Status == "stopped" {
			stopped = true
		}
	}
	server.mu.Unlock()
	if !stopped {
		t.Fatalf("exit terminal trajectory event (stopped) not emitted")
	}
	if resp, _ := server.handleEduJamChat(ctx, conv, "chat", "乐局状态", eduJamChatCtx()); !strings.Contains(resp.Reply, "已退出") {
		t.Fatalf("status after exit unexpected: %s", resp.Reply)
	}
}

func TestEduJamCheckAndConfirmWithoutJamNotHandled(t *testing.T) {
	fake := &eduJamFakeKernel{tracks: eduJamRealShapeTracks(), notes: map[string][]any{}}
	server := newEduJamTestServer(t, fake)
	ctx := context.Background()
	for _, msg := range []string{"写好了", "结束乐局", "确认收尾", "开始乐局吧，我想先聊聊"} {
		if _, handled := server.handleEduJamChat(ctx, "conv-edu-7", "chat", msg, eduJamChatCtx()); handled {
			t.Fatalf("%q without an active jam must flow to the normal loop", msg)
		}
	}
	// 自然语别名（书名号）仍入局。
	if _, handled := server.handleEduJamChat(ctx, "conv-edu-7", "chat", "开始乐局《小星星》", eduJamChatCtx()); !handled {
		t.Fatalf("alias enter should be handled")
	}
}

func TestEduJamEnterFailClosedWithoutProjectBinding(t *testing.T) {
	fake := &eduJamFakeKernel{tracks: eduJamRealShapeTracks(), notes: map[string][]any{}}
	server := newEduJamTestServer(t, fake)
	server.eduJamKernelSendOverride = nil // 无内核读面
	server.mu.Lock()
	server.activeWorkspaceUUID = "" // 无激活工作区
	server.mu.Unlock()
	// 281 修复腿后该场景由激活门收口（裁定 B 形态）：stop_reason 从
	// edu_jam_project_binding_unavailable 演进为 edu_jam_workspace_activation_incomplete，
	// 细分原因进 WorkflowData.gate_reason（此处工作区未激活先于内核读面判定）。
	resp, handled := server.handleEduJamChat(context.Background(), "conv-edu-8", "chat", "开始乐局", eduJamChatCtx())
	if !handled || resp.StopReason != "edu_jam_workspace_activation_incomplete" {
		t.Fatalf("enter without binding must fail closed: handled=%v stop=%s", handled, resp.StopReason)
	}
	if got := resp.WorkflowData["gate_reason"]; got != "workspace_not_activated" {
		t.Fatalf("gate_reason=%v data=%v", got, resp.WorkflowData)
	}
	if !strings.Contains(resp.Reply, "工程还没打开好") {
		t.Fatalf("gate reject reply must be actionable: %s", resp.Reply)
	}
}

func TestEduJamPersistenceRoundTripAndRestoreFailClosed(t *testing.T) {
	theme := eduJamThemeNotes(t)
	fake := &eduJamFakeKernel{
		tracks: eduJamRealShapeTracks(
			eduJamFakeTrack("t1", eduJamFakeClip("c1", 26.0)),
			eduJamFakeTrack("t2"),
			eduJamFakeTrack("t3"),
		),
		notes: map[string][]any{"c1": eduJamNoteRows(theme)},
	}
	server := newEduJamTestServer(t, fake)
	ctx := context.Background()
	conv := "conv-edu-9"
	if _, handled := server.handleEduJamChat(ctx, conv, "chat", "开始乐局", eduJamChatCtx()); !handled {
		t.Fatalf("enter not handled")
	}
	if _, handled := server.handleEduJamChat(ctx, conv, "chat", "写好了", eduJamChatCtx()); !handled {
		t.Fatalf("n0 advance not handled")
	}

	// 快照 JSON 往返（workspace 持久化面，projectAgentRuntimeState 字段族）。
	snapshot := server.projectAgentRuntimeStateLocked()
	blob, err := json.Marshal(snapshot.EduJamSessions)
	if err != nil {
		t.Fatalf("marshal sessions: %v", err)
	}
	restored := map[string]eduJamSession{}
	if err := json.Unmarshal(blob, &restored); err != nil {
		t.Fatalf("unmarshal sessions: %v", err)
	}
	clean := server.restoreEduJamSessionsLocked(restored)
	session, ok := clean[conv]
	if !ok {
		t.Fatalf("valid session dropped on restore: %v", clean)
	}
	if session.Status != eduJamStatusActive || session.Jam.LevelID != "first-run-twinkle" {
		t.Fatalf("restored session fields wrong: %+v", session)
	}
	if eduJamNodeStateOf(session, "n0_photo_score_open") != edu.JamNodeDone {
		t.Fatalf("restored n0 state lost: %+v", session.Jam.Nodes)
	}

	// fail-closed：未知 schema / 坏 level 绑定 / 未知状态值 → 丢弃。
	bad := map[string]eduJamSession{}
	unknownSchema := restored[conv]
	unknownSchema.SchemaVersion = "vit.edu_jam_session.v9"
	bad[conv] = unknownSchema
	if got := server.restoreEduJamSessionsLocked(bad); len(got) != 0 {
		t.Fatalf("unknown schema must drop the session: %v", got)
	}
	badBinding := restored[conv]
	badBinding.Jam.LevelID = "another-level" // 与卡绑定不符 → RestoreJamState 拒绝
	bad[conv] = badBinding
	if got := server.restoreEduJamSessionsLocked(bad); len(got) != 0 {
		t.Fatalf("level binding mismatch must drop the session: %v", got)
	}
	badStatus := restored[conv]
	badStatus.Status = "mystery"
	bad[conv] = badStatus
	if got := server.restoreEduJamSessionsLocked(bad); len(got) != 0 {
		t.Fatalf("unknown status must drop the session: %v", got)
	}
	// 既有工程无记录 → 空覆盖层（零行为变化）。
	if got := server.restoreEduJamSessionsLocked(nil); len(got) != 0 {
		t.Fatalf("nil overlay must restore empty: %v", got)
	}
}

// TestEduJamEnterActivationRacePreClosedPostSurvives 是 281 修复腿的竞态建模
// 单测（主管裁定 B 形态），复刻 smokerun1_20261107_0924 exit 4 的
// 「enter→激活→推进」时序。段 1 的 9 个测例全在单一预置工作区状态下跑，未建
// 模请求内激活切换——本测即补该交互面，两方向：
//
//	方向 1（pre-activation enter 被 fail-closed 拒绝）：内核已切到新工程而工作
//	  区仍绑旧工程（shadow 惯性窗口=竞态入口形态）→ enter 必须拒绝且不落会话
//	  （stop_reason=edu_jam_workspace_activation_incomplete、回话含「工程还没
//	  打开好…稍等」）；此刻惰性激活的空态整表覆盖（server.go 恢复面同型覆盖行
//	  `s.eduJamSessions = s.restoreEduJamSessionsLocked(nil)`）发生也无会话可丢。
//
//	方向 2（激活后 enter 成功且 turn 2 仍查得会话）：工作区绑定追上内核工程 →
//	  enter 成功并绑定激活工作区 uuid → 请求边界再次激活的 restore 重放（同身份
//	  激活从持久化快照 restore，freeStateLoop 同型）不丢会话 → 「写好了」推进轮
//	  查得会话并在拦截面内闭环（不再放行语义入口）。
//
// 边界（如实声明）：门通过后内核才切换工程=用户真实换工程，属 B5 跨身份切换
// 语义（settle+新工程快照 restore），不在本竞态类，不在本测覆盖内。
func TestEduJamEnterActivationRacePreClosedPostSurvives(t *testing.T) {
	theme := eduJamThemeNotes(t)
	fake := &eduJamFakeKernel{
		tracks: eduJamRealShapeTracks(
			eduJamFakeTrack("t1", eduJamFakeClip("c1", 26.0)),
			eduJamFakeTrack("t2"),
			eduJamFakeTrack("t3"),
		),
		notes: map[string][]any{"c1": eduJamNoteRows(theme)},
	}
	server := newEduJamTestServer(t, fake)
	ctx := context.Background()
	conv := "conv-edu-race"

	// 时序起点（smokerun1 形态）：内核已持有 fixture 工程（fake kernel
	// project_uuid=uuid-edu-jam-test），工作区仍绑切换前的旧草稿工程。
	server.mu.Lock()
	server.activeWorkspaceUUID = "uuid-stale-draft"
	server.mu.Unlock()

	// —— 方向 1：pre-activation enter → fail-closed 拒绝 + 会话表保持空。
	resp, handled := server.handleEduJamChat(ctx, conv, "chat", "开始乐局", eduJamChatCtx())
	if !handled {
		t.Fatalf("pre-activation enter must be handled (fail-closed reject), never released to the semantic entry")
	}
	if resp.StopReason != "edu_jam_workspace_activation_incomplete" {
		t.Fatalf("pre-activation enter stop=%s want edu_jam_workspace_activation_incomplete reply=%s", resp.StopReason, resp.Reply)
	}
	if !strings.Contains(resp.Reply, "工程还没打开好") || !strings.Contains(resp.Reply, "稍等") {
		t.Fatalf("gate reject reply must tell the user to retry later: %s", resp.Reply)
	}
	if got := resp.WorkflowData["gate_reason"]; got != "workspace_identity_lagging_kernel" {
		t.Fatalf("gate_reason=%v data=%v", got, resp.WorkflowData)
	}
	if _, exists := server.eduJamSessionFor(conv); exists {
		t.Fatalf("rejected enter must not store a session")
	}
	// 竞态杀手锏验证：此刻惰性激活的空态整表覆盖（server.go 恢复面对新工程
	// 无盘上记录时的同型覆盖行）发生，也无在途会话可丢。
	server.mu.Lock()
	server.eduJamSessions = server.restoreEduJamSessionsLocked(nil)
	server.mu.Unlock()
	if _, exists := server.eduJamSessionFor(conv); exists {
		t.Fatalf("empty-state full-table overwrite must find nothing to wipe (race class defused)")
	}

	// —— 激活完成：工作区绑定追上内核工程（新工程盘上无历史会话记录）。
	server.mu.Lock()
	server.activeWorkspaceUUID = "uuid-edu-jam-test"
	server.eduJamSessions = server.restoreEduJamSessionsLocked(nil)
	server.mu.Unlock()

	// —— 方向 2a：激活后 enter 成功，绑定=激活工作区 uuid。
	resp, handled = server.handleEduJamChat(ctx, conv, "chat", "开始乐局", eduJamChatCtx())
	if !handled || resp.StopReason != "edu_jam_entered" {
		t.Fatalf("post-activation enter unexpected: handled=%v stop=%s reply=%s", handled, resp.StopReason, resp.Reply)
	}
	session, ok := server.eduJamSessionFor(conv)
	if !ok || session.ProjectUUID != "uuid-edu-jam-test" {
		t.Fatalf("post-activation enter must bind the activated workspace uuid: ok=%v session=%+v", ok, session)
	}
	// 请求边界再次激活：同身份从持久化快照重放 restore（enter 已同步落盘；
	// 以 projectAgentRuntimeStateLocked 快照模拟盘上状态，freeStateLoop 同型）。
	server.mu.Lock()
	snapshot := server.projectAgentRuntimeStateLocked()
	server.eduJamSessions = server.restoreEduJamSessionsLocked(snapshot.EduJamSessions)
	server.mu.Unlock()

	// —— 方向 2b：turn 2 推进轮必须查得会话并在拦截面内闭环（n0 判定过），
	// 不再重演「查无会话按设计放行 → semantic_entry」丢局路径。
	resp, handled = server.handleEduJamChat(ctx, conv, "chat", "写好了", eduJamChatCtx())
	if !handled || resp.StopReason != "edu_jam_node_done" || resp.WorkflowData["node_done"] != "n0_photo_score_open" {
		t.Fatalf("turn-2 advance must find the session after boundary restore: handled=%v stop=%s data=%v reply=%s",
			handled, resp.StopReason, resp.WorkflowData, resp.Reply)
	}
}

// eduJamNotesEqual 逐音比较（id/pitch/start/length；start/length 用 1e-6 容差，
// 与 edu.beatEpsilon 同口径）。
func eduJamNotesEqual(a, b []edu.Note) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Pitch != b[i].Pitch ||
			math.Abs(a[i].Start-b[i].Start) > 1e-6 || math.Abs(a[i].Length-b[i].Length) > 1e-6 {
			return false
		}
	}
	return true
}

// eduJamLoadFixtureCard 加载仓库夹具乐局卡（观察面单测直读用）。
func eduJamLoadFixtureCard(t *testing.T) *edu.LevelCard {
	t.Helper()
	card, err := edu.LoadLevelCard(eduJamTestLevelsDir + "/first-run-twinkle/level.yaml")
	if err != nil {
		t.Fatalf("load fixture level card: %v", err)
	}
	return card
}

// TestEduJamObserveAddressesUserTracksPastServiceTracks 是 281 修复腿 2 的配对
// **正锚**：真实内核轨树形状（5 服务轨前置+3 用户轨，smokerun2 实证）下，三个
// runtime_slots 槽必须按**过滤后**的 1 基用户轨号正确寻址——melody_imported
// (track_index=1)→t1(theme 全表)、melody_corrupted(2)→t2(损坏表，37 拍 68)、
// accompaniment_blank(3)→t3(无 clip=留白)。修复前（原始数组下标）本测必失败：
// track_index=1 落 tracks[0]=Arranger 轨无 clips→clip_exists=false（smokerun2_
// 20261107_1006 B2 失败面）。
func TestEduJamObserveAddressesUserTracksPastServiceTracks(t *testing.T) {
	theme := eduJamThemeNotes(t)
	corrupted := eduJamCorruptedNotes(t)
	fake := &eduJamFakeKernel{
		tracks: eduJamRealShapeTracks(
			eduJamFakeTrack("t1", eduJamFakeClip("c1", 26.0)),
			eduJamFakeTrack("t2", eduJamFakeClip("c2", 26.0)),
			eduJamFakeTrack("t3"),
		),
		notes: map[string][]any{
			"c1": eduJamNoteRows(theme),
			"c2": eduJamNoteRows(corrupted),
		},
	}
	server := newEduJamTestServer(t, fake)

	obs, _, err := server.observeEduJam(context.Background(), eduJamLoadFixtureCard(t))
	if err != nil {
		t.Fatalf("observeEduJam: %v", err)
	}
	if got := obs.Slots["melody_imported"]; !eduJamNotesEqual(got, theme) {
		t.Fatalf("melody_imported must address user track t1 (theme, %d notes), got %d notes", len(theme), len(got))
	}
	if got := obs.Slots["melody_corrupted"]; !eduJamNotesEqual(got, corrupted) {
		t.Fatalf("melody_corrupted must address user track t2 (corrupted table), got %d notes", len(got))
	}
	if _, observed := obs.Slots["accompaniment_blank"]; observed {
		t.Fatalf("accompaniment_blank (t3 without clips) must stay unobserved (clip_exists=false)")
	}
	if !obs.TrackNonEmpty["melody_imported"] || !obs.TrackNonEmpty["melody_corrupted"] || obs.TrackNonEmpty["accompaniment_blank"] {
		t.Fatalf("TrackNonEmpty wrong under real kernel track shape: %v", obs.TrackNonEmpty)
	}
}

// TestEduJamObserveNeverSelectsServiceTrackClips 是配对**负锚（修复前形态）**：
// 即使服务轨行带有 MIDI clip（修复前原始下标寻址会把 track_index=1 的槽观察落
// 到 tracks[0]=Arranger 的 clip 上——「clip_exists=true 但数据来自服务轨」的
// 错位形态），过滤后寻址也必须永不选中服务轨行。本测在过滤被移除/绕过时必
// 失败（观察到 svc 锚音=过滤失效；观察到空=整体寻址失效）。
func TestEduJamObserveNeverSelectsServiceTrackClips(t *testing.T) {
	theme := eduJamThemeNotes(t)
	// Arranger 服务轨携带 MIDI clip（真实内核服务轨无 clips，此为负锚加压形态）。
	tracks := eduJamServiceTracks()
	tracks[0] = map[string]any{
		"track_id": "1002", "track_name": "Arranger", "track_type": "track", "is_audio_track": false,
		"clips": []any{eduJamFakeClip("clip_svc_arranger", 1.0)},
	}
	fake := &eduJamFakeKernel{
		tracks: append(tracks, eduJamFakeTrack("t1", eduJamFakeClip("c1", 26.0))),
		notes: map[string][]any{
			"clip_svc_arranger": eduJamNoteRows([]edu.Note{{ID: "svc-arranger", Pitch: 1, Start: 0, Length: 1}}),
			"c1":                eduJamNoteRows(theme),
		},
	}
	server := newEduJamTestServer(t, fake)

	obs, _, err := server.observeEduJam(context.Background(), eduJamLoadFixtureCard(t))
	if err != nil {
		t.Fatalf("observeEduJam: %v", err)
	}
	if got := obs.Slots["melody_imported"]; !eduJamNotesEqual(got, theme) {
		t.Fatalf("melody_imported must address the user track t1 even when a service track carries a MIDI clip, got %d notes", len(got))
	}
	for slot, notes := range obs.Slots {
		for _, n := range notes {
			if n.ID == "svc-arranger" {
				t.Fatalf("service track clip leaked into slot observation: slot=%s note=%v", slot, n)
			}
		}
	}
}
