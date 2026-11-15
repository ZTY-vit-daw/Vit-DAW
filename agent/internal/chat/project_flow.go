package chat

import (
	"context"
	"time"
)

// ── SHELL-6（池序 367，2026-11-15）：工程面对话意图执行 ───────────────────────
// launcher 确定性意图路由的工程面平移（SHELL-5 语义：对话流新建/打开意图=自动
// 执行，用户裁定 2026-11-15 第 3 条）。
//
// 缺陷背景（22:49 取证在卷）：工程面说「新建工程」→ launcher 分支不适用
// （launcherFlowApplicable 副门 user_entered_project 已置）→ 裸落 LLM 4 次调用
// → 纯文字回复「要确认」→ 无任何可交互确认面。本分支把同一套确定性意图分类
// （launcherClassifyMessage，零 LLM）适用到「宿主已播报工程进入」的会话事实上：
// 新建/打开工程意图 → directive 回执，UI 自动经桥执行；跨工程治理确认（337
// 保守语义）由宿主按需出页内三键卡（有在开工程即出），agent 出指令不带执行权、
// 不产生 chat 确认面（NeedsConfirmation 恒 false）——与 launcher 分支同一边界。
//
// 适用面（与 launcher 分支互补，不重叠）：
//   - 副门 user_entered_project=true（进程内会话事实，工程面与「回启动台再进
//     工程」两种位置都覆盖——22:49 实况即后者：launcher 会话在进入过工程后被
//     副门关闭，落入 LLM 纯文字）。
//   - 意图=新建（带名/无名）或打开（须带显式引用——工程面里裸「继续/接着」是
//     对话延续不是切工程，不拦截；软动词引用零命中回落 LLM，同 launcher 口径）。
//   - 列表/闲聊/疑问口吻不拦截，照旧回落既有轮（launcherClassifyMessage 自带）。

// projectFlowApplicable 报告工程面跨工程意图分支是否适用：宿主已播报工程进入
// （user_entered_project 会话事实）。launcher 分支先查（其副门正好相反），两个
// 分支在「未进工程=launcher 面 / 已进工程=工程面」上互斥衔接。
func (s *Server) projectFlowApplicable() bool {
	if s == nil || s.harness == nil {
		return false
	}
	return s.userEnteredProjectSnapshot()
}

// handleProjectFlowChat 是 chat 管线里的工程面跨工程意图分支（handleChat 在
// launcher 分支之后调用）：意图命中 → directive/结果卡/说明；意图不明或裸引用
// 打开 → handled=false 原样回落。不触 authority/pendingmanager；directive 只是
// 回执字段，切工程动作由 UI 经桥执行（写者不变式与 launcher 分支一致）。
func (s *Server) handleProjectFlowChat(ctx context.Context, conversationID, message, agentMode string) (ChatResponse, bool) {
	if s == nil || !s.projectFlowApplicable() {
		return ChatResponse{}, false
	}
	intent := launcherClassifyMessage(message)
	if intent.Kind != launcherIntentNew && intent.Kind != launcherIntentOpen {
		return ChatResponse{}, false
	}
	// 工程面打开意图须带显式引用：裸「继续/打开上次」在工程内是歧义句（延续
	// 当前工作 vs 切到上个工程），不拦截，交 LLM/既有轮；带引用才构成跨工程切换。
	if intent.Kind == launcherIntentOpen && intent.Ref == "" {
		return ChatResponse{}, false
	}

	resp := ChatResponse{ConversationID: conversationID, AgentMode: agentMode}
	resp.ContextFlow = launcherFlowProject

	if intent.Kind == launcherIntentNew {
		// 与 launcher 分支同款：无名新建=缺省草稿名直接出 directive（SHELL-5
		// 自动执行语义；名实相符的内核侧承接是 363 域）。
		name := intent.Name
		if name == "" {
			name = launcherDefaultNewProjectName(time.Now())
			resp.Reply = "好——这就新建一个工程（暂名《" + name + "》，进去后可以改名）并切换过去——如当前工程需要处理，切换确认卡会出现。"
		} else {
			resp.Reply = "好——这就新建工程《" + name + "》并切换过去——如当前工程需要处理，切换确认卡会出现。"
		}
		resp.LauncherDirective = &LauncherDirective{
			Action:             launcherActionNew,
			ProjectName:        name,
			Reason:             "新建工程《" + name + "》（工程面对话）",
			RequiresGovernance: true,
		}
		return resp, true
	}

	// 打开（带引用）：与 launcher 分支同一 recents 分层匹配面。
	paths, recentsErr := s.launcherRecentProjectPaths(ctx)
	entries := launcherRecentEntries(paths)
	if recentsErr != nil {
		resp.Reply = launcherKernelUnavailableReply
		return resp, true
	}
	hits := launcherMatchReference(intent.Ref, entries)
	if intent.Soft && len(hits) == 0 {
		// 软动词带引用零命中：更像普通对话，回落 LLM（同 launcher 口径）。
		return ChatResponse{}, false
	}
	switch {
	case len(hits) == 1:
		resp.Reply = "好——这就切换到《" + hits[0].Name + "》——如当前工程需要处理，切换确认卡会出现。"
		resp.LauncherDirective = launcherOpenDirective(hits[0])
	case len(hits) > 1:
		resp.Reply = "有好几个都像「" + intent.Ref + "」——点一张卡片直接切换，或说得更具体一点："
		resp.ProjectResultCards = launcherProjectCards(hits, launcherMaxCards)
	case len(entries) == 0:
		resp.Reply = "还没找到叫「" + intent.Ref + "」的工程，最近也没有工程记录——可以说「新建一个工程」重新开始。"
	default:
		resp.Reply = "没找到叫「" + intent.Ref + "」的工程。这些是最近的工程——点卡片直接切换，或换个说法："
		resp.ProjectResultCards = launcherProjectCards(entries, launcherMaxCards)
	}
	return resp, true
}
