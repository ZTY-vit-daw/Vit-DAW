package chat

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"
)

// ── LAUNCHER-CHAT-BE-1（332 拆卡 C4，2026-11-09）─────────────────────────────
// launcher 流（启动卷）确定性意图路由 + 门厅简版开场白。
//
// 边界（设计稿 ai-first-launcher-design-v1 §2.2/§3.1/§4.3）：
//   - 触发条件 = chat 请求 context.launcher=true（主门=UI 声明，§3.1 启动台态
//     自报，非权限声明）且本 agent 进程内尚未收到 version.project_opened 工程
//     进入播报（副门=会话事实 user_entered_project）。LAUNCHER-GATE-FIX-1
//     （340）：旧「无活跃工程」uuid=="" 判据废弃——真栈结构性不可达（339 双层
//     证据：boot 恒载 default XML 自带 uuid；VSP 紧凑快照无条件占位
//     project_current 使 CurrentProjectIdentity 恒非空）；uuid/path 均不再参与
//     门判定。有活跃工程时的跨工程意图归 LAUNCHER-BRIDGE-BE-1（C5）治理链。
//   - launcher 档是结构性准入（无工程则工程内工具无对象），不新增 authority
//     枚举：本文件零 authority/pendingmanager 读写；directive 只是回执字段，
//     打开/新建动作由 UI 执行——kernel open_project 与 change_scene 的写者保持
//     Godot（写者不变式 §4.3，agent 不调 project.open/project.new 工具）。
//   - 全确定性零 LLM：意图识别/匹配/文案均为模板；意图不明不拦截，照旧回落
//     既有 LLM 轮（结果卡点选路径不依赖 LLM，§8 确定性退路）。

const (
	// launcherContextKey 是 chat 请求 context 中标记 launcher 流的键（§3.1：
	// 提示词分支提示，不是权限声明）。
	launcherContextKey = "launcher"
	// launcherMaxCards 限制单回执结果卡数量（多命中/最近清单共用）。
	launcherMaxCards = 6
	// launcherMaxRecents 限制从 kernel get_recent_projects 消费的条数。
	launcherMaxRecents = 16
	// launcherMaxIntentRunes 限制确定性路由拦截的消息长度；更长的长句交给
	// LLM（降低复合句误拦截）。
	launcherMaxIntentRunes = 48
)

const (
	launcherActionOpen = "open_project"
	launcherActionNew  = "new_project"
)

// LauncherDirective 是回执驱动的启动指令（设计稿 §4.1 冻结 schema）。agent
// 只决定与呈报，不执行；requires_governance 恒 true——UI 侧仍自查（双保险）。
type LauncherDirective struct {
	Action             string `json:"action"`                 // open_project | new_project
	FilePath           string `json:"file_path,omitempty"`    // open 时必填
	ProjectName        string `json:"project_name,omitempty"` // new 时必填
	Reason             string `json:"reason,omitempty"`       // 播报用一句话（可选）
	RequiresGovernance bool   `json:"requires_governance"`    // 恒 true，UI 自查
}

func launcherContextActive(chatCtx map[string]any) bool {
	return chatCtx != nil && boolValue(chatCtx[launcherContextKey])
}

// launcherFlowApplicable 报告本轮 chat 是否走 launcher 分支：UI 声明
// context.launcher=true（主门——启动台态自报，§3.1 非权限声明）且本 agent
// 进程内尚未收到 version.project_opened 工程进入播报（副门=会话事实
// user_entered_project，server.go handleInvoke 接收点置位，进程内不重置）。
// 340 修复：uuid=="" 判据废弃（339 真栈双层证据=boot default uuid/VSP 占位
// project_current 恒非空）；uuid/path 不参与门判定。
func (s *Server) launcherFlowApplicable(_ context.Context, chatCtx map[string]any) bool {
	if !launcherContextActive(chatCtx) {
		return false
	}
	if s == nil || s.harness == nil {
		return false
	}
	return !s.userEnteredProjectSnapshot()
}

