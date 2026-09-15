package chat

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/trajectory"
)

// AUDITION-RESEAT-1 (2026-09-15): 现场形态是 UNSTICK-1 钉测没有走过的完整内核
// 状态机链路——turn 终局（waiting_confirmation）后 select A 成功、stop 成功、
// select B 被内核以 audition_not_ready 拒绝 14 次且 agent 侧零重落座。既有钉测
// 是把 loop 快照手工摆成 stopped 再发 select；本组测试复刻现场的真实事件链
// （prepare → 内核 ready 回声 → select A → 内核 select.changed 回声 → stop →
// 内核 stopped 回声 → select B），让失守形态在单元层可钉。
//
// statefulAuditionKernel 模拟内核 audition 状态机的可观察行为：
// ready --select--> playing --stop--> stopped；stopped 上的 select 返回与现场
// 相同的错误（audition.select requires a ready or playing session，
// code=audition_not_ready）；audition.prepare 用同一 session_id 整体重建会话
// （状态机唯一的复位路径）。
type statefulAuditionKernel struct {
	prepareRequests []kernel.AuditionSessionRequest
	selectCalls     []string
	// refuseSelect forces every AuditionSelect into this error (Kernel-style
	// error reply), independent of the tracked session state.
	refuseSelect string
	session      map[string]any
}

func (f *statefulAuditionKernel) sessionResult(status string) *kernel.VSPCommandResult {
	session := cloneContext(f.session)
	session["status"] = status
	return &kernel.VSPCommandResult{LegacyReply: map[string]any{"status": "ok", "session": session}}
}

func (f *statefulAuditionKernel) AuditionPrepare(_ context.Context, request kernel.AuditionSessionRequest) (*kernel.VSPCommandResult, error) {
	f.prepareRequests = append(f.prepareRequests, request)
	rows := make([]any, 0, len(request.Candidates))
	for _, candidate := range request.Candidates {
		row := map[string]any{"id": candidate.ID, "label": candidate.Label, "status": "ready"}
		if candidate.PreviewRef != "" {
			row["preview_ref"] = candidate.PreviewRef
		}
		if candidate.SourceRef != "" {
			row["source_ref"] = candidate.SourceRef
		}
		if candidate.SourceKind != "" {
			row["source_kind"] = candidate.SourceKind
		}
		rows = append(rows, row)
	}
	f.session = map[string]any{
		"session_id": request.SessionID, "conversation_id": request.ConversationID, "scope": request.Scope,
		"status": "ready", "active_project_plane": map[string]any{"plane": "active_project", "project_ref": request.ActiveProjectRef, "project_revision": request.ActiveProjectRevision},
		"transport": map[string]any{"timeline_revision": request.TimelineRevision, "is_playing": false},
		"candidates": rows,
	}
	return f.sessionResult("ready"), nil
}

func (f *statefulAuditionKernel) AuditionStatus(_ context.Context, _ string) (*kernel.VSPCommandResult, error) {
	if f.session == nil {
		return nil, nil
	}
	status := "ready"
	if value, ok := f.session["status"].(string); ok {
		status = value
	}
	return f.sessionResult(status), nil
}

func (f *statefulAuditionKernel) AuditionSelect(_ context.Context, sessionID, candidateID string) (*kernel.VSPCommandResult, error) {
	f.selectCalls = append(f.selectCalls, sessionID+":"+candidateID)
	if f.refuseSelect != "" {
		return &kernel.VSPCommandResult{LegacyReply: map[string]any{
			"status": "error", "error": f.refuseSelect, "code": "audition_not_ready",
		}}, nil
	}
	if value, ok := f.session["status"].(string); ok && value == "stopped" {
		return &kernel.VSPCommandResult{LegacyReply: map[string]any{
			"status": "error", "error": "audition.select requires a ready or playing session", "code": "audition_not_ready",
		}}, nil
	}
	f.session["status"] = "playing"
	f.session["active_candidate_id"] = candidateID
	return f.sessionResult("playing"), nil
}

