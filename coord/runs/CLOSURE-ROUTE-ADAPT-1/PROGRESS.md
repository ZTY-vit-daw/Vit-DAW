# PROGRESS — CLOSURE-ROUTE-ADAPT-1 勘察腿（池序 271）

- Owner: GLM 主会话 freeflash 子代理（勘察腿，读档零 LLM 零代码改动）
- 开工时间: 2026-11-07
- 任务: 路由面取证 + 适配方案设计，产出 recon-design.md 交主管裁定
- 工件目录: D:\Vit_DAW\coord\runs\CLOSURE-ROUTE-ADAPT-1\

## 步骤日志

| # | 状态 | 内容 | 产物/证据 |
|---|---|---|---|
| 0 | DONE | 检查工件目录（不存在，新建）+ 读卡面全文 | 本文件；卡面 D:/Vit_DAW/coord/cards/todo/2026-11-07-CLOSURE-ROUTE-ADAPT-1.md |
| 1 | DONE | git fetch（origin/main=a8be057e）+ worktree D:/Vit_DAW_wt_271impl @ port/closure-route-adapt-1（新建 clean）+ 卡面 todo→doing 领取回填，claim commit 08ecec8c 已推 main | worktree=D:/Vit_DAW_wt_271impl；commit=08ecec8c |
| 2 | DONE | 路由面取证完成（判据链 6 环+分析类归类现状+武装入口过滤现状，见 PROGRESS 中期要点 1-7 与 recon-design.md §1-§3） | recon-design.md |
| 3 | DONE | 预算面取证完成：MaxTurns=1 源自 baseline 38ecd24b（2026-08-19）注释逐字同今=有意设计；但 diagnostic_only base=5（goalrunner_chat.go:801-807）被 closure clamp 覆盖回 1=同一诊断语义两个预算值并存（欠账形态）；测试钉 TestAudioClosureBudgetDiagnosticRoundKeepsClamp 钉死 1；276 卷内确认披露面已含 analyze_midi_transformations（工具可达非失败因素） | recon-design.md §4 |
| 4 | DONE | 适配方案设计 3 案（A 路由面/B 预算面/C 工具面）+案 D 驱动腿+判断权边界标红 2 处；建议=B 主案+A 提请裁定 | recon-design.md §5-§6 |
| 5 | DONE | recon-design.md 落盘 + 卡面（doing）勘察腿回执回填 | recon-design.md；coord/cards/doing/2026-11-07-CLOSURE-ROUTE-ADAPT-1.md |

## 终态

**DONE**（2026-11-07）——勘察腿完成。方案要点一句话：分析类请求误入 closure 的根因是 semantic_entry LLM 在 discussion/observation 判据边界摇摆且 observation 被无条件派给 MinimalAudioClosure 闭包（ModeDiagnostic、1-turn clamp），建议主推案 B（诊断闭包轮预算面最小修，承认 base 预算，零判断权边界触碰），案 A（路由面拆纯分析身份）交判断权归属裁定后叠加。

## 长命令记录（跑之前先记）

（暂无）

## 中期取证要点（滚动记录）

1. 路由判据链核心：`semantic_entry.go:279-290 defaultSemanticEntryController` — discussion/other→OrdinaryConversation；**observation 与 open_semantic 都→MinimalAudioClosure**（:285-286）。
2. `capability_routing.go:269-327 planObservationFirstCapabilityRoute` — observation/open_semantic 一律容量评估；非 ProjectMix 且非短路→无条件 `entry.Controller = MinimalAudioClosure`（:308）；`promoteOpenImprovementEntry`（:363-371）只把「工程+问题词」的 observation 提升 route 标签为 open_semantic，**不改 controller**；「分析这段 MIDI 的变换关系」无 improve/问题词→保持 observation 但仍进 closure。
3. closure 武装入口：`audio_closure_controller.go:108-119` — 判据仅 `decision.Controller == MinimalAudioClosure` + verified semantic entry；`entry.Route==observation → ModeDiagnostic`（:117-119）。**没有「分析类不进 closure」的过滤**；ModeDiagnostic 是 observation 路由在 closure 内的合法身份（`audioClosureSourceRoute`/`audioClosureAuthorization` :356-367 双向映射）。
4. free-state 自动化轮（48-turn 大预算）只对 open_semantic 启动（`free_state_reasoning_loop.go:298-315`，要求 route==open_semantic+semantic_loop+action）；observation/diagnostic 闭包轮不满足→走 1-2 turn clamp。
5. 预算面：`audio_closure_controller.go:524-546 audioClosureMessageLoopBudget` — 非自动化轮一律 `base.MaxTurns=1`，仅 frontier 有候选且无 candidate_id 时=2（:530-533）；自动化轮 48/80（:478-481，M3-LLM-TURN-BUDGET-1，232 用户裁定 relay baton）；diagnostic-only verbatim 保持 clamp（:442-444,505-509）。
6. **276 rerun1 卷内事实修正**：chat_response.json 实际 closure=`mode:diagnostic, phase:fs2_capacity_assessed, frontier 空(candidates=0)`→实际生效 MaxTurns=1；rerun1 evidence.md「frontier 有候选=2」与卷内 JSON 不符（需在方案文档中标注）。
7. `free_state_diagnostic_only` 生产代码只读不写（grep 全库仅消费点），来自持久化 loop 行/外部 context。

## 遇到的问题

（暂无）