// markUserEnteredProject 记录宿主已播报工程进入（version.project_opened
// lifecycle 通知到达即置）。进程内一次置位不重置：agent 重启回 false——「重进
// 工程再播报一次桥接」正是设计形态（同 launcherBridgeServed 生命周期口径）。
func (s *Server) markUserEnteredProject() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.userEnteredProject = true
	s.mu.Unlock()
}

// userEnteredProjectSnapshot 只读快照（s.mu 保护；nil server 视为未进入）。
func (s *Server) userEnteredProjectSnapshot() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.userEnteredProject
}

// ── 意图识别（纯函数，零 LLM）───────────────────────────────────────────────

const (
	launcherIntentNone = ""
	launcherIntentOpen = "open"
	launcherIntentNew  = "new"
	launcherIntentList = "list"
)

type launcherIntent struct {
	Kind string
	Ref  string // open：工程引用；空=裸继续（目标=最近一条）
	Name string // new：新工程名；空=追问名字
	Soft bool   // open：软动词（继续/接着/恢复/进入…）——带引用零命中时回落 LLM，
	//不出「没找到」卡面（「继续聊聊刚才的话题」是聊天不是开工程）。
}

var (
	// 引号名提取（打开《X》/继续「X」/新建叫"X"的工程共用）。
	launcherQuotedNameRe = regexp.MustCompile("[《「『“\"'【]([^》」』”\"'】]{1,64})[》」』”\"'】]")
	// 新建叫X的工程 / 新建X工程 / 创建XX项目（惰性名+尾锚工程/项目）。
	launcherNewNamedRe = regexp.MustCompile(`(?:新建|创建|新开|开个新|开一个新的|来个新|来一个新|建一个|建个)(?:一个|个)?(?:名字叫|名为|叫)?([^，。,．.!！?？;；]{1,64}?)(?:的)?(?:工程|项目|project)`)
	// 新建工程 X / 新建个项目：X（尾缀名）。
	launcherNewTailNameRe = regexp.MustCompile(`(?:新建|创建|新开)(?:一个|个)?(?:工程|项目)(?:文件)?(?:，|,|:|：)?(?:名字叫|名为|叫)?\s*([^，。,．.!！?？;；]{1,64})$`)
	// 把X打开（动词后置句式）。
	launcherBaOpenRe = regexp.MustCompile(`^把(.{1,64}?)(?:给)?(?:打开|开启|开一下)$`)
)

// 硬动词=工程对象语义自足（打开/载入…带引用零命中仍出确定性「没找到」面）；
// 软动词=可指工程也可指对话（继续/接着/恢复/进入…零命中回落 LLM）。
var launcherOpenHardVerbs = []string{"打开", "开启", "开一下", "载入", "加载", "换到", "切到", "回到"}
var launcherOpenSoftVerbs = []string{"继续", "接着", "恢复", "进入", "上次", "上局"}

var launcherAllOpenVerbs = append(append([]string(nil), launcherOpenHardVerbs...), launcherOpenSoftVerbs...)

// launcherInterrogativeProbes 命中即不拦截：疑问/讲解/商量口吻交给 LLM，
// 确定性路由只认祈使句（防「怎么打开工程文件」被误当打开指令）。
var launcherInterrogativeProbes = []string{
	"怎么", "如何", "为什么", "什么意思", "啥意思", "是指", "介绍一下", "介绍下", "解释", "讲解", "吗", "能不能", "可不可以", "能不能直接",
}

func launcherClassifyMessage(message string) launcherIntent {
	msg := strings.TrimSpace(message)
	if msg == "" || strings.HasPrefix(msg, "/") {
		return launcherIntent{Kind: launcherIntentNone}
	}
	if utf8.RuneCountInString(msg) > launcherMaxIntentRunes {
		return launcherIntent{Kind: launcherIntentNone}
	}
	for _, probe := range launcherInterrogativeProbes {
		if strings.Contains(msg, probe) {
			return launcherIntent{Kind: launcherIntentNone}
		}
	}
	switch {
	case launcherMessageWantsNewProject(msg):
		return launcherIntent{Kind: launcherIntentNew, Name: launcherExtractNewProjectName(msg)}
	case launcherMessageWantsOpenProject(msg):
		ref, soft := launcherExtractOpenReference(msg)
		return launcherIntent{Kind: launcherIntentOpen, Ref: ref, Soft: soft}
	case launcherMessageWantsProjectList(msg):
		return launcherIntent{Kind: launcherIntentList}
	default:
		return launcherIntent{Kind: launcherIntentNone}
	}
}