func (f *statefulAuditionKernel) AuditionStop(_ context.Context, _ string) (*kernel.VSPCommandResult, error) {
	f.session["status"] = "stopped"
	delete(f.session, "active_candidate_id")
	return f.sessionResult("stopped"), nil
}

func postAuditionAction(server *Server, action, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/agent/audition/"+action, bytes.NewBufferString(body))
	recorder := httptest.NewRecorder()
	switch action {
	case "select":
		server.handleAuditionSelect(recorder, request)
	case "stop":
		server.handleAuditionStop(recorder, request)
	}
	return recorder
}

// TestStoppedSelectAfterLiveStopCycleReseats 复刻现场事件链：真实 prepare 链路
// 落座 → 内核 ready 回声 → select A（playing）→ 内核 select.changed 回声 →
// stop（stopped）→ 内核 stopped 回声 → select B。B 必须先经 audition.prepare
// 重落座再 select，而不是把内核的 stopped 拒绝透传给 UI。
func TestStoppedSelectAfterLiveStopCycleReseats(t *testing.T) {
	fake := &statefulAuditionKernel{}
	server := New(nil, nil, nil)
	server.auditionKernel = fake
	server.auditionCandidateDriver = newCandidateDriverForTest(auditionProjectPathForTest(t))
	loop := auditionReadyLoop(t)
	server.recordFreeStateExperimentDecision(context.Background(), &loop, agentloop.FreeStateDecision{ExperimentTargetResponse: &experiment.TargetEvaluation{Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady, EvidenceRefs: []string{"after"}}})
	if len(fake.prepareRequests) == 0 {
		t.Fatal("prepare chain never ran")
	}
	sessionID := fake.prepareRequests[0].SessionID
	conversationID := loop.ConversationID
	// 内核 telemetry 回声（现场 seq19/20 的第二回声路径）。
	server.HandleKernelTelemetry(map[string]any{"type": "audition.ready", "conversation_id": conversationID, "session": map[string]any{"session_id": sessionID, "conversation_id": conversationID, "status": "ready"}})

	// 判别锚点：现场 B 被拒时 auditionSnapshotStopped 三关哪一关失守。
	if loopRow, ok := server.freeStateLoop(conversationID); !ok {
		t.Logf("gate probe: freeStateLoop(%s) MISSING", conversationID)
	} else {
		t.Logf("gate probe: session_id=%q snapshot_status=%q", loopRow.AuditionSessionID, firstStringFromMap(loopRow.AuditionSessionSnapshot, "status"))
	}

	selectA := postAuditionAction(server, "select", `{"conversation_id":"`+conversationID+`","session_id":"`+sessionID+`","candidate_id":"candidate-a"}`)
	if selectA.Code != http.StatusOK {
		t.Fatalf("select A status=%d body=%s", selectA.Code, selectA.Body.String())
	}
	server.HandleKernelTelemetry(map[string]any{"type": "audition.select.changed", "conversation_id": conversationID, "session": map[string]any{"session_id": sessionID, "conversation_id": conversationID, "status": "playing", "active_candidate_id": "candidate-a"}})

	stop := postAuditionAction(server, "stop", `{"conversation_id":"`+conversationID+`","session_id":"`+sessionID+`"}`)
	if stop.Code != http.StatusOK {
		t.Fatalf("stop status=%d body=%s", stop.Code, stop.Body.String())
	}
	server.HandleKernelTelemetry(map[string]any{"type": "audition.stopped", "conversation_id": conversationID, "session": map[string]any{"session_id": sessionID, "conversation_id": conversationID, "status": "stopped"}})

	if loopRow, ok := server.freeStateLoop(conversationID); !ok {
		t.Logf("gate probe after stop: freeStateLoop(%s) MISSING", conversationID)
	} else {
		t.Logf("gate probe after stop: session_id=%q snapshot_status=%q stopped_gate=%v", loopRow.AuditionSessionID, firstStringFromMap(loopRow.AuditionSessionSnapshot, "status"), server.auditionSnapshotStopped(conversationID, sessionID))
	}

	selectB := postAuditionAction(server, "select", `{"conversation_id":"`+conversationID+`","session_id":"`+sessionID+`","candidate_id":"candidate-b"}`)
	if selectB.Code != http.StatusOK {
		t.Fatalf("stopped select B must re-seat and play, status=%d body=%s", selectB.Code, selectB.Body.String())
	}
	if len(fake.prepareRequests) < 2 {
		t.Fatalf("stopped select B must re-seat via audition.prepare first, prepare calls=%d", len(fake.prepareRequests))
	}
	if reseat := fake.prepareRequests[len(fake.prepareRequests)-1]; reseat.SessionID != sessionID {
		t.Fatalf("reseat must keep the same session identity: %+v", reseat)
	}
	events, _ := server.agentEventsSince(conversationID, 0, 100)
	recovered := false
	for _, event := range events {
		if (event.Type == "audition.prepare" || event.Type == "audition.ready") && event.Payload["recovered_from"] == "stopped" {
			recovered = true
		}
	}
	if !recovered {
		t.Fatalf("recovery transition not observable in events")
	}
}

