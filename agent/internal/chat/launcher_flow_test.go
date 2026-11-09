package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/harness"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
)

// LAUNCHER-CHAT-BE-1（332 拆卡 C4）零栈单测：三分支表驱动 + 无工程态回归 +
// authority 不变断言（红线：零 authority 代码改动）+ 开场白端点 + state 披露。

type launcherFakeKernel struct {
	recents []string
	err     error
	calls   int
}

func (f *launcherFakeKernel) SendCommand(_ context.Context, cmd map[string]any) (map[string]any, string, error) {
	f.calls++
	if name := fmt.Sprint(cmd["cmd"]); name != "get_recent_projects" {
		return map[string]any{"status": "error", "error": "unexpected cmd " + name}, "", nil
	}
	if f.err != nil {
		return nil, "", f.err
	}
	arr := make([]any, len(f.recents))
	for i, path := range f.recents {
		arr[i] = path
	}
	return map[string]any{"status": "ok", "recent_projects": arr}, "", nil
}

func newLauncherTestServer(kernel *launcherFakeKernel, project *shadow.Project) *Server {
	s := testContinuationServer()
	s.harness = harness.NewWithSender(kernel, project, nil)
	return s
}

var launcherTestRecents = []string{
	"D:/VIT/projects/小星星乐局.vit",
	"D:/VIT/projects/鼓组混音v2.vit",
	"D:/VIT/projects/人声混音.vit",
	"D:/VIT/projects/ProQ3裸控实验.vit",
}

func launcherChatCtx() map[string]any {
	return map[string]any{"launcher": true}
}

// 三分支表驱动：唯一命中→directive；多命中→结果卡；无命中/意图不明→说明或
// 追问（或回落 LLM），一律不出指令。
func TestLauncherFlowChatThreeBranches(t *testing.T) {
	cases := []struct {
		name string
		// 输入
		message string
		// 期望
		wantHandled   bool
		wantDirective bool
		wantAction    string // directive.action（wantDirective 时必填）
		wantTarget    string // open=file_path / new=project_name
		wantCards     int
		replyContains []string
	}{
		{
			name: "唯一命中-名称模糊", message: "打开小星星",
			wantHandled: true, wantDirective: true, wantAction: "open_project",
			wantTarget:    "D:/VIT/projects/小星星乐局.vit",
			replyContains: []string{"小星星乐局"},
		},
		{
			name: "唯一命中-引号名归一（含空格）", message: "继续《鼓组混音 v2》",
			wantHandled: true, wantDirective: true, wantAction: "open_project",
			wantTarget:    "D:/VIT/projects/鼓组混音v2.vit",
			replyContains: []string{"鼓组混音v2"},
		},
		{
			name: "唯一命中-完整路径", message: "打开 D:/VIT/projects/ProQ3裸控实验.vit",
			wantHandled: true, wantDirective: true, wantAction: "open_project",
			wantTarget: "D:/VIT/projects/ProQ3裸控实验.vit",
		},
		{
			name: "唯一命中-把字句", message: "把小星星打开",
			wantHandled: true, wantDirective: true, wantAction: "open_project",
			wantTarget: "D:/VIT/projects/小星星乐局.vit",
		},
		{
			name: "多命中-结果卡不出指令", message: "打开混音",
			wantHandled: true, wantDirective: false, wantCards: 2,
			replyContains: []string{"有好几个"},
		},
		{
			name: "无命中-最近清单兜底", message: "打开不存在",
			wantHandled: true, wantDirective: false, wantCards: 4,
			replyContains: []string{"没找到"},
		},
		{
			name: "新建-带名", message: "新建一个叫测试小样的工程",
			wantHandled: true, wantDirective: true, wantAction: "new_project",
			wantTarget: "测试小样",
		},
		{
			name: "新建-无名追问", message: "新建工程",
			wantHandled: true, wantDirective: false,
			replyContains: []string{"名字"},
		},
		{
			name: "列表-问句", message: "我有哪些工程",
			wantHandled: true, wantDirective: false, wantCards: 4,
			replyContains: []string{"最近的工程有 4 个"},
		},
		{
			name: "意图不明-闲聊回落LLM", message: "随便聊聊今天的心得",
			wantHandled: false,
		},
		{
			name: "软动词零命中-回落LLM", message: "继续聊聊刚才的话题",
			wantHandled: false,
		},
		{
			name: "疑问口吻-回落LLM", message: "怎么打开工程文件",
			wantHandled: false,
		},
		{
			name: "许可问句-回落LLM", message: "能打开小星星吗",
			wantHandled: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newLauncherTestServer(&launcherFakeKernel{recents: launcherTestRecents}, nil)
			resp, handled := s.handleLauncherFlowChat(context.Background(), launcherChatCtx(), "conv-l", tc.message, "default")
			if handled != tc.wantHandled {
				t.Fatalf("handled=%v want %v (reply=%q)", handled, tc.wantHandled, resp.Reply)
			}
			if !tc.wantHandled {
				return
			}
			if strings.TrimSpace(resp.Reply) == "" {
				t.Fatal("handled turn must carry a deterministic reply")
			}
			for _, want := range tc.replyContains {
				if !strings.Contains(resp.Reply, want) {
					t.Fatalf("reply %q missing %q", resp.Reply, want)
				}
			}
			if (resp.LauncherDirective != nil) != tc.wantDirective {
				t.Fatalf("directive presence=%v want %v (resp=%+v)", resp.LauncherDirective != nil, tc.wantDirective, resp.LauncherDirective)
			}
			if tc.wantDirective {
				d := resp.LauncherDirective
				if d.Action != tc.wantAction {
					t.Fatalf("directive action=%q want %q", d.Action, tc.wantAction)
				}
				if !d.RequiresGovernance {
					t.Fatal("directive requires_governance must always be true (UI 自查双保险)")
				}
				got := d.FilePath
				if d.Action == "new_project" {
					got = d.ProjectName
					if d.FilePath != "" {
						t.Fatal("new_project directive must not carry file_path")
					}
				} else if d.ProjectName != "" || strings.TrimSpace(d.FilePath) == "" {
					t.Fatal("open_project directive must carry file_path only")
				}
				if got != tc.wantTarget {
					t.Fatalf("directive target=%q want %q", got, tc.wantTarget)
				}
			}
			if len(resp.ProjectResultCards) != tc.wantCards {
				t.Fatalf("cards=%d want %d", len(resp.ProjectResultCards), tc.wantCards)
			}
			for _, card := range resp.ProjectResultCards {
				if kind := fmt.Sprint(card["kind"]); kind != "launcher_project" {
					t.Fatalf("card kind=%q want launcher_project", kind)
				}
				inner, _ := card["launcher_directive"].(*LauncherDirective)
				if inner == nil || inner.Action != "open_project" || inner.FilePath == "" || !inner.RequiresGovernance {
					t.Fatalf("card must embed an executable open directive, got %+v", inner)
				}
			}
			if resp.NeedsConfirmation {
				t.Fatal("launcher deterministic turns never open a confirmation face (治理确认在 UI 层)")
			}
		})
	}
}