func launcherMessageWantsNewProject(msg string) bool {
	hasNoun := strings.Contains(msg, "工程") || strings.Contains(msg, "项目") || strings.Contains(strings.ToLower(msg), "project")
	if !hasNoun {
		return false
	}
	for _, verb := range []string{"新建", "创建", "新开", "开个新", "开一个新的", "来个新", "来一个新", "建一个", "建个"} {
		if strings.Contains(msg, verb) {
			return true
		}
	}
	return false
}

func launcherMessageWantsOpenProject(msg string) bool {
	_, verb := launcherEarliestOpenVerb(msg)
	return verb != ""
}

func launcherVerbIsSoft(verb string) bool {
	for _, soft := range launcherOpenSoftVerbs {
		if verb == soft {
			return true
		}
	}
	return false
}

func launcherMessageWantsProjectList(msg string) bool {
	// 列表问句不得与打开/新建动词同现（「打开最近的工程」是打开意图）。
	if launcherMessageWantsOpenProject(msg) || launcherMessageWantsNewProject(msg) {
		return false
	}
	probes := []string{
		"有哪些工程", "有哪些项目", "有几个工程", "有几个项目", "多少个工程", "多少个项目",
		"什么工程", "工程列表", "项目列表", "列出工程", "列出项目", "列一下工程", "列一下项目",
		"最近工程", "最近的工程", "最近的项目", "看看工程", "看下工程", "所有工程", "全部工程",
	}
	for _, probe := range probes {
		if strings.Contains(msg, probe) {
			return true
		}
	}
	return false
}

func launcherEarliestOpenVerb(msg string) (int, string) {
	best, bestVerb := -1, ""
	for _, verb := range launcherAllOpenVerbs {
		if idx := strings.Index(msg, verb); idx >= 0 && (best < 0 || idx < best) {
			best, bestVerb = idx, verb
		}
	}
	if best < 0 {
		return 0, ""
	}
	return best, bestVerb
}

func launcherExtractNewProjectName(msg string) string {
	for _, re := range []*regexp.Regexp{launcherQuotedNameRe, launcherNewNamedRe, launcherNewTailNameRe} {
		if m := re.FindStringSubmatch(msg); len(m) > 1 {
			if name := launcherCleanName(m[1]); name != "" {
				return name
			}
		}
	}
	return ""
}

// launcherExtractOpenReference 返回（引用，软动词标记）。引用空=裸继续。
func launcherExtractOpenReference(msg string) (string, bool) {
	idx, verb := launcherEarliestOpenVerb(msg)
	if verb == "" {
		return "", false
	}
	soft := launcherVerbIsSoft(verb)
	for _, re := range []*regexp.Regexp{launcherQuotedNameRe, launcherBaOpenRe} {
		if m := re.FindStringSubmatch(msg); len(m) > 1 {
			if ref := launcherCleanReference(m[1]); ref != "" {
				return ref, soft
			}
		}
	}
	return launcherCleanReference(msg[idx+len(verb):]), soft
}

// launcherCleanName 清洗新工程名：去标点空白，剥离开头量词，超长视为提取失败。
func launcherCleanName(raw string) string {
	s := strings.TrimSpace(strings.Trim(strings.TrimSpace(raw), "，。,.!！?？;；:：、 \t"))
	for _, lead := range []string{"一个", "那个", "这个"} {
		s = strings.TrimPrefix(s, lead)
	}
	s = strings.TrimSpace(s)
	if s == "" || utf8.RuneCountInString(s) > 64 {
		return ""
	}
	return s
}

