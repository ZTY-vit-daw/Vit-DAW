package chat

import (
	"context"
	"errors"
	"regexp"
	"testing"
)

// SHELL-6（池序 367）零栈单测：工程面跨工程意图路由——门（user_entered_project
// 会话事实）+ 新建/打开分支 + 裸引用不拦截 + 红线（NeedsConfirmation 恒 false、
// 不触 authority）+ 22:49 缺陷形态回归（进入过工程后启动台回访会话同路由）。

func newProjectFlowTestServer(entered bool) *Server {
	s := newLauncherTestServer(&launcherFakeKernel{recents: launcherTestRecents}, nil)
	if entered {
		s.markUserEnteredProject()
	}
	return s
}

func TestProjectFlowGateOnEnteredProjectFact(t *testing.T) {
	// 未进入工程：工程面分支不适用（launcher 分支的主场，零行为变化）。
	s := newProjectFlowTestServer(false)
	if _, handled := s.handleProjectFlowChat(context.Background(), "c", "新建工程", "default"); handled {
		t.Fatalf("未进入工程时工程面分支不得拦截（launcher 分支主场）")
	}
	// 进入工程后：同一会话事实开门（22:49 实况：launcher 会话被副门关闭后落入
	// LLM 纯文字「要确认」——现在必须确定性出 directive）。
	s2 := newProjectFlowTestServer(true)
	resp, handled := s2.handleProjectFlowChat(context.Background(), "webui_launcher_x", "新建工程", "default")
	if !handled {
		t.Fatalf("进入工程后「新建工程」必须确定性路由（22:49 缺陷回归）")
	}
	if resp.LauncherDirective == nil || resp.LauncherDirective.Action != launcherActionNew {
		t.Fatalf("新建意图必须带 new_project directive，got %+v", resp.LauncherDirective)
	}
}

func TestProjectFlowNewIntentBareAndNamed(t *testing.T) {
	s := newProjectFlowTestServer(true)
	// 无名：缺省草稿名（SHELL-5 同款 MMDD_HHmm）。
	resp, handled := s.handleProjectFlowChat(context.Background(), "c", "新建工程", "default")
	if !handled {
		t.Fatalf("无名新建必须路由")
	}
	if resp.LauncherDirective == nil || resp.LauncherDirective.Action != launcherActionNew {
		t.Fatalf("directive 缺失/类型错误: %+v", resp.LauncherDirective)
	}
	if ok, err := regexp.MatchString(`^新工程_\d{4}_\d{4}$`, resp.LauncherDirective.ProjectName); err != nil || !ok {
		t.Fatalf("无名新建缺省名不符 MMDD_HHmm: %q", resp.LauncherDirective.ProjectName)
	}
	if !resp.LauncherDirective.RequiresGovernance {
		t.Fatalf("跨工程 directive requires_governance 恒 true")
	}
	// 带名。
	resp2, _ := s.handleProjectFlowChat(context.Background(), "c", "新建一个叫下一段的工程", "default")
	if resp2.LauncherDirective == nil || resp2.LauncherDirective.ProjectName != "下一段" {
		t.Fatalf("带名新建 directive 名不符: %+v", resp2.LauncherDirective)
	}
}

func TestProjectFlowOpenIntent(t *testing.T) {
	s := newProjectFlowTestServer(true)
	// 带引用唯一命中 → open directive（切换语态）。
	resp, handled := s.handleProjectFlowChat(context.Background(), "c", "打开小星星", "default")
	if !handled || resp.LauncherDirective == nil {
		t.Fatalf("带引用打开必须出 directive: handled=%v resp=%+v", handled, resp)
	}
	if resp.LauncherDirective.Action != launcherActionOpen || resp.LauncherDirective.FilePath != "D:/VIT/projects/小星星乐局.vit" {
		t.Fatalf("open directive 目标不符: %+v", resp.LauncherDirective)
	}
	// 多命中 → 结果卡（点卡切换），不出指令。
	resp2, _ := s.handleProjectFlowChat(context.Background(), "c", "打开混音", "default")
	if resp2.LauncherDirective != nil || len(resp2.ProjectResultCards) != 2 {
		t.Fatalf("多命中应出 2 张结果卡不出指令: cards=%d directive=%v", len(resp2.ProjectResultCards), resp2.LauncherDirective)
	}
	// 零命中硬动词 → 最近清单兜底卡。
	resp3, _ := s.handleProjectFlowChat(context.Background(), "c", "打开不存在", "default")
	if resp3.LauncherDirective != nil || len(resp3.ProjectResultCards) == 0 {
		t.Fatalf("零命中应出最近清单卡: cards=%d", len(resp3.ProjectResultCards))
	}
	// 零命中软动词 → 回落 LLM（同 launcher 口径）。
	if _, handled4 := s.handleProjectFlowChat(context.Background(), "c", "继续《不存在的话题》", "default"); handled4 {
		t.Fatalf("软动词带引用零命中应回落 LLM")
	}
}

func TestProjectFlowBareOpenReferenceNotIntercepted(t *testing.T) {
	s := newProjectFlowTestServer(true)
	// 工程面裸「继续/打开上次」是对话延续歧义句，不得拦截（区别于启动台面）。
	for _, msg := range []string{"继续", "接着", "打开上次的工程"} {
		if _, handled := s.handleProjectFlowChat(context.Background(), "c", msg, "default"); handled {
			t.Fatalf("裸引用打开意图不得拦截（工程内歧义句）: %q", msg)
		}
	}
	// 列表/闲聊/疑问口吻同样不拦截（工程面只认跨工程意图）。
	for _, msg := range []string{"我有哪些工程", "随便聊聊", "怎么打开工程文件"} {
		if _, handled := s.handleProjectFlowChat(context.Background(), "c", msg, "default"); handled {
			t.Fatalf("非跨工程意图不得拦截: %q", msg)
		}
	}
}

func TestProjectFlowResponseRedLines(t *testing.T) {
	s := newProjectFlowTestServer(true)
	resp, handled := s.handleProjectFlowChat(context.Background(), "c", "新建工程", "default")
	if !handled {
		t.Fatalf("必须路由")
	}
	// 红线 1：不产生 chat 确认面（治理归属宿主页内三键卡，337 语义）。
	if resp.NeedsConfirmation {
		t.Fatalf("工程面意图轮不得带 NeedsConfirmation（确认面归属宿主治理卡）")
	}
	// 红线 2：回执流标记=project（工程面对话语境）。
	if resp.ContextFlow != launcherFlowProject {
		t.Fatalf("context_flow 应为 project: %q", resp.ContextFlow)
	}
	// 红线 3：不触 authority（authority mode 保持缺省）。
	if mode := s.authorityModeSnapshot(); mode != authorityModeManual {
		t.Fatalf("工程面分支不得写 authority（got %q）", mode)
	}
}

func TestProjectFlowKernelUnavailableHonest(t *testing.T) {
	s := newLauncherTestServer(&launcherFakeKernel{err: errors.New("kernel down")}, nil)
	s.markUserEnteredProject()
	resp, handled := s.handleProjectFlowChat(context.Background(), "c", "打开小星星", "default")
	if !handled {
		t.Fatalf("内核不可达也应诚实回执（不落 LLM 猜测）")
	}
	if resp.LauncherDirective != nil {
		t.Fatalf("内核不可达不得出 directive")
	}
	if resp.Reply != launcherKernelUnavailableReply {
		t.Fatalf("内核不可达回执文案不符: %q", resp.Reply)
	}
}