// TestStoppedSelectAfterStaleSliceWriteBackReseats 钉现场形态（2026-09-15
// webui_mu228fc5）：turn 终局（waiting_confirmation，post-action 评估片在
// 调度队列）后 select A 成功、stop 成功，随后并行 turn/slice 用它开始时的
// loop 副本写回——prepareFreeStateReasoningContext（每个 resume 的入口）与
// recordFreeStateDecision（continuation/context merge）都会把副本里非空的旧
// audition 快照整体盖回 server loop，agent 快照从 stopped 回退到 ready，
// select B 的第一门 auditionSnapshotStopped 失守、裸 select 被内核拒绝并
// 透传成死锁。内核才是 stopped 的权威：裸 select 被内核以 not-ready 拒绝后
// 必须以内核 status 二次核对并重落座，与 agent 快照当时是什么无关。
func TestStoppedSelectAfterStaleSliceWriteBackReseats(t *testing.T) {
	fake := &statefulAuditionKernel{}
	logPath := filepath.Join(t.TempDir(), "agent.log")
	server := New(nil, nil, newTestLogger(logPath))
	server.auditionKernel = fake
	server.auditionCandidateDriver = newCandidateDriverForTest(auditionProjectPathForTest(t))
	loop := auditionReadyLoop(t)
	server.recordFreeStateExperimentDecision(context.Background(), &loop, agentloop.FreeStateDecision{ExperimentTargetResponse: &experiment.TargetEvaluation{Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady, EvidenceRefs: []string{"after"}}})
	if len(fake.prepareRequests) == 0 {
		t.Fatal("prepare chain never ran")
	}
	sessionID := fake.prepareRequests[0].SessionID
	conversationID := loop.ConversationID
	server.HandleKernelTelemetry(map[string]any{"type": "audition.ready", "conversation_id": conversationID, "session": map[string]any{"session_id": sessionID, "conversation_id": conversationID, "status": "ready"}})

	selectA := postAuditionAction(server, "select", `{"conversation_id":"`+conversationID+`","session_id":"`+sessionID+`","candidate_id":"candidate-a"}`)
	if selectA.Code != http.StatusOK {
		t.Fatalf("select A status=%d body=%s", selectA.Code, selectA.Body.String())
	}
	// slice 副本：并行 turn/slice 在 stop 之前绑定的 loop 形态（快照 playing）。
	staleLoop, ok := server.freeStateLoop(conversationID)
	if !ok {
		t.Fatal("loop missing before stale write-back")
	}
	stop := postAuditionAction(server, "stop", `{"conversation_id":"`+conversationID+`","session_id":"`+sessionID+`"}`)
	if stop.Code != http.StatusOK {
		t.Fatalf("stop status=%d body=%s", stop.Code, stop.Body.String())
	}
	if gate := server.auditionSnapshotStopped(conversationID, sessionID); !gate {
		t.Fatalf("snapshot must be stopped right after the live stop, gate=%v", gate)
	}

	// 晚到写回：slice resume 的真实入口把 stop 之前的副本盖回 server loop。
	server.prepareFreeStateReasoningContext(conversationID, "继续", map[string]any{
		"free_state_internal_resume":        true,
		"free_state_reasoning_loop": freeStateLoopMap(staleLoop),
	})
	if gate := server.auditionSnapshotStopped(conversationID, sessionID); gate {
		t.Fatalf("stale write-back must regress the snapshot for this field-shape pin (got gate still true)")
	}

	selectB := postAuditionAction(server, "select", `{"conversation_id":"`+conversationID+`","session_id":"`+sessionID+`","candidate_id":"candidate-b"}`)
	if selectB.Code != http.StatusOK {
		t.Fatalf("select B after a stale write-back must still play via the kernel-authoritative reseat, status=%d body=%s", selectB.Code, selectB.Body.String())
	}
	if len(fake.prepareRequests) < 2 {
		t.Fatalf("kernel-stopped select must re-seat via audition.prepare, prepare calls=%d", len(fake.prepareRequests))
	}
	if reseat := fake.prepareRequests[len(fake.prepareRequests)-1]; reseat.SessionID != sessionID {
		t.Fatalf("reseat must keep the same session identity: %+v", reseat)
	}
	// 日志锚点（AUDITION-RESEAT-1）：现场 14 次拒绝零日志不可再取证。被拒
	// 时的快照诊断、内核确认 stopped、重落座三段锚点各至少一行。
	logged := readTestLog(t, logPath)
	for _, anchor := range []string{
		"[audition] select refused while the agent snapshot said live",
		"gate=loop_present session_bound=true snapshot_status=\"playing\"",
		"[audition] kernel confirms stopped; reseating",
	} {
		if !strings.Contains(logged, anchor) {
			t.Fatalf("missing log anchor %q in agent log:\n%s", anchor, logged)
		}
	}
}