// launcherCleanReference 清洗工程引用：剥离开头指代词（一个/那个/上次…）与
// 结尾语气词/工程名词，剩余为引用；空=裸继续。超长视为无效（回落裸继续）。
func launcherCleanReference(raw string) string {
	s := strings.TrimSpace(raw)
	leadFillers := []string{"一个", "那个", "这个", "刚才", "最近", "上次", "上局", "我", "的"}
	for changed := true; changed; {
		changed = false
		for _, filler := range leadFillers {
			if strings.HasPrefix(s, filler) {
				s = strings.TrimSpace(s[len(filler):])
				changed = true
			}
		}
	}
	trailing := []string{"工程文件", "工程", "项目", "的", "吧", "呢", "啊", "呀", "哈", "了", "。", "！", "!", "？", "?", "，", ",", "；", ";", "：", ":", "~"}
	for changed := true; changed; {
		changed = false
		for _, filler := range trailing {
			if strings.HasSuffix(s, filler) {
				s = strings.TrimSpace(s[:len(s)-len(filler)])
				changed = true
			}
		}
	}
	if utf8.RuneCountInString(s) > 64 {
		return ""
	}
	return s
}

// ── project.recent 读取与匹配（路径/名称/模糊，分层取先）────────────────────

type launcherProjectEntry struct {
	Path string
	Name string // 去扩展名的工程显示名
}

func launcherProjectDisplayName(path string) string {
	base := filepath.Base(strings.TrimSpace(path))
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func launcherRecentEntries(paths []string) []launcherProjectEntry {
	out := make([]launcherProjectEntry, 0, len(paths))
	seen := map[string]bool{}
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		key := strings.ToLower(strings.ReplaceAll(path, "\\", "/"))
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, launcherProjectEntry{Path: path, Name: launcherProjectDisplayName(path)})
	}
	return out
}

func launcherNormalizeRef(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, "\\", "/")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.TrimSuffix(s, ".vit")
	return s
}

// launcherMatchReference 分层匹配：①路径全等 ②路径后缀 ③名称全等 ④互相包含
// （模糊，引用与名称均 ≥2 字才启用）。取最先有命中的层，同层多命中=歧义。
func launcherMatchReference(ref string, entries []launcherProjectEntry) []launcherProjectEntry {
	normRef := launcherNormalizeRef(ref)
	if normRef == "" {
		return nil
	}
	var tierPath, tierSuffix, tierName, tierFuzzy []launcherProjectEntry
	for _, entry := range entries {
		// normPath 与 normRef 同一归一（含 .vit 剥离/斜杠归一），保证①②层
		// 「带扩展名引用」与「裸名引用」都能命中。
		normPath := launcherNormalizeRef(entry.Path)
		normName := launcherNormalizeRef(entry.Name)
		switch {
		case normRef == normPath:
			tierPath = append(tierPath, entry)
		case strings.HasSuffix(normPath, "/"+normRef):
			tierSuffix = append(tierSuffix, entry)
		case normRef == normName:
			tierName = append(tierName, entry)
		case utf8.RuneCountInString(normRef) >= 2 && utf8.RuneCountInString(normName) >= 2 &&
			(strings.Contains(normName, normRef) || strings.Contains(normRef, normName)):
			tierFuzzy = append(tierFuzzy, entry)
		}
	}
	for _, tier := range [][]launcherProjectEntry{tierPath, tierSuffix, tierName, tierFuzzy} {
		if len(tier) > 0 {
			return tier
		}
	}
	return nil
}

// launcherRecentProjectPaths 读 kernel get_recent_projects（project.recent 工具
// 同一 RiskDirect 只读面；chat 内直接只读调用=runtimeKernelStatus ping 先例）。
func (s *Server) launcherRecentProjectPaths(ctx context.Context) ([]string, error) {
	if s == nil || s.harness == nil {
		return nil, fmt.Errorf("harness is nil")
	}
	reply, _, err := s.harness.KernelSendCommand(ctx, map[string]any{"cmd": "get_recent_projects"})
	if err != nil {
		return nil, err
	}
	if status := strings.TrimSpace(fmt.Sprint(reply["status"])); strings.EqualFold(status, "error") {
		return nil, fmt.Errorf("kernel get_recent_projects: %s", firstNonEmpty(strings.TrimSpace(fmt.Sprint(reply["error"])), status))
	}
	raw, _ := reply["recent_projects"].([]any)
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
			out = append(out, strings.TrimSpace(text))
		}
	}
	if len(out) > launcherMaxRecents {
		out = out[:launcherMaxRecents]
	}
	return out, nil
}

