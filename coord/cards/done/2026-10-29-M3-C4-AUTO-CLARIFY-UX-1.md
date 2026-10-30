# M3-C4-AUTO-CLARIFY-UX-1：编舞澄清/确认面五断点——span 降级利用+答非所问引导+裸单端候选+完全权限桥接+权限持久化核查（218 手测反馈快修）

- 发卡：GLM 主管决策流 / 2026-10-29 深夜（依据=用户收口话术手测三轮 clarify 断点+二轮确认卡权限疑问；agent_last.log 23:33 三轮 clarify+23:5x 确认链亲证+权限链代码亲读）
- 派发确认：已确认（用户 2026-10-29 深夜「归纳好开卡明天继续修」）
- 验收负责人：GLM 主管决策流
- 池序 219；目标仓库=**主仓 agent/internal/chat/ 域**；P1 / 0.5 天 / 依赖=218 已合入 ✓；L1 量产档可做

## 手测复盘（断点定位，2026-10-29 23:33+23:5x 会话）

**二轮结果**：修正话术「前 30 秒…」直通确认卡→确认→写入生效（用户亲证包络线落地）——正面链 ✓。

1. 「副歌那 30 秒，…」→通道 5 命中 section=副歌+span=30→**工程段落标记缺失**→整段降级 clarify，span=30 信息被丢弃（automation_intent.go:227-241）。
2. 裸「30 秒…」→五通道全不匹配（单端无前缀不认）→clarify。
3. 用户回「确认写入」→澄清阶段不识别该词→重复同文案 clarify（automation_choreo.go:435/:726）——用户感知「打字无效」。
4. **完全权限疑问两层根因（主管亲读）**：
   a. **用户手测时权限模式未生效**——agent 启动 restore 采纳盘面 mode=manual_confirmation（agent_last.log :22 亲证；server.go:7884 adopted 分支）——用户 webui 完全权限设置未持久化到该状态面或写入面不同步（webui 开关写的面 vs restore 读的面需核对）。
   b. **choreo 确认卡未接完全权限桥**——full_access_semantic_chain.go 既有桥面（插件推荐选择/插件装载确认完全权限下 full_access_auto_authorized 自动放行）不含 choreo 面；automation_choreo.go 零查询 authority mode——完全权限即使生效编舞确认也照弹（218 接线缺口）。
5. 注：链路行为本身合规（不猜原则/文案有格式示例）；缺陷面=体验与接线五断点。

## 目标（五小面）

1. **span 降级利用**：section 命中但标记缺失且 span 在场（「副歌那 30 秒」）→不再纯丢弃：澄清问句升级为**带候选确认**（「副歌标记未找到；用开头 30 秒可以吗？回'可以'或给绝对区间」）——回「可以」=region 0..span 进确认卡。不猜原则不破（用户显式认可候选，非系统猜）。
2. **答非所问引导**：澄清 pending 时收到「确认写入/写入/可以吗的肯定词族」→差异化回复（「还在等时间区间，尚未生成方案——例如'0 到 30 秒'；回'可以'采纳候选区间」），不再原样重复问句。
3. **裸单端候选**：裸「N 秒」（无前/到/段落词）→澄清问句带候选（「是从 0 开始的 N 秒吗？」）——同面 1 的候选确认形态。不直接默认（歧义风险）。
4. **choreo 确认接完全权限桥**：完全权限显式上下文（authority_mode=full_project_access+authority_mode_explicit——**裸模式串永不授权**红线维持）下编舞确认卡自动放行（full_access_auto_authorized+intervention_skipped 同既有桥形态，与 pluginRecommendation/pluginGrabber 同族入 full_access_semantic_chain）。
5. **权限持久化核查**：webui 完全权限开关写入面 vs server restore 读取面（chat state AuthorityMode）是否同一持久化链——不同步则修同步（先勘察定案根因再修，回执披露写入面锚点）。

## 探针义务

场景/单测断言五面（span 候选链/答非所问引导词族/裸单端候选/完全权限桥放行+裸模式串拒绝/权限持久化往返）；既有 218 意图测试族+full_access_semantic_chain 测试族回归零回退；真栈腿可选（确定性词表面单测覆盖充分，按执行侧裁量+边界声明）。

## 文件域

`agent/internal/chat/automation_intent.go`+`automation_choreo.go`+测试。越域即停。

## 停止条件

- 候选确认形态与既有 pending 状态机冲突（状态分支不可扩展）→锚点上交两案。

## 文件域（五面版）

`agent/internal/chat/automation_intent.go`+`automation_choreo.go`+`full_access_semantic_chain.go`（桥面 4）+权限持久化涉及文件（面 5 勘察定）+测试。越域即停。

## 领取（2026-10-30）

