package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/experiment"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/trajectory"
)

func gapTestAdmission(mode experiment.AuthorityMode) experiment.Admission {
	return experiment.Admission{
		SchemaVersion: experiment.SchemaVersion, TargetRef: map[string]any{"kind": "track", "id": "vocal"}, EvidenceRefs: []string{"obs-1"},
		Hypothesis: "bounded treatment", TypedAction: map[string]any{"domain": "eq"}, DiagnosticDoseBounds: map[string]any{"max": 1}, RetainedDoseBounds: map[string]any{"max": 2},
		ExperimentBudget: 2, ExpectedEffect: "forwardness", VerificationPlan: map[string]any{"view_ids": []any{"track.timbre_frequency"}}, CheckpointRef: "checkpoint-1", RollbackPlan: map[string]any{"kind": "undo"}, AuthorityMode: mode,
	}
}

func TestAuthorityModeIsExplicitAndImmutableForActiveTurn(t *testing.T) {
	s := New(nil, shadow.New(nil), nil)
	bound, err := s.bindChatAuthorityMode(authorityModeFull, map[string]any{"conversation_id": "chat-authority"})
	if err != nil || firstStringFromMap(bound, "authority_mode") != authorityModeFull || !boolValue(bound["authority_mode_explicit"]) {
		t.Fatalf("bound=%+v err=%v", bound, err)
	}
	turn, err := experiment.NewTurn(experiment.Identity{ConversationID: "chat-authority"}, "test", gapTestAdmission(experiment.AuthorityFull), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	s.storeFreeStateLoop(freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, ConversationID: "chat-authority", LoopID: "loop-authority", OriginalIntent: "test", Status: "reasoning", AuthorityMode: experiment.AuthorityFull, Experiment: &turn})
	if _, err := s.bindChatAuthorityMode(authorityModeManual, map[string]any{"conversation_id": "chat-authority"}); err == nil {
		t.Fatal("active Turn authority mode changed")
	}
}