// ── 回执构造（三分支）──────────────────────────────────────────────────────

func launcherOpenDirective(entry launcherProjectEntry) *LauncherDirective {
	return &LauncherDirective{
		Action:             launcherActionOpen,
		FilePath:           entry.Path,
		Reason:             "打开工程《" + entry.Name + "》",
		RequiresGovernance: true,
	}
}

// launcherProjectCards 构造可点选的启动台结果卡（ProjectResultCards 字段族
// 先例 F15；kind=launcher_project，卡内嵌 directive——点卡=执行同一指令）。
func launcherProjectCards(entries []launcherProjectEntry, limit int) []map[string]any {
	if len(entries) == 0 {
		return nil
	}
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	cards := make([]map[string]any, 0, len(entries))
	for index, entry := range entries {
		cards = append(cards, map[string]any{
			"kind":               "launcher_project",
			"type":               "project_open_candidate",
			"id":                 fmt.Sprintf("launcher_project_%d_%s", index+1, launcherCardIDSuffix(entry.Name)),
			"name":               entry.Name,
			"file_path":          entry.Path,
			"launcher_directive": launcherOpenDirective(entry),
		})
	}
	return cards
}

func launcherCardIDSuffix(name string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r >= 0x4e00 && r <= 0x9fff:
			builder.WriteRune(r)
		default:
			builder.WriteRune('_')
		}
	}
	out := strings.Trim(builder.String(), "_")
	if out == "" {
		out = "proj"
	}
	if runes := []rune(out); len(runes) > 32 {
		out = string(runes[:32])
	}
	return out
}