// 裸继续（无引用）：0/1/N 最近工程 → 说明/唯一 directive/结果卡。
func TestLauncherFlowBareContinue(t *testing.T) {
	t.Run("零最近-说明", func(t *testing.T) {
		s := newLauncherTestServer(&launcherFakeKernel{}, nil)
		resp, handled := s.handleLauncherFlowChat(context.Background(), launcherChatCtx(), "c", "继续", "default")
		if !handled || resp.LauncherDirective != nil || len(resp.ProjectResultCards) != 0 {
			t.Fatalf("want handled explanation, got handled=%v directive=%v cards=%d", handled, resp.LauncherDirective, len(resp.ProjectResultCards))
		}
		if !strings.Contains(resp.Reply, "还没有") {
			t.Fatalf("reply %q", resp.Reply)
		}
	})
	t.Run("唯一最近-directive", func(t *testing.T) {
		s := newLauncherTestServer(&launcherFakeKernel{recents: []string{"D:/a/唯一工程.vit"}}, nil)
		resp, handled := s.handleLauncherFlowChat(context.Background(), launcherChatCtx(), "c", "继续上次那个工程", "default")
		if !handled || resp.LauncherDirective == nil || resp.LauncherDirective.FilePath != "D:/a/唯一工程.vit" {
			t.Fatalf("want unique directive, got %+v handled=%v", resp.LauncherDirective, handled)
		}
	})
	t.Run("多个最近-结果卡", func(t *testing.T) {
		s := newLauncherTestServer(&launcherFakeKernel{recents: launcherTestRecents}, nil)
		resp, handled := s.handleLauncherFlowChat(context.Background(), launcherChatCtx(), "c", "继续", "default")
		if !handled || resp.LauncherDirective != nil || len(resp.ProjectResultCards) != 4 {
			t.Fatalf("want cards, got directive=%v cards=%d handled=%v", resp.LauncherDirective, len(resp.ProjectResultCards), handled)
		}
	})
}

// 内核不可达：确定性降级说明（零 LLM 依赖），不出指令。
func TestLauncherFlowKernelUnavailable(t *testing.T) {
	s := newLauncherTestServer(&launcherFakeKernel{err: errors.New("kernel down")}, nil)
	resp, handled := s.handleLauncherFlowChat(context.Background(), launcherChatCtx(), "c", "打开小星星", "default")
	if !handled || resp.LauncherDirective != nil || len(resp.ProjectResultCards) != 0 {
		t.Fatalf("want graceful explanation, got handled=%v directive=%v", handled, resp.LauncherDirective)
	}
	if !strings.Contains(resp.Reply, "连不上内核") {
		t.Fatalf("reply %q", resp.Reply)
	}
}