func TestHandleTurnStopMarksGoalStoppedAndEmitsTrajectory(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "Stop.vit")
	if err := os.WriteFile(projectPath, []byte("stable"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := shadow.New(nil)
	project.Initialize(map[string]any{"status": "ok", "project_path": projectPath, "project_uuid": "vitproj_stop"})
	s := New(nil, project, nil)
	s.activateCurrentProjectWorkspace(context.Background())
	goal := s.harness.BeginGoal("stop me")
	s.harness.SetGoalStatus(goal.GoalID, agentruntime.StatusWaitingContinue, nil)
	body, _ := json.Marshal(map[string]any{"conversation_id": "chat-stop", "goal_id": goal.GoalID, "run_id": goal.RunID, "turn_id": "turn-stop", "reason": "user_stop"})
	req := httptest.NewRequest(http.MethodPost, "/agent/turn/stop", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	s.Routes().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
	stopped := s.harness.RuntimeStatus(goal.GoalID)
	if stopped.Status != agentruntime.StatusStopped || stopped.LastCheckpoint == "" {
		t.Fatalf("goal=%+v", stopped)
	}
	events, _ := s.agentEventsSince("chat-stop", 0, 100)
	found := false
	for _, event := range events {
		if event.Type == string(trajectory.EventTurnStopped) {
			found = true
			if event.Payload["schema_version"] != trajectory.SchemaVersion {
				t.Fatalf("payload=%+v", event.Payload)
			}
		}
	}
	if !found {
		t.Fatalf("missing trajectory.turn.stopped events=%+v", events)
	}
}

func TestStopPreventsNewExperimentActivityAndPreservesCheckpointOnRuntimeRestore(t *testing.T) {
	s := New(nil, shadow.New(nil), nil)
	goal := s.harness.BeginGoal("experiment stop")
	turn, err := experiment.NewTurn(experiment.Identity{ConversationID: "chat-experiment-stop", GoalID: goal.GoalID, RunID: goal.RunID, TurnID: "turn-experiment-stop"}, "bounded", gapTestAdmission(experiment.AuthorityFull), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	s.authorityMode = authorityModeFull
	if _, err := turn.StartRound([]string{"track.timbre_frequency"}, "checkpoint-round", "rev-1", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	s.storeFreeStateLoop(freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, ConversationID: "chat-experiment-stop", LoopID: "loop-stop", GoalID: goal.GoalID, RunID: goal.RunID, OriginalIntent: "bounded", Status: "reasoning", AuthorityMode: experiment.AuthorityFull, Experiment: &turn})
	s.harness.SetGoalStatus(goal.GoalID, agentruntime.StatusWaitingContinue, nil)
	body, _ := json.Marshal(map[string]any{"conversation_id": "chat-experiment-stop", "goal_id": goal.GoalID, "run_id": goal.RunID, "turn_id": turn.ID})
	req := httptest.NewRequest(http.MethodPost, "/agent/turn/stop", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	s.Routes().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
	stopped, _ := s.freeStateLoop("chat-experiment-stop")
	if stopped.Status != "stopped" || stopped.Experiment == nil || stopped.Experiment.Status != experiment.StatusStopped {
		t.Fatalf("loop=%+v", stopped)
	}
	if _, err := stopped.Experiment.StartRound([]string{"track.transient"}, "checkpoint-2", "rev-2", time.Now().UTC()); err == nil {
		t.Fatal("stopped experiment accepted another round")
	}
	state := s.projectAgentRuntimeStateLocked()
	restarted := New(nil, shadow.New(nil), nil)
	restarted.restoreProjectAgentRuntimeStateLocked(state)
	if restarted.authorityMode != authorityModeFull {
		t.Fatalf("authority mode=%q", restarted.authorityMode)
	}
	restoredGoal := restarted.harness.RuntimeStatus(goal.GoalID)
	if restoredGoal.Status != agentruntime.StatusStopped {
		t.Fatalf("restored goal=%+v", restoredGoal)
	}
	restoredLoop, ok := restarted.freeStateLoop("chat-experiment-stop")
	if !ok || restoredLoop.Experiment == nil || restoredLoop.Experiment.Status != experiment.StatusStopped {
		t.Fatalf("restored loop=%+v ok=%v", restoredLoop, ok)
	}
}

func TestFullAccessRequestFlowsIntoExperimentAdmission(t *testing.T) {
	s := New(nil, shadow.New(nil), nil)
	ctx, err := s.bindChatAuthorityMode(authorityModeFull, map[string]any{"conversation_id": "chat-flow"})
	if err != nil {
		t.Fatal(err)
	}
	loop := freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-flow", ConversationID: "chat-flow", OriginalIntent: "improve", Status: "reasoning", AuthorityMode: authorityModeFromContext(ctx), LatestObservation: d1FreshObservationForTest("7")}
	proposal := experimentTestProposal()
	admission, err := freeStateExperimentAdmission(loop, proposal)
	if err != nil {
		t.Fatal(err)
	}
	if admission.AuthorityMode != experiment.AuthorityFull {
		t.Fatalf("admission authority=%q", admission.AuthorityMode)
	}
}

func TestAuthorityEndpointPersistsExplicitMode(t *testing.T) {
	s := New(nil, shadow.New(nil), nil)
	body := bytes.NewBufferString(`{"authority_mode":"full_project_access"}`)
	req := httptest.NewRequest(http.MethodPost, "/agent/authority", body)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	s.Routes().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || s.authorityMode != authorityModeFull {
		t.Fatalf("status=%d body=%s mode=%q", resp.Code, resp.Body.String(), s.authorityMode)
	}
	state := s.projectAgentRuntimeStateLocked()
	restarted := New(nil, shadow.New(nil), nil)
	restarted.restoreProjectAgentRuntimeStateLocked(state)
	if restarted.authorityMode != authorityModeFull {
		t.Fatalf("restored authority mode=%q", restarted.authorityMode)
	}
}

// AUTHORITY-LOST-1: 用户「启动后切换至完全访问，输入后运行却成了普通权限」的
// 命中形态之一——切换 POST 先于内核发布工程身份到达（workspace 未激活，
// activate 空转、persist 空转，磁盘无 full 记录），工程身份就绪后的第一次完整
// 激活从磁盘旧值 restore，把进程内显式切换吞掉。显式切换必须跨该激活存活。
func TestAuthoritySwitchBeforeWorkspaceIdentitySurvivesRestore(t *testing.T) {
	s := New(nil, shadow.New(nil), nil)
	switchBody := bytes.NewBufferString(`{"authority_mode":"full_project_access"}`)
	switchReq := httptest.NewRequest(http.MethodPost, "/agent/authority", switchBody)
	switchReq.Header.Set("Content-Type", "application/json")
	switchResp := httptest.NewRecorder()
	s.Routes().ServeHTTP(switchResp, switchReq)
	if switchResp.Code != http.StatusOK {
		t.Fatalf("switch status=%d body=%s", switchResp.Code, switchResp.Body.String())
	}
	projectPath := filepath.Join(t.TempDir(), "Late.vit")
	if err := os.WriteFile(projectPath, []byte("stable"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 工程身份随后就绪：下一个 GET 走完整激活路径，磁盘仍是旧 manual 形态。
	s.shadow.Initialize(map[string]any{"status": "ok", "project_path": projectPath, "project_uuid": "vitproj_late"})
	getResp := httptest.NewRecorder()
	s.Routes().ServeHTTP(getResp, httptest.NewRequest(http.MethodGet, "/agent/authority", nil))
	if getResp.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", getResp.Code, getResp.Body.String())
	}
	var payload struct {
		AuthorityMode string `json:"authority_mode"`
	}
	if err := json.Unmarshal(getResp.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.AuthorityMode != authorityModeFull {
		t.Fatalf("authority mode after late workspace activation=%q want %q", payload.AuthorityMode, authorityModeFull)
	}
	bound, err := s.bindChatAuthorityMode("", map[string]any{"conversation_id": "chat-late"})
	if err != nil || firstStringFromMap(bound, "authority_mode") != authorityModeFull {
		t.Fatalf("bound=%+v err=%v", bound, err)
	}
}

// AUTHORITY-LOST-1 第二命中面：切换已写入内存、persist 尚未落盘的竞态窗口内，
// continuation scheduler 的磁盘重放（reloadActiveRuntimeState）不得把显式
// 切换翻回磁盘旧值。bind 写内存与 POST 同源，覆盖 bind 翻转面。
func TestAuthoritySwitchHoldsAcrossSchedulerDiskReload(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "Reload.vit")
	if err := os.WriteFile(projectPath, []byte("stable"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := shadow.New(nil)
	project.Initialize(map[string]any{"status": "ok", "project_path": projectPath, "project_uuid": "vitproj_reload"})
	s := New(nil, project, nil)
	s.activateCurrentProjectWorkspace(context.Background())
	s.persistCurrentProjectWorkspace() // 磁盘停留 manual（切换前的权威形态）
	if _, err := s.bindChatAuthorityMode(authorityModeFull, map[string]any{"conversation_id": "chat-reload"}); err != nil {
		t.Fatal(err)
	}
	if err := s.reloadActiveRuntimeState(); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	mode := s.authorityMode
	s.mu.Unlock()
	if mode != authorityModeFull {
		t.Fatalf("authority mode after scheduler disk reload=%q want %q", mode, authorityModeFull)
	}
}

// 冻结语义零回退：goal 运行中切换必须保持 409 且内存模式不变。
func TestAuthorityModeChangeBlockedWhileGoalRunning(t *testing.T) {
	s := New(nil, shadow.New(nil), nil)
	goal := s.harness.BeginGoal("running turn")
	if status := s.harness.RuntimeStatus(goal.GoalID).Status; status != agentruntime.StatusRunning {
		t.Fatalf("goal status=%q want running", status)
	}
	body := bytes.NewBufferString(`{"authority_mode":"full_project_access"}`)
	req := httptest.NewRequest(http.MethodPost, "/agent/authority", body)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	s.Routes().ServeHTTP(resp, req)
	if resp.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "authority_mode_change_blocked") {
		t.Fatalf("missing error_code body=%s", resp.Body.String())
	}
	s.mu.Lock()
	mode := s.authorityMode
	s.mu.Unlock()
	if mode != authorityModeManual {
		t.Fatalf("authority mode=%q want manual", mode)
	}
}

// 显式权限是 per-workspace 状态：跨工程身份切换不得把工程 A 的 full 带进
// 工程 B（B 磁盘旧值 manual 仍权威）。
func TestAuthorityExplicitSwitchDoesNotLeakAcrossProjectIdentity(t *testing.T) {
	projectA := filepath.Join(t.TempDir(), "IdentA.vit")
	if err := os.WriteFile(projectA, []byte("stable"), 0o644); err != nil {
		t.Fatal(err)
	}
	projectB := filepath.Join(t.TempDir(), "IdentB.vit")
	if err := os.WriteFile(projectB, []byte("stable"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := New(nil, shadow.New(nil), nil)
	s.shadow.Initialize(map[string]any{"status": "ok", "project_path": projectA, "project_uuid": "vitproj_ident_a"})
	if resp := authorityPostForTest(s, authorityModeFull); resp.Code != http.StatusOK {
		t.Fatalf("switch status=%d body=%s", resp.Code, resp.Body.String())
	}
	if mode := authorityGetForTest(s); mode != authorityModeFull {
		t.Fatalf("project A authority=%q", mode)
	}
	s.shadow.Initialize(map[string]any{"status": "ok", "project_path": projectB, "project_uuid": "vitproj_ident_b"})
	if mode := authorityGetForTest(s); mode != authorityModeManual {
		t.Fatalf("project B authority=%q want manual (explicit mode must not leak across identity)", mode)
	}
}

func authorityPostForTest(s *Server, mode string) *httptest.ResponseRecorder {
	body := bytes.NewBufferString(`{"authority_mode":"` + mode + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/agent/authority", body)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	s.Routes().ServeHTTP(resp, req)
	return resp
}

func authorityGetForTest(s *Server) string {
	resp := httptest.NewRecorder()
	s.Routes().ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/agent/authority", nil))
	var payload struct {
		AuthorityMode string `json:"authority_mode"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		return ""
	}
	return payload.AuthorityMode
}