// handleLauncherFlowChat 是 chat 管线里的 launcher 确定性分支（handleChat 在
// pending 确认/开发命令之后调用）：意图命中→directive/结果卡/说明；意图不明
// 或不适用→handled=false 原样回落。不产生任何确认面（NeedsConfirmation 恒
// false）——治理确认在 UI 层（§3.3），agent 出指令不带执行权。
func (s *Server) handleLauncherFlowChat(ctx context.Context, chatCtx map[string]any, conversationID, message, agentMode string) (ChatResponse, bool) {
	if s == nil || !s.launcherFlowApplicable(ctx, chatCtx) {
		return ChatResponse{}, false
	}
	// LAUNCHER-BRIDGE-BE-1（338）：launcher 卷起源标记（幂等）——该会话此后经
	// directive 进工程时，桥接播报检查点据此识别「跨卷进入」（普通工程进入零差异）。
	rememberLauncherFlowConversation(conversationID)
	intent := launcherClassifyMessage(message)
	if intent.Kind == launcherIntentNone {
		return ChatResponse{}, false
	}
	resp := ChatResponse{ConversationID: conversationID, AgentMode: agentMode}
	resp.ContextFlow = launcherFlowLauncher
	paths, recentsErr := s.launcherRecentProjectPaths(ctx)
	entries := launcherRecentEntries(paths)
	switch intent.Kind {
	case launcherIntentNew:
		if intent.Name == "" {
			resp.Reply = "好——新工程想叫什么名字？直接说「新建 XX 的工程」就行。"
			return resp, true
		}
		resp.Reply = "好的，新建工程《" + intent.Name + "》。"
		resp.LauncherDirective = &LauncherDirective{
			Action:             launcherActionNew,
			ProjectName:        intent.Name,
			Reason:             "新建工程《" + intent.Name + "》",
			RequiresGovernance: true,
		}
		return resp, true
	case launcherIntentList:
		if recentsErr != nil {
			resp.Reply = launcherKernelUnavailableReply
			return resp, true
		}
		if len(entries) == 0 {
			resp.Reply = launcherNoRecentsReply
			return resp, true
		}
		resp.Reply = fmt.Sprintf("最近的工程有 %d 个——点卡片打开，或直接跟我说名字：", len(entries))
		resp.ProjectResultCards = launcherProjectCards(entries, launcherMaxCards)
		return resp, true
	default: // launcherIntentOpen
		if recentsErr != nil {
			resp.Reply = launcherKernelUnavailableReply
			return resp, true
		}
		if intent.Ref == "" {
			// 裸继续（继续/打开上次…）：唯一最近=唯一命中；多个=结果卡点选。
			switch len(entries) {
			case 0:
				resp.Reply = launcherNoRecentsReply
			case 1:
				resp.Reply = "好的，继续《" + entries[0].Name + "》。"
				resp.LauncherDirective = launcherOpenDirective(entries[0])
			default:
				resp.Reply = "最近打开过几个工程——挑一个接着来（最近的在最前）："
				resp.ProjectResultCards = launcherProjectCards(entries, launcherMaxCards)
			}
			return resp, true
		}
		// 「打开磁带流/磁带」=看工程清单（启动页既有语义，出确定性列表卡）。
		if intent.Ref == "磁带流" || intent.Ref == "磁带" {
			if len(entries) == 0 {
				resp.Reply = launcherNoRecentsReply
				return resp, true
			}
			resp.Reply = fmt.Sprintf("最近的工程有 %d 个——点卡片打开，或直接跟我说名字：", len(entries))
			resp.ProjectResultCards = launcherProjectCards(entries, launcherMaxCards)
			return resp, true
		}
		hits := launcherMatchReference(intent.Ref, entries)
		if intent.Soft && len(hits) == 0 {
			// 软动词带引用零命中：更像普通对话（「继续聊聊刚才的话题」），
			// 回落 LLM，不出「没找到」卡面。
			return ChatResponse{}, false
		}
		switch {
		case len(hits) == 1:
			resp.Reply = "好的，打开《" + hits[0].Name + "》。"
			resp.LauncherDirective = launcherOpenDirective(hits[0])
		case len(hits) > 1:
			resp.Reply = "有好几个都像「" + intent.Ref + "」——点一张卡片打开，或说得更具体一点："
			resp.ProjectResultCards = launcherProjectCards(hits, launcherMaxCards)
		case len(entries) == 0:
			resp.Reply = "还没找到叫「" + intent.Ref + "」的工程，最近也没有工程记录——可以说「新建一个工程」开始。"
		default:
			resp.Reply = "没找到叫「" + intent.Ref + "」的工程。这些是最近的工程——点卡片打开，或换个说法："
			resp.ProjectResultCards = launcherProjectCards(entries, launcherMaxCards)
		}
		return resp, true
	}
}

const (
	launcherNoRecentsReply         = "还没有最近的工程记录。可以说「新建一个工程」，或从磁带流里选文件打开。"
	launcherKernelUnavailableReply = "我这儿暂时连不上内核，看不到工程列表——稍等一下再说一次，或先从磁带流里选工程打开。"
)

// handleLauncherGreeting 服务门厅简版开场白（确定性模板，零 LLM）：在场问候+
// 最近工程一条（project.recent 首项）+「想从哪开始？」。启动窗在 agent 就绪
// 时 GET 一次；无最近工程/内核暂不可达时给同样确定的降级模板。
func (s *Server) handleLauncherGreeting(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "GET required"})
		return
	}
	response := map[string]any{"status": "ok", "launcher": true}
	paths, err := s.launcherRecentProjectPaths(r.Context())
	if err == nil && len(paths) > 0 {
		entries := launcherRecentEntries(paths)
		response["reply"] = "我在。最近的工程是《" + entries[0].Name + "》——想从哪开始？"
		response["recent_project"] = map[string]any{"name": entries[0].Name, "file_path": entries[0].Path}
		response["recent_count"] = len(entries)
		writeJSON(w, http.StatusOK, response)
		return
	}
	if err != nil && s != nil && s.logger != nil {
		s.logger.Warn("[launcher.greeting] recent projects unavailable: %v", err)
	}
	response["reply"] = "我在。还没有最近的工程记录——想从哪开始？可以说「新建一个工程」，或从磁带流里选一个。"
	response["recent_count"] = 0
	writeJSON(w, http.StatusOK, response)
}