- owner=ZCode GLM-5.3-Flash PC 执行会话（主检出直接实现，无 worktree）；领取基线：HEAD=61a25d33、`git status --short` 脏文件全部位于本卡域之外（VitApp/Workspace 运行态+coord/runs 工件+webui/design 未跟踪件），`git diff --stat` 6 文件与本卡域零相交；领取前主检出无其他卡在飞（真栈腿未跑，无栈占用）。
- 按用户指令直接在主检出工作树实现；**不提交不 reset，工作树改动即成果**（§12 纪律，待主管验收后按依赖顺序形成 commit）。

## 回执（执行侧自验完成，待主管验收）

- 变更四文件+一新增测试：`automation_intent.go`(+25/-7)、`automation_choreo.go`(+212)、`full_access_semantic_chain.go`(+47)、`turn_control.go`(+11/-7 面五勘察定)、`automation_clarify_ux_test.go`（新增 10 测试）。diff 亲读零越域。
- **五面证据锚点**：
  1. **面1 span 降级利用**：`automation_choreo.go:430-436`+`:913-918`（两入口 marker 缺失且 `region.SectionSeconds>0` → 改走 span 候选澄清，不再纯丢弃）；候选澄清器 `automationChoreoClarifySpanCandidateResponse` `automation_choreo.go:547`（问句含「用开头 30 秒可以吗？回「可以」」+section 名披露；候选搭乘完整已解析目标上下文 TrackID/Plugin/Param `:563-573`）；显式采纳链 `automationChoreoAdoptSpanCandidate` `automation_choreo.go:651`（region 0..span→重建 points→进**确认卡**非直执 `:693-695`；超上限如实拒绝 `:660-668`）；采纳门=pending 路由 `automation_choreo.go:788-796`（内嵌绝对区间答案优先走 merge 路径、采纳仅限显式肯定词）。
  2. **面2 答非所问引导**：`automationChoreoAffirmAnswer` `automation_choreo.go:577`（紧词族+8 rune 长度帽，整句答案一律放行 merge）；引导响应 `automationChoreoClarifyAffirmGuidance` `automation_choreo.go:616`（按 clarify kind 差异化：「还在等时间区间，尚未生成写入方案…确认类回复要等方案卡出来后才生效」，不再原样重复问句）；pending 接线 `automation_choreo.go:780-786`（非候选类澄清+裸肯定词→引导，澄清 pending 原样保活不失效）。
  3. **面3 裸单端候选**：`automation_intent.go:155`（`BareSpanSeconds` 字段）+`:242-250`（绝对窗口/段落词全 miss 后 automationNumberRe 提取裸「N 秒」为候选）；两入口接线 `automation_choreo.go:440-444`+`:921-926`；问句「是从 0 开始的 N 秒吗？」`:551`。
  4. **面4 choreo 确认接完全权限桥**：桥门 `fullAccessChoreoConfirmationContext` `full_access_semantic_chain.go:64`（=explicitFullAccessRequestContext 显式对——authority_mode=full_project_access+authority_mode_explicit，裸模式串永不构成授权，与插件族同门）；桥执行 `automationChoreoFullAccessExecution` `full_access_semantic_chain.go:76`（stop-pending 先读 FS-STOP-APPLY-1 Part A `:81-83`→expire→同一条 executeAutomationChoreo 治理腿零降界 `:86`→家族标记 authority_mode/full_access_auto_authorized/confirmation_bypassed/interaction_card_issued=false `:87-90`→仅成功回执加「自主执行（无确认卡）」前缀 `:95-98` 家族约定）；三放行点=两入口确认卡发出点 `automation_choreo.go:481`+`:948`+采纳路径 `:678`；**澄清问句在完全权限下保持用户面对**（信息收集非干预，卡面口径 `full_access_semantic_chain.go:55-62`）。
  5. **面5 权限持久化核查**：**勘察结论=同一持久化链，无第二链**。写入面锚点：webui `src/lib/api.ts:66-67` setAuthorityMode→POST `/agent/authority`→`turn_control.go:152` handleAuthorityMode→内存置位+显式闩 `:188-191`→`persistCurrentProjectWorkspaceChecked`→`server.go:7614` persistActiveProjectWorkspaceLocked→`server.go:7651` `AuthorityMode` 字段→`history.go:1176` WriteAgentRuntimeState；显示面同源（`server.go:1255` /agent/ui/state 载荷=authorityModeSnapshot，webui 无本地权威副本）。读取面锚点：`server.go:7857-7887` restore（adopted 分支 :7886）。**定案根因（三个丢失窗口，事故日志无法裁决具体哪个——切换发生时的进程日志已被轮转）**：(i) workspace 未激活时 persist **静默 no-op 返回 nil**（`server.go:7615-7617`）→开关 200 OK 但只活在内存、重启即失；(ii) 409 拒绝（Turn 运行中切换，webui 已有 notice 面）；(iii) 跨工程身份按设计隔离（AUTHORITY-LOST-1 `server.go:7469-7483`，A 工程的完全权限不漏入 B 工程=用户既有裁定）。**修复=窗口(i)关静默**：`turn_control.go:196-210` persisted/workspace_ready 字段进响应+未持久化 WARN 日志（「a restart loses this selection」）。事故日志亲证：`agent/bin/VitApp/Workspace/Logs/agent_last.log:22`=`2026-10-22T21:05:40 [authority] restore adopted disk mode=manual_confirmation`，全进程无任何 switch accepted 行。
