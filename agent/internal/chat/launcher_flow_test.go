package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