// 无工程态回归（F11）：无 launcher 标记的普通 chat 不被本分支拦截——语义
// 不变，照旧走既有 LLM 轮；launcher 分支只在显式 launcher:true 且无活跃工程
// 时生效。
func TestLauncherFlowRequiresFlagAndNoProject(t *testing.T) {
	s := newLauncherTestServer(&launcherFakeKernel{recents: launcherTestRecents}, nil)
	ctx := context.Background()

	if _, handled := s.handleLauncherFlowChat(ctx, nil, "c", "打开小星星", "default"); handled {
		t.Fatal("no-context chat must not be intercepted (F11 无工程 chat 语义不变)")
	}
	if _, handled := s.handleLauncherFlowChat(ctx, map[string]any{"launcher": false}, "c", "打开小星星", "default"); handled {
		t.Fatal("launcher=false must not be intercepted")
	}

	// 有活跃工程（态 C）：跨工程意图归 C5 治理链，本分支必须让路。
	project := shadow.New(nil)
	project.Initialize(map[string]any{"status": "ok", "project_path": "D:/a/在开工程.vit", "project_uuid": "vitproj_bound"})
	bound := newLauncherTestServer(&launcherFakeKernel{recents: launcherTestRecents}, project)
	if _, handled := bound.handleLauncherFlowChat(ctx, launcherChatCtx(), "c", "打开小星星", "default"); handled {
		t.Fatal("project-bound chat must not route through the launcher branch (归 C5)")
	}
}

// 红线断言：launcher:true 不改 authority_mode——服务端 authority 状态是唯一
// 真源，launcher 流全程零 authority 读写；full 权限声明在 launcher 上下文里
// 照旧被拒（请求体永远不能授予）。
func TestLauncherFlowDoesNotChangeAuthorityMode(t *testing.T) {
	s := newLauncherTestServer(&launcherFakeKernel{recents: launcherTestRecents}, nil)
	before := s.authorityModeSnapshot()
	if before != authorityModeManual {
		t.Fatalf("fresh server authority mode=%q want %q", before, authorityModeManual)
	}

	resp, handled := s.handleLauncherFlowChat(context.Background(), launcherChatCtx(), "conv-auth", "打开小星星", "default")
	if !handled || resp.LauncherDirective == nil {
		t.Fatalf("want served launcher turn, handled=%v directive=%v", handled, resp.LauncherDirective)
	}
	if after := s.authorityModeSnapshot(); after != before {
		t.Fatalf("launcher turn changed authority mode: %q -> %q", before, after)
	}

	// full 声明在 launcher 上下文仍被拒：launcher 标记不构成任何授权路径。
	if _, err := s.bindChatAuthorityMode(authorityModeFull, launcherChatCtx()); err == nil {
		t.Fatal("full_project_access claim must stay rejected in a launcher context")
	}
	// 无声明的 launcher 轮照旧按服务端状态绑定 manual（上下文只读复述）。
	bound, err := s.bindChatAuthorityMode("", launcherChatCtx())
	if err != nil {
		t.Fatalf("bind with launcher ctx: %v", err)
	}
	if mode := firstStringFromMap(bound, "authority_mode"); mode != authorityModeManual {
		t.Fatalf("bound authority mode=%q want %q", mode, authorityModeManual)
	}
}