- **测试**：新增 10 测（`automation_clarify_ux_test.go`）：裸 span 解析分界(:30)/候选问句不丢 span+目标上下文存续(:52)/显式采纳链+绝对区间优先+「可以吗」不采纳(:81)/引导不重放+澄清保活+真实区间答案走 merge(:128)/两词族紧致性(:164)/桥门四例含裸模式串拒绝(:188)/完全权限零卡直执+无 pending 残留(:203)/manual 保持确认卡字节形态(:231)/开关持久化披露(:252)/**持久化往返**（写盘→restart 形状重载→采纳，含事故形态 manual 采纳证 :289）。回归：既有 218 意图测试族+full_access_semantic_chain 测试族+authority_restore_logspam 族零回退。
- **验收命令与退出码**：`go build ./...` exit 0；`go test ./internal/chat/ -count=1` ok（148.6s）；`go test ./... -count=1` **96 包全 ok exit 0**。
- **真栈腿**：未跑（卡面「可选」，执行侧裁量依据：五面均为确定性词表/路由/持久化面，单测覆盖充分；且面4 真栈放行依赖面5 的权限生效链先由用户手测复核）。**边界声明**：真栈未验证面=完全权限下编舞全链真栈放行、webui 开关→重启→生效的用户旅程（需用户手测或后续烟测卡）；单测已覆盖=桥门语义+桥执行形态+持久化往返文件链。
- 停止条件：未触发（候选确认形态走既有 Clarify map 扩展 kind=region_span_candidate，pending 状态机零冲突）。
- 手测建议（供主管转交用户，按 AGENTS §5 神前端入口）：①「副歌那 30 秒…」（无标记工程）→预期不再吞 span，问「用开头 30 秒可以吗？」→回「可以」→确认卡区间 0-30s；②裸「做 30 秒的渐强」→预期问「是从 0 开始的 30 秒吗？」；③澄清等待时回「确认写入」→预期得到「还在等时间区间」引导而非重复问句；④webui 开完全权限→同话术→预期无确认卡直执；⑤开完全权限后**重启 agent**→GET /agent/authority 应仍为 full_project_access（若为 manual，即命中窗口(ii)/(iii)，回执根因段可定位）。

## 验收（主管裁定 2026-10-30 晚，GLM 主管决策流）

- **整卡 pass（代码面）**。diff 四文件+新增测试逐行亲读零越域；五面证据锚点与实现一一核对：
  - 面1/3：BareSpanSeconds 提取边界（>0 且 ≤max）+两入口接线+采纳门=显式肯定词族且**内嵌绝对区间答案优先走 merge**（采纳路径不吞改口）✓；采纳后进确认卡非直执，不猜原则维持 ✓。
  - 面2：紧词族（8 rune 帽整句放行）+四 kind 差异化引导+pending 保活 ✓。
  - 面4：桥门=explicit 对（无链 key 的单 issuer 理由成立）；三放行点齐全；桥执行 stop-pending 先查+同一治理腿+「自主执行」前缀仅 mutation_performed 时加（诚实边界）；澄清问句保持用户面对符合卡面口径 ✓。
  - 面5：勘察定案「同一持久化链+三丢失窗口」与主管 10-29 亲读一致；修复=窗口(i)关静默（persisted/workspace_ready 披露+WARN）——最小如实披露不改行为，事故日志不可裁决三窗口的现实约束下是正确形态 ✓。
- **抽查复跑**：`go build ./...` exit 0；`go test ./internal/chat/ -count=1` ok 134.7s（回执 148.6s 同族）。测试抽查两关键件（采纳链不猜三段/持久化往返含事故形态）：断言真实、t.TempDir 合规。
- **真栈腿**：执行侧依卡面裁量未跑（可选项+边界声明）。**裁定=接受裁量，真栈验证移交用户手测**（五面均为用户可感知对话行为，手测即真实栈覆盖；完全权限真栈放行面由手测步骤④⑤覆盖）。手测步骤按回执建议①-⑤交用户。
- **边界遗留**：面5 窗口(ii) 409 与窗口(iii) 跨工程隔离维持既有设计（后者=用户既有裁定），不在本卡扩面。
- 形成 commit（本验收记录随入）；收口话术终验（完全权限形态）待用户手测后随 220 结论一并收口。

## 方向补记（2026-10-30 晚，判断权归属裁定）

面 1/2/3 的固定澄清文案列入 225 迁移清单（[2026-10-30 判断权归属决策](../../decisions/2026-10-30-judgment-ownership.md)：判断类决策不再新增 Go 固定文案；既有面降级为 LLM 不可用兜底）。本卡验收结论不变（断点修复有效，用户手测亲证）；面 4 真栈接线缺口由 223 承接。