// ── LAUNCHER-BRIDGE-BE-1（332 拆卡 C5，2026-11-09）───────────────────────────
// 回执流标记（context_flow）+ 卷切换 typed_event + 桥接播报（案 B 承接核心）。
//
// 语义（设计稿 ai-first-launcher-design-v1 §2.1/§2.2/§4.2）：
//   - 流归属=当前工程绑定态（F10/F11），不是文本分类。回执 context_flow 三态：
//     "launcher"（launcher 确定性分支回执，自标）/"project"（工程绑定态 chat 回执，
//     writeChat 默认标）/空值（无工程无 launcher 标记的回执——LLM 回落轮、无工程
//     普通 chat、note 等路径保持空，omitempty 零回归）。
//   - 卷切换 typed_event 只落在桥接播报回执上：切换由 UI 执行 directive 成功后
//     发生（agent 不预写工程卷状态，设计稿 §2.2 切换时机），agent 在下一轮观察到
//     绑定态成立时补发切换标记，前端据此插「── 进入工程 · 《名称》 ──」分隔条。
//   - 桥接播报两分支：有乐局 binding 的工程由 315 自动开场/恢复播报既有机制承接
//    （eduJamAutoIntroCheckpoint 在 handleChat 中先行，本文件对 binding 工程
//     fail-closed 让位，零重写零干预）；无 binding 工程给一句确定性摘要（工程名+
//     最近修改时间，零 LLM）。
//
// 进程内态口径：launcher 卷本身即进程内会话缓冲（设计稿 §2.1 诚实边界），两组
// 标记随进程存活、重启即失——重播一次正是「重进工程」的设计形态（315
// eduJamIntroServed 同款生命周期，非持久化状态）。

const (
	// launcherFlowLauncher / launcherFlowProject 是回执 context_flow 字段值
	//（设计稿 §2.1 两流；空值=未标记，零回归）。
	launcherFlowLauncher = "launcher"
	launcherFlowProject  = "project"
	// launcherBridgeStopReason 桥接播报回执 stop_reason（edu_jam_recovery_broadcast
	// 同族命名先例）。
	launcherBridgeStopReason = "launcher_bridge_broadcast"
)

// launcherFlowConversations 记录 launcher 卷起源会话（key=conversationID）。
var launcherFlowConversations sync.Map

// launcherBridgeServed 记录已播报过桥接摘要的（conversationID, projectUUID,
// projectPath）——同一会话对同一工程的桥接播报会话级一次性。
var launcherBridgeServed sync.Map

// resetLauncherBridgeStateForTest 清空包级标记（包级态不随 Server 生命周期回收，
// 测试间互染防线；仅测试文件可调用）。
func resetLauncherBridgeStateForTest() {
	launcherFlowConversations.Range(func(key, _ any) bool {
		launcherFlowConversations.Delete(key)
		return true
	})
	launcherBridgeServed.Range(func(key, _ any) bool {
		launcherBridgeServed.Delete(key)
		return true
	})
}

func rememberLauncherFlowConversation(conversationID string) {
	if strings.TrimSpace(conversationID) == "" {
		return
	}
	launcherFlowConversations.Store(conversationID, true)
}

// applyChatContextFlowMark 回执流标记默认值（handleChat 的 writeChat 统一插点）：
// launcher 分支回执已自标 launcher（非空即保持）；其余回执在工程绑定态下默认标
// project；无绑定态保持空值（零回归——既有消费方看不到新字段）。
func (s *Server) applyChatContextFlowMark(resp *ChatResponse) {
	if s == nil || resp == nil || strings.TrimSpace(resp.ContextFlow) != "" {
		return
	}
	s.mu.Lock()
	projectPath := strings.TrimSpace(s.activeWorkspacePath)
	s.mu.Unlock()
	if projectPath != "" {
		resp.ContextFlow = launcherFlowProject
	}
}

