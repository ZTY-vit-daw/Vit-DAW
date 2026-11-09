package chat

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

// ── LAUNCHER-CHAT-BE-1（332 拆卡 C4，2026-11-09）─────────────────────────────
// launcher 流（启动卷）确定性意图路由 + 门厅简版开场白。
//
// 边界（设计稿 ai-first-launcher-design-v1 §2.2/§3.1/§4.3）：
//   - 触发条件 = chat 请求 context.launcher=true 且无活跃工程（F11：projectUUID
//     为空时 activation 早退，会话走进程态）。有活跃工程时的跨工程意图归
//     LAUNCHER-BRIDGE-BE-1（C5）治理链，本分支不路由。
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

// launcherFlowApplicable 报告本轮 chat 是否走 launcher 分支：context 标记
// launcher=true 且当前无活跃工程（CurrentProjectIdentity 与 note_sessions
// 同源；有工程即退出，跨工程意图归 C5）。
func (s *Server) launcherFlowApplicable(ctx context.Context, chatCtx map[string]any) bool {
	if !launcherContextActive(chatCtx) {
		return false
	}
	if s == nil || s.harness == nil {
		return false
	}
	_, projectUUID := s.harness.CurrentProjectIdentity(ctx)
	return strings.TrimSpace(projectUUID) == ""
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
	intent := launcherClassifyMessage(message)
	if intent.Kind == launcherIntentNone {
		return ChatResponse{}, false
	}
	resp := ChatResponse{ConversationID: conversationID, AgentMode: agentMode}
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