// 门厅简版开场白：确定性模板（在场问候+最近工程一条+「想从哪开始？」），零
// LLM；无最近/内核不可达时降级模板；非 GET 拒绝。
func TestLauncherGreetingEndpoint(t *testing.T) {
	t.Run("有最近工程", func(t *testing.T) {
		s := newLauncherTestServer(&launcherFakeKernel{recents: launcherTestRecents}, nil)
		rec := httptest.NewRecorder()
		s.handleLauncherGreeting(rec, httptest.NewRequest(http.MethodGet, "/agent/launcher/greeting", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var payload struct {
			Status string `json:"status"`
			Reply  string `json:"reply"`
			Recent struct {
				Name string `json:"name"`
				Path string `json:"file_path"`
			} `json:"recent_project"`
			Count int `json:"recent_count"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Reply != "我在。最近的工程是《小星星乐局》——想从哪开始？" {
			t.Fatalf("greeting template drift: %q", payload.Reply)
		}
		if payload.Recent.Name != "小星星乐局" || payload.Recent.Path != "D:/VIT/projects/小星星乐局.vit" || payload.Count != 4 {
			t.Fatalf("recent payload: %+v", payload)
		}
	})
	t.Run("零最近-降级模板", func(t *testing.T) {
		s := newLauncherTestServer(&launcherFakeKernel{}, nil)
		rec := httptest.NewRecorder()
		s.handleLauncherGreeting(rec, httptest.NewRequest(http.MethodGet, "/agent/launcher/greeting", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "想从哪开始") {
			t.Fatalf("fallback greeting body: %s", rec.Body.String())
		}
	})
	t.Run("内核不可达-降级模板", func(t *testing.T) {
		s := newLauncherTestServer(&launcherFakeKernel{err: errors.New("down")}, nil)
		rec := httptest.NewRecorder()
		s.handleLauncherGreeting(rec, httptest.NewRequest(http.MethodGet, "/agent/launcher/greeting", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "还没有最近的工程记录") {
			t.Fatalf("degraded greeting body: %s", rec.Body.String())
		}
	})
	t.Run("非GET拒绝", func(t *testing.T) {
		s := newLauncherTestServer(&launcherFakeKernel{}, nil)
		rec := httptest.NewRecorder()
		s.handleLauncherGreeting(rec, httptest.NewRequest(http.MethodPost, "/agent/launcher/greeting", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

// HTTP 面端到端：POST /agent/chat（context.launcher=true，无活跃工程）走完整
// handleChat→writeChat 管线——回执 JSON 带序列化的 launcher_directive、goal
// 完成、authority 不变，且无工程态不被物化成 default draft（F11）。
func TestLauncherFlowChatHTTPFace(t *testing.T) {
	shadowProject := shadow.New(nil)
	server := New(nil, shadowProject, nil)
	server.harness = harness.NewWithSender(&launcherFakeKernel{recents: launcherTestRecents}, shadowProject, nil)
	authorityBefore := server.authorityModeSnapshot()

	body, _ := json.Marshal(ChatRequest{
		ConversationID: "chat_launcher_http",
		Message:        "打开小星星",
		Context:        map[string]any{"launcher": true, "agent_mode": agentModeDefault},
	})
	rec := httptest.NewRecorder()
	server.handleChat(rec, httptest.NewRequest(http.MethodPost, "/agent/chat", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	for _, want := range []string{`"launcher_directive"`, `"open_project"`, `"requires_governance":true`} {
		if !strings.Contains(raw, want) {
			t.Fatalf("response JSON missing %q: %s", want, raw)
		}
	}
	var resp ChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.LauncherDirective == nil || resp.LauncherDirective.Action != "open_project" ||
		resp.LauncherDirective.FilePath != "D:/VIT/projects/小星星乐局.vit" {
		t.Fatalf("directive=%+v", resp.LauncherDirective)
	}
	if resp.GoalStatus != string(agentruntime.StatusCompleted) {
		t.Fatalf("goal status=%q want completed", resp.GoalStatus)
	}
	if resp.NeedsConfirmation {
		t.Fatal("launcher turn must not open a confirmation face")
	}
	if after := server.authorityModeSnapshot(); after != authorityBefore {
		t.Fatalf("authority mode changed over launcher turn: %q -> %q", authorityBefore, after)
	}
	if path, uuid := server.harness.CurrentProjectIdentity(context.Background()); path != "" || uuid != "" {
		t.Fatalf("launcher turn must not materialize a project identity, got path=%q uuid=%q", path, uuid)
	}
}

// /agent/state 两面（/agent/state 与 /agent/runtime/status）披露
// current_project{path,uuid}，与 note_sessions 同源；无工程=空字符串字段。
func TestAgentStateDisclosesCurrentProject(t *testing.T) {
	decodeCurrentProject := func(body string) map[string]string {
		var payload struct {
			CurrentProject map[string]string `json:"current_project"`
		}
		if err := json.Unmarshal([]byte(body), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.CurrentProject == nil {
			t.Fatalf("current_project missing in %s", body)
		}
		return payload.CurrentProject
	}
	t.Run("无工程-两面空值", func(t *testing.T) {
		s := newLauncherTestServer(&launcherFakeKernel{}, nil)
		stateRec := httptest.NewRecorder()
		s.handleState(stateRec, httptest.NewRequest(http.MethodGet, "/agent/state", nil))
		if stateRec.Code != http.StatusOK {
			t.Fatalf("state status=%d", stateRec.Code)
		}
		cp := decodeCurrentProject(stateRec.Body.String())
		if cp["path"] != "" || cp["uuid"] != "" {
			t.Fatalf("no-project current_project must be empty, got %v", cp)
		}
		runtimeRec := httptest.NewRecorder()
		s.handleRuntimeStatus(runtimeRec, httptest.NewRequest(http.MethodGet, "/agent/runtime/status", nil))
		if runtimeRec.Code != http.StatusOK {
			t.Fatalf("runtime status=%d", runtimeRec.Code)
		}
		cp = decodeCurrentProject(runtimeRec.Body.String())
		if cp["path"] != "" || cp["uuid"] != "" {
			t.Fatalf("no-project runtime current_project must be empty, got %v", cp)
		}
	})
	t.Run("有工程-两面同源", func(t *testing.T) {
		project := shadow.New(nil)
		project.Initialize(map[string]any{"status": "ok", "project_path": "D:/a/在开工程.vit", "project_uuid": "vitproj_state"})
		s := newLauncherTestServer(&launcherFakeKernel{}, project)
		stateRec := httptest.NewRecorder()
		s.handleState(stateRec, httptest.NewRequest(http.MethodGet, "/agent/state", nil))
		cp := decodeCurrentProject(stateRec.Body.String())
		if cp["path"] != "D:/a/在开工程.vit" || cp["uuid"] != "vitproj_state" {
			t.Fatalf("state current_project=%v", cp)
		}
		runtimeRec := httptest.NewRecorder()
		s.handleRuntimeStatus(runtimeRec, httptest.NewRequest(http.MethodGet, "/agent/runtime/status", nil))
		cp = decodeCurrentProject(runtimeRec.Body.String())
		if cp["path"] != "D:/a/在开工程.vit" || cp["uuid"] != "vitproj_state" {
			t.Fatalf("runtime current_project=%v", cp)
		}
	})
}

// ── LAUNCHER-BRIDGE-BE-1（332 拆卡 C5 / 338）零栈单测 ────────────────────────
// 流标记三态 + 桥接播报两分支（无 binding=确定性摘要；有 binding=315 既有机制
// 承接的衔接验证）+ directive 自然语确认句同发回归。全部 t.TempDir 落盘，零
// LLM 零真栈。

// launcherBridgeBindProject 在 t.TempDir 造真实工程文件（可控 mtime）并绑定为
// 活跃工作区（eduJam 测试同款 s.mu 直设先例）。
func launcherBridgeBindProject(t *testing.T, s *Server, name string, modTime time.Time) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name+".vit")
	if err := os.WriteFile(path, []byte("bridge-test-project"), 0o644); err != nil {
		t.Fatalf("write fake project file: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	s.mu.Lock()
	s.activeWorkspacePath = path
	s.activeWorkspaceUUID = "vitproj_bridge_" + name
	s.mu.Unlock()
	return path
}

// 流标记三态：launcher（launcher 分支回执自标）/ project（工程绑定态默认标）/
// 空（无绑定无标记回执，omitempty 序列化缺席=零回归）。
func TestLauncherContextFlowThreeStates(t *testing.T) {
	resetLauncherBridgeStateForTest()
	t.Cleanup(resetLauncherBridgeStateForTest)
	ctx := context.Background()

	t.Run("态1-launcher分支自标", func(t *testing.T) {
		s := newLauncherTestServer(&launcherFakeKernel{recents: launcherTestRecents}, nil)
		for _, msg := range []string{"打开小星星", "我有哪些工程", "新建工程"} {
			resp, handled := s.handleLauncherFlowChat(ctx, launcherChatCtx(), "conv-flow-l", msg, "default")
			if !handled {
				t.Fatalf("launcher turn %q not handled", msg)
			}
			if resp.ContextFlow != launcherFlowLauncher {
				t.Fatalf("launcher turn %q context_flow=%q want %q", msg, resp.ContextFlow, launcherFlowLauncher)
			}
		}
	})
	t.Run("态2-工程绑定态默认标project且不自标覆盖", func(t *testing.T) {
		s := newLauncherTestServer(&launcherFakeKernel{}, nil)
		launcherBridgeBindProject(t, s, "在开工程", time.Now())
		resp := &ChatResponse{Reply: "普通工程轮"}
		s.applyChatContextFlowMark(resp)
		if resp.ContextFlow != launcherFlowProject {
			t.Fatalf("bound turn context_flow=%q want %q", resp.ContextFlow, launcherFlowProject)
		}
		marked := &ChatResponse{ContextFlow: launcherFlowLauncher}
		s.applyChatContextFlowMark(marked)
		if marked.ContextFlow != launcherFlowLauncher {
			t.Fatalf("launcher self-mark must be preserved, got %q", marked.ContextFlow)
		}
	})
	t.Run("态3-未绑定保持空值且JSON缺席", func(t *testing.T) {
		s := newLauncherTestServer(&launcherFakeKernel{}, nil)
		resp := &ChatResponse{Reply: "无工程普通轮"}
		s.applyChatContextFlowMark(resp)
		if resp.ContextFlow != "" {
			t.Fatalf("unbound turn context_flow=%q want empty", resp.ContextFlow)
		}
		raw, err := json.Marshal(resp)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "context_flow") {
			t.Fatalf("empty context_flow must stay absent in JSON (omitempty 零回归): %s", raw)
		}
	})
	t.Run("HTTP面-launcher回执带流标记", func(t *testing.T) {
		shadowProject := shadow.New(nil)
		server := New(nil, shadowProject, nil)
		server.harness = harness.NewWithSender(&launcherFakeKernel{recents: launcherTestRecents}, shadowProject, nil)
		body, _ := json.Marshal(ChatRequest{
			ConversationID: "chat_launcher_flow_http",
			Message:        "打开小星星",
			Context:        map[string]any{"launcher": true, "agent_mode": agentModeDefault},
		})
		rec := httptest.NewRecorder()
		server.handleChat(rec, httptest.NewRequest(http.MethodPost, "/agent/chat", bytes.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var resp ChatResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if resp.ContextFlow != launcherFlowLauncher {
			t.Fatalf("HTTP launcher receipt context_flow=%q want %q (body=%s)", resp.ContextFlow, launcherFlowLauncher, rec.Body.String())
		}
	})
}

// 桥接播报·无 binding 分支：launcher 起源会话进工程后首轮命中确定性摘要
//（工程名+最近修改时间，零 LLM）+卷切换 typed_event；幂等/守卫/fail-soft 全断言。
func TestLauncherBridgeBroadcastNoBindingProject(t *testing.T) {
	resetLauncherBridgeStateForTest()
	t.Cleanup(resetLauncherBridgeStateForTest)
	ctx := context.Background()

	newBoundServer := func(t *testing.T, conv string) (*Server, string) {
		t.Helper()
		s := newLauncherTestServer(&launcherFakeKernel{recents: launcherTestRecents}, nil)
		// 步 1：launcher 卷轮（标记会话起源；此刻无工程，分支正常服务）。
		if _, handled := s.handleLauncherFlowChat(ctx, launcherChatCtx(), conv, "打开小星星", "default"); !handled {
			t.Fatalf("launcher turn not handled")
		}
		// 步 2：模拟 UI 执行 directive 后 activation 绑定工程（无 binding sidecar）。
		path := launcherBridgeBindProject(t, s, "桥接乐局", time.Date(2026, 11, 8, 21, 14, 0, 0, time.UTC))
		return s, path
	}

	t.Run("首轮播报-摘要+typed_event", func(t *testing.T) {
		s, path := newBoundServer(t, "conv-bridge")
		resp, served := s.launcherBridgeBroadcastCheckpoint(ctx, "conv-bridge", "chat", "把主推子拉低一点")
		if !served {
			t.Fatal("bridge broadcast must serve the first turn after launcher-origin entry")
		}
		if !strings.Contains(resp.Reply, "已进入《桥接乐局》") {
			t.Fatalf("reply missing project name template: %q", resp.Reply)
		}
		if !strings.Contains(resp.Reply, "最近修改于") || !regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}`).MatchString(resp.Reply) {
			t.Fatalf("reply missing deterministic mtime: %q", resp.Reply)
		}
		if resp.ContextFlow != launcherFlowProject || resp.StopReason != launcherBridgeStopReason {
			t.Fatalf("flow=%q stop=%q", resp.ContextFlow, resp.StopReason)
		}
		if len(resp.TypedEvents) != 1 {
			t.Fatalf("typed_events=%d want 1", len(resp.TypedEvents))
		}
		event := resp.TypedEvents[0]
		if event["type"] != "volume_switch" || event["context_flow"] != launcherFlowProject {
			t.Fatalf("volume_switch event malformed: %v", event)
		}
		if event["project_name"] != "桥接乐局" || event["project_path"] != path {
			t.Fatalf("volume_switch event identity mismatch: %v", event)
		}
		if resp.WorkflowData["context_flow"] != launcherFlowProject {
			t.Fatalf("workflow_data context_flow=%v", resp.WorkflowData["context_flow"])
		}
	})
	t.Run("会话级幂等-第二轮不播报", func(t *testing.T) {
		s, _ := newBoundServer(t, "conv-bridge-once")
		if _, served := s.launcherBridgeBroadcastCheckpoint(ctx, "conv-bridge-once", "chat", "第一条"); !served {
			t.Fatal("first turn must serve")
		}
		if _, served := s.launcherBridgeBroadcastCheckpoint(ctx, "conv-bridge-once", "chat", "第二条"); served {
			t.Fatal("second turn must not re-serve (会话级一次性)")
		}
	})
	t.Run("非launcher起源会话-零差异", func(t *testing.T) {
		s, _ := newBoundServer(t, "conv-launcher") // 标记的是 conv-launcher，非 conv-direct
		if _, served := s.launcherBridgeBroadcastCheckpoint(ctx, "conv-direct", "chat", "在吗"); served {
			t.Fatal("non-launcher-origin conversation must stay untouched (普通工程进入零差异)")
		}
	})
	t.Run("未绑定-不播报", func(t *testing.T) {
		s := newLauncherTestServer(&launcherFakeKernel{}, nil)
		if _, handled := s.handleLauncherFlowChat(ctx, launcherChatCtx(), "conv-unbound", "打开小星星", "default"); !handled {
			t.Fatal("launcher turn not handled")
		}
		if _, served := s.launcherBridgeBroadcastCheckpoint(ctx, "conv-unbound", "chat", "在吗"); served {
			t.Fatal("unbound state must not broadcast (仍在 launcher 卷)")
		}
	})
	t.Run("斜杠与命令词-放行且不消耗一次性名额", func(t *testing.T) {
		s, _ := newBoundServer(t, "conv-bridge-cmd")
		for _, msg := range []string{"/status", "开始乐局"} {
			if _, served := s.launcherBridgeBroadcastCheckpoint(ctx, "conv-bridge-cmd", "chat", msg); served {
				t.Fatalf("guard message %q must pass through", msg)
			}
		}
		if _, served := s.launcherBridgeBroadcastCheckpoint(ctx, "conv-bridge-cmd", "chat", "普通第一句"); !served {
			t.Fatal("guard pass-through must not consume the once slot")
		}
	})
	t.Run("stat失败-failsoft模板", func(t *testing.T) {
		s := newLauncherTestServer(&launcherFakeKernel{}, nil)
		if _, handled := s.handleLauncherFlowChat(ctx, launcherChatCtx(), "conv-bridge-soft", "打开小星星", "default"); !handled {
			t.Fatal("launcher turn not handled")
		}
		s.mu.Lock()
		s.activeWorkspacePath = filepath.Join(t.TempDir(), "不存在工程.vit")
		s.activeWorkspaceUUID = "vitproj_bridge_missing"
		s.mu.Unlock()
		resp, served := s.launcherBridgeBroadcastCheckpoint(ctx, "conv-bridge-soft", "chat", "在吗")
		if !served {
			t.Fatal("stat failure must degrade, not suppress the broadcast")
		}
		if !strings.Contains(resp.Reply, "《不存在工程》") || !strings.Contains(resp.Reply, "没读到最近修改时间") {
			t.Fatalf("fail-soft template drift: %q", resp.Reply)
		}
	})
	t.Run("HTTP面-全管线桥接回执", func(t *testing.T) {
		shadowProject := shadow.New(nil)
		server := New(nil, shadowProject, nil)
		server.harness = harness.NewWithSender(&launcherFakeKernel{recents: launcherTestRecents}, shadowProject, nil)
		// 步 1：launcher 卷轮（HTTP，标记会话起源）。
		body, _ := json.Marshal(ChatRequest{
			ConversationID: "chat_bridge_http",
			Message:        "打开小星星",
			Context:        map[string]any{"launcher": true, "agent_mode": agentModeDefault},
		})
		rec := httptest.NewRecorder()
		server.handleChat(rec, httptest.NewRequest(http.MethodPost, "/agent/chat", bytes.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("launcher turn status=%d body=%s", rec.Code, rec.Body.String())
		}
		// 步 2：模拟进工程绑定（直设活跃工作区；无 binding sidecar）。
		path := launcherBridgeBindProject(t, server, "桥接HTTP", time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC))
		// 步 3：工程卷首轮（普通 context）→ 桥接播报经 writeChat 全链。
		body2, _ := json.Marshal(ChatRequest{
			ConversationID: "chat_bridge_http",
			Message:        "在吗",
			Context:        map[string]any{"agent_mode": agentModeDefault},
		})
		rec2 := httptest.NewRecorder()
		server.handleChat(rec2, httptest.NewRequest(http.MethodPost, "/agent/chat", bytes.NewReader(body2)))
		if rec2.Code != http.StatusOK {
			t.Fatalf("bridge turn status=%d body=%s", rec2.Code, rec2.Body.String())
		}
		raw := rec2.Body.String()
		for _, want := range []string{`"context_flow":"project"`, `"type":"volume_switch"`, "已进入《桥接HTTP》", `"stop_reason":"launcher_bridge_broadcast"`} {
			if !strings.Contains(raw, want) {
				t.Fatalf("bridge receipt missing %q: %s", want, raw)
			}
		}
		// 工程身份走结构化断言（Windows 路径在 JSON 中反斜杠转义，不做字符串直比）。
		var receipt struct {
			WorkflowData map[string]any `json:"workflow_data"`
			TypedEvents  []map[string]any `json:"typed_events"`
		}
		if err := json.Unmarshal(rec2.Body.Bytes(), &receipt); err != nil {
			t.Fatal(err)
		}
		if receipt.WorkflowData["project_path"] != path || receipt.WorkflowData["project_uuid"] != "vitproj_bridge_桥接HTTP" {
			t.Fatalf("workflow_data identity mismatch: %v (want path %s)", receipt.WorkflowData, path)
		}
		if len(receipt.TypedEvents) != 1 || receipt.TypedEvents[0]["project_path"] != path || receipt.TypedEvents[0]["type"] != "volume_switch" {
			t.Fatalf("typed_events identity mismatch: %v", receipt.TypedEvents)
		}
	})
}

// 桥接播报·有 binding 分支（315 衔接验证，零重写）：launcher 起源会话经
// directive 进 binding 工程后，315 自动开场/恢复播报既有机制照常承接；本卡
// 桥接检查点对 binding 工程 fail-closed 让位，零干预。
func TestLauncherBridgeBindingProjectHandsOffToEduJam(t *testing.T) {
	resetLauncherBridgeStateForTest()
	t.Cleanup(resetLauncherBridgeStateForTest)
	ctx := context.Background()
	seedTracks := eduJamRealShapeTracks(eduJamFakeTrack("t1"), eduJamFakeTrack("t2"), eduJamFakeTrack("t3"))

	// 沿用 315 测试基建（edu_jam_intro_test.go 同款：假内核+仓库夹具乐局卡+
	// t.TempDir 伪工程旁真 sidecar），只加 harness 以驱动 launcher 卷轮。
	newBridgeEduServer := func(t *testing.T) *Server {
		t.Helper()
		server := newEduJamTestServer(t, &eduJamFakeKernel{tracks: seedTracks, notes: map[string][]any{}})
		server.harness = harness.NewWithSender(&launcherFakeKernel{recents: launcherTestRecents}, nil, nil)
		return server
	}
	launcherOriginThenBind := func(t *testing.T, server *Server) string {
		t.Helper()
		if _, handled := server.handleLauncherFlowChat(ctx, launcherChatCtx(), "conv-bridge-315", "打开小星星", "default"); !handled {
			t.Fatalf("launcher turn not handled")
		}
		path := filepath.Join(t.TempDir(), "乐局工程.vit")
		if err := os.WriteFile(path, []byte("edu-bridge-project"), 0o644); err != nil {
			t.Fatalf("write fake project: %v", err)
		}
		server.mu.Lock()
		server.activeWorkspacePath = path
		server.mu.Unlock() // UUID 已由 newEduJamTestServer 绑定（假内核 get_project_state 同源）
		return path
	}

	t.Run("活跃局-315恢复播报承接", func(t *testing.T) {
		server := newBridgeEduServer(t)
		path := launcherOriginThenBind(t, server)
		eduJamWriteIntroBinding(t, path, legameBindingSchemaVersion, "first-run-twinkle", "", "")
		// 播种活跃局（owner 会话）——恢复播报分支的在场条件。
		if resp, handled := server.eduJamEnter(ctx, "conv-owner", "chat", ""); !handled || resp.StopReason != "edu_jam_entered" {
			t.Fatalf("seed active jam failed: handled=%v stop=%s", handled, resp.StopReason)
		}
		// 315 既有机制在新链路（launcher 起源会话）下照常触发——零重写衔接证明。
		resp, served := server.eduJamAutoIntroCheckpoint(ctx, "conv-bridge-315", "chat", "继续")
		if !served || resp.StopReason != "edu_jam_recovery_broadcast" {
			t.Fatalf("315 recovery broadcast must own the bridge: served=%v stop=%s reply=%s", served, resp.StopReason, resp.Reply)
		}
		if !strings.Contains(resp.Reply, "接着上局") {
			t.Fatalf("recovery reply drift: %q", resp.Reply)
		}
		// 本卡检查点对 binding 工程 fail-closed 让位（同一会话同一状态）。
		if _, bridged := server.launcherBridgeBroadcastCheckpoint(ctx, "conv-bridge-315", "chat", "继续"); bridged {
			t.Fatal("binding project must defer to 315 (零干预)")
		}
	})
	t.Run("新binding-315自动开场承接", func(t *testing.T) {
		server := newBridgeEduServer(t)
		path := launcherOriginThenBind(t, server)
		eduJamWriteIntroBinding(t, path, legameBindingSchemaVersion, "first-run-twinkle", "", "")
		resp, served := server.eduJamAutoIntroCheckpoint(ctx, "conv-bridge-315", "chat", "在吗")
		if !served || resp.StopReason != "edu_jam_entered" {
			t.Fatalf("315 auto intro must own the fresh-binding entry: served=%v stop=%s", served, resp.StopReason)
		}
		if binding, ok := loadLegameBinding(path); !ok || binding.OpenedAt == nil || *binding.OpenedConversationID != "conv-bridge-315" {
			t.Fatalf("opened_at write-back missing: binding=%+v ok=%v", binding, ok)
		}
		if _, bridged := server.launcherBridgeBroadcastCheckpoint(ctx, "conv-bridge-315", "chat", "在吗"); bridged {
			t.Fatal("binding project must defer to 315 (零干预)")
		}
	})
}

// launcher 终答→directive 的自然语确认句与回执同发（337 既有文案复用回归）：
// directive 非空的回执必带一句确认（「好的，…《X》。」），同响应不重复。
func TestLauncherDirectiveConfirmationSentence(t *testing.T) {
	resetLauncherBridgeStateForTest()
	t.Cleanup(resetLauncherBridgeStateForTest)
	ctx := context.Background()
	cases := []struct {
		name    string
		message string
		recents []string
	}{
		{name: "打开-唯一命中", message: "打开小星星", recents: launcherTestRecents},
		{name: "裸继续-唯一最近", message: "继续上次那个工程", recents: []string{"D:/a/唯一工程.vit"}},
		{name: "新建-带名", message: "新建一个叫测试小样的工程", recents: launcherTestRecents},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newLauncherTestServer(&launcherFakeKernel{recents: tc.recents}, nil)
			resp, handled := s.handleLauncherFlowChat(ctx, launcherChatCtx(), "conv-confirm", tc.message, "default")
			if !handled || resp.LauncherDirective == nil {
				t.Fatalf("want directive turn, handled=%v directive=%v", handled, resp.LauncherDirective)
			}
			name := resp.LauncherDirective.ProjectName
			if resp.LauncherDirective.Action == launcherActionOpen {
				name = launcherProjectDisplayName(resp.LauncherDirective.FilePath)
			}
			if !strings.Contains(resp.Reply, "好的") || !strings.Contains(resp.Reply, "《"+name+"》") {
				t.Fatalf("confirmation sentence missing 好的/《%s》: %q", name, resp.Reply)
			}
			if got := strings.Count(resp.Reply, "《"+name+"》"); got != 1 {
				t.Fatalf("confirmation sentence duplicated %d times: %q", got, resp.Reply)
			}
			if !strings.Contains(resp.LauncherDirective.Reason, name) {
				t.Fatalf("directive reason %q missing %q", resp.LauncherDirective.Reason, name)
			}
			if resp.ContextFlow != launcherFlowLauncher {
				t.Fatalf("directive receipt context_flow=%q want %q", resp.ContextFlow, launcherFlowLauncher)
			}
		})
	}
}