// launcherBridgeBroadcastCheckpoint 桥接播报检查点（案 B 承接核心；handleChat 在
// 315 自动开场检查点之后调用——binding 工程已由 315 先行接管，本检查点对其静默
// 让位）。
//
// 触发条件（全查）：
//  1. 该会话在 launcher 卷有过轮次（rememberLauncherFlowConversation 标记——
//     跨卷衔接只对 launcher 起源的会话发生，普通工程进入行为零差异）
//  2. 工程已绑定（activation 后 activeWorkspacePath 非空）
//  3. 无乐局 binding（binding 在案=315 承接域，fail-closed 让位，零干预）
//  4. 该（会话,工程）尚未播报过（LoadOrStore 原子领取，会话级一次性）
//  5. 非空/斜杠/乐局命令词消息（归既有命令面，315 同款守卫）
//
// 播报只读不动状态：不写 binding、不建乐局会话、不触 authority/pendingmanager；
// 失败面只有摘要模板内部的 os.Stat（fail-soft 如实降级），不存在部分播报。
func (s *Server) launcherBridgeBroadcastCheckpoint(_ context.Context, conversationID, mode, text string) (ChatResponse, bool) {
	if s == nil {
		return ChatResponse{}, false
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || strings.HasPrefix(trimmed, "/") {
		return ChatResponse{}, false
	}
	if _, _, _, _, _, ok := parseEduJamCommand(trimmed); ok {
		return ChatResponse{}, false // 命令词归既有命令面（315 同款守卫）
	}
	if _, origin := launcherFlowConversations.Load(conversationID); !origin {
		return ChatResponse{}, false // 非 launcher 起源：普通工程进入零差异
	}
	s.mu.Lock()
	projectPath := strings.TrimSpace(s.activeWorkspacePath)
	projectUUID := strings.TrimSpace(s.activeWorkspaceUUID)
	s.mu.Unlock()
	if projectPath == "" {
		return ChatResponse{}, false // 未绑定：仍在 launcher 卷，无桥接可言
	}
	if _, hasBinding := loadLegameBinding(projectPath); hasBinding {
		return ChatResponse{}, false // binding 工程=315 自动开场/恢复播报承接域
	}
	bridgeKey := conversationID + "\x00" + projectUUID + "\x00" + projectPath
	if _, served := launcherBridgeServed.LoadOrStore(bridgeKey, true); served {
		return ChatResponse{}, false // 同会话同工程只播报一次（原子领取防并发双发）
	}
	resp := ChatResponse{ConversationID: conversationID, AgentMode: mode}
	resp.Reply = launcherBridgeSummaryReply(projectPath)
	resp.ContextFlow = launcherFlowProject
	resp.StopReason = launcherBridgeStopReason
	resp.WorkflowData = map[string]any{
		"context_flow": launcherFlowProject,
		"project_path": projectPath,
		"project_uuid": projectUUID,
	}
	resp.TypedEvents = []map[string]any{launcherVolumeSwitchEvent(projectPath, projectUUID)}
	return resp, true
}

// launcherBridgeSummaryReply 桥接播报确定性模板（工程名+最近修改时间，零 LLM；
// 设计稿 §2.2「已进入《X》。上次……」形态）。mtime 取工程文件 os.Stat——fail-soft：
// 读不到如实说没读到，不猜测不编造。
func launcherBridgeSummaryReply(projectPath string) string {
	name := launcherProjectDisplayName(projectPath)
	when := "没读到最近修改时间"
	if info, err := os.Stat(projectPath); err == nil {
		when = "最近修改于 " + info.ModTime().Format("2006-01-02 15:04")
	}
	return fmt.Sprintf("已进入《%s》。%s——想从哪儿继续？", name, when)
}

// launcherVolumeSwitchEvent 卷切换 typed_event（设计稿 §1.2 分隔条/§2.2 切换时
// 时机：切换由 UI 回执驱动完成后，agent 在确认绑定态的首轮回执上补发标记——agent
// 不预写工程卷状态）。前端据此插「── 进入工程 · 《名称》 ──」分隔条与流徽章对账。
func launcherVolumeSwitchEvent(projectPath, projectUUID string) map[string]any {
	event := map[string]any{
		"type":         "volume_switch",
		"context_flow": launcherFlowProject,
		"project_name": launcherProjectDisplayName(projectPath),
		"project_path": projectPath,
	}
	if strings.TrimSpace(projectUUID) != "" {
		event["project_uuid"] = projectUUID
	}
	return event
}