// TestRefusedSelectOnLiveKernelSessionSurfacesRawError：内核拒绝但内核自己
// 不确认 stopped（如 candidate 非法、session 未知）时，拒绝必须原样透传，
// 且带「不重落座」的日志锚点——这是拒绝路径的可取证边界。
func TestRefusedSelectOnLiveKernelSessionSurfacesRawError(t *testing.T) {
	fake := &statefulAuditionKernel{}
	fake.session = map[string]any{"session_id": "session-live", "conversation_id": "conversation-reseat", "scope": "target", "status": "ready"}
	logPath := filepath.Join(t.TempDir(), "agent.log")
	server := New(nil, nil, newTestLogger(logPath))
	server.auditionKernel = fake
	loop := auditionReadyLoop(t)
	loop.ConversationID = "conversation-reseat"
	loop.AuditionSessionID = "session-live"
	loop.AuditionSessionSnapshot = map[string]any{"session_id": "session-live", "status": "ready"}
	server.storeFreeStateLoop(loop)
	fake.refuseSelect = "candidate unknown"

	selectB := postAuditionAction(server, "select", `{"conversation_id":"conversation-reseat","session_id":"session-live","candidate_id":"candidate-b"}`)
	if selectB.Code != http.StatusConflict || !strings.Contains(selectB.Body.String(), "candidate unknown") {
		t.Fatalf("raw refusal must surface verbatim, status=%d body=%s", selectB.Code, selectB.Body.String())
	}
	if len(fake.prepareRequests) != 0 {
		t.Fatalf("a live kernel session must not be re-seated, prepare calls=%d", len(fake.prepareRequests))
	}
	logged := readTestLog(t, logPath)
	for _, anchor := range []string{
		"[audition] select refused while the agent snapshot said live",
		"[audition] kernel does not confirm stopped; surfacing the raw select refusal",
	} {
		if !strings.Contains(logged, anchor) {
			t.Fatalf("missing log anchor %q in agent log:\n%s", anchor, logged)
		}
	}
}
