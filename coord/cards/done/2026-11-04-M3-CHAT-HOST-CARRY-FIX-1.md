# M3-CHAT-HOST-CARRY-FIX-1：终局轮披露跨 run 边界存活（面一 T1）+前台吞回执升级与终局补投递（面三 T1）

- 发卡：GLM 主管决策流 / 2026-11-04（依据=244 勘察报告 [2026-11-04-m3-breakpoint-forensics-1.md](../reports/2026-11-04-m3-breakpoint-forensics-1.md) §1.2/§1.4/§3.1/§3.3+裁定 [2026-11-04-M3-BREAKPOINT-FORENSICS-1-pass.md](../rulings/2026-11-04-M3-BREAKPOINT-FORENSICS-1-pass.md)）
- 派发确认：已确认（链内续卡，主管发卡；用户口头开工即有效）
- 验收负责人：GLM 主管决策流
- 池序 245；目标仓库=主仓 agent；P1 / 0.5 天 / 依赖=244 报告已在 main（319e3444）✓；L1 / PC 执行会话
- **并行注记（§2.2 ③）**：本卡与 246（M3-TERMINAL-ROUTE-WORDING-1）同时派发，文件域不相交；两卡均含真栈腿——**错峰执行，本卡先跑**，246 见本卡释放登记后再登记启动。

## 目标（三面，全部确定性判据）

1. **面一 T1 披露跨边界存活**：`agent/internal/chat/free_state_reasoning_loop.go`——`freeStateReasoningLoop` struct 增 `AutomationSurfaces []map[string]any` 字段（json tag `automation_surfaces,omitempty`）+`freeStateLoopFromAny`/`freeStateLoopMap`/`cloneFreeStateLoop`/`mergeFreeStateLoops` 对应携带与合并分支（沿用行合并+`project_revision` 严格前进过期语义与上限 `freeStateAutomationSurfaceLedgerLimit=8`，锚点见报告 §1.2 步骤 1/3）。
2. **面三 T1a 吞回执升级**：`agent/internal/chat/goalrunner_chat.go:194-204` 邻接——`automatic_continuation_already_scheduled` 回执的 `WorkflowData` 携带 pending continuation 当前状态（attempt/status 之外补最近一次终局/观察回执摘要，pendingmanager 已有面可复用则复用，不新建协议）。
3. **面三 T1b 终局补投递**：goal 终局落状态处（`goalrunner_chat.go:3359-3381` 记账邻接）对原 conversation 补投递一条终态摘要回执（capability_blocked/needs_experiment+提案概要，一段文本，无新 UX 面）。

## 判据与验收标准

- **单测**：①round-trip 保键（map 含 automation_surfaces 行→`freeStateLoopFromAny`→字段在→`freeStateLoopMap` 回写键在）②merge 语义（revision 前进过期+上限 8）③吞回执携带状态断言 ④终局补投递断言；既有测试同步（回执文本/WF 数据变化影响的断言面）。
- **真栈腿**（-Groups 1 单组一次，deepseek-flash，载荷捕获全开）：**确定性 needle=终局轮请求载荷含 1011/0.740818**（模型行为面只记录不设门）+「继续」回执携带状态+终局摘要投递可见。脚本退出码 0；栈登记/释放全流程+TEARDOWN_ALL_ZERO。
- §8 预算 ≈¥1.5-2（单组单 run，止损三连同型自动中止沿用）；计费披露。

## 停止条件

- needle 仍缺披露（终局轮载荷无 1011/0.740818）→不扩大修复面，取证包上交（含边界导入链路逐点 dump）；
- 修复破坏既有测试（零回退红）→回退两案上交；
- merge/投递触点超出文件域→停上交。

## 文件域

`agent/internal/chat/free_state_reasoning_loop.go`+`agent/internal/chat/goalrunner_chat.go`+chat 包测试。越域即停上交。

## 领取

- 领取时间：2026-11-04T20:24+08:00；origin/main=2ad3346c（=依赖 244 报告合入基线 ✓）
- 执行 owner：GLM-5.3 Flash（ZCode PC 执行会话 2026-11-04，M3-CHAT-HOST-CARRY-FIX-1 专属会话）
- 协调 worktree：`D:\Vit_DAW_wt_245coord`（分支 `coord/245-claim`，仅卡片移动+回填）；实现 worktree：`D:\Vit_DAW_wt_245impl`（分支 `port/m3-chat-host-carry-fix-1`，从 2ad3346c 建立）
- 领取基线核对：主工作树 `git status --short` 既存改动（VitApp/Workspace/*.xml、coord/runs/UX-AB-*、FE-GUI-* 未跟踪等）与本卡文件域（`agent/internal/chat/free_state_reasoning_loop.go`+`goalrunner_chat.go`+chat 测试）不相交 ✓；`git diff --stat` 零 chat 域条目 ✓
- 与 246 错峰：本卡先跑真栈腿；246 待本卡释放 `coord/resources/PC-RUNTIME-STACK.md` 登记后方可登记启动

## 回执

- **执行侧自验完成，转 done 待主管验收**。实现 commit=fae86236（分支 `port/m3-chat-host-carry-fix-1`，基线 8d17f8b9；diff 恰好三文件=卡面文件域零越域：free_state_reasoning_loop.go +161-19/goalrunner_chat.go +196-3/free_state_chat_host_carry_test.go 新增 281 行）；工件 commit=14c3e173（coord/runs/M3-CHAT-HOST-CARRY-FIX-1/carryrun1/ + RECEIPT.md 判读；payload_capture 6.2M+prompt_render 3.4M 留 worktree，clip.wav 已删）。
- **面一 T1 ✓**：`freeStateReasoningLoop` 增 `AutomationSurfaces []map[string]any`（json `automation_surfaces,omitempty`）——fromAny/loopMap/clone 经 JSON round-trip 自动携带（字段注释锚定 244 报告 §1.2 步骤 3）；`mergeFreeStateLoops` 增合并分支（track_id+plugin_id 复合身份刷新/追加+`project_revision` 严格前进过期+上限 8 FIFO，chat 侧新增 `mergeFreeStateAutomationSurfaceRows`/`freeStateLoopProjectRevision`/`freeStateRevisionStrictlyNewer`/`freeStateRevisionTrailingNumber` 四 helper，语义逐条沿用 agentloop `freeStateAutomationSurfaceLedgerLimit`/`freeStateExpireAutomationSurfaceRows`/`freeStateRevisionStrictlyNewer` 并注释锚定）。
- **面三 T1a ✓**：裸「继续」吞回执（goalrunner_chat.go 自动续跑队列分支）WorkflowData 补 `free_state_summary`（decision_status/decision_summary 截断/proposal 概要/latest_observation 面/cycle/continuation_used+budget/terminal_turn_locked/observation_receipt_count；复用 `s.freeStateLoop` 既有面零新协议）。
- **面三 T1b ✓**：`recordGoalResult` 记账邻接（pendingEvents emit 前、锁外）对 scheduler 认领的链终片（判据=`freeStateSchedulerDrivenGoalLocked`——s.mu 持有区在 durable 状态改写**前**捕获 Running 记录+`res.Continuation==nil`+非 waiting_continue/非交互边界+loop 决策面存在）补投递终态摘要（`freeStateTerminalSummaryReply` 人话 lead+decision.Summary 截断+提案概要，经 `sanitizeSettledTerminalReply` 护栏；通道=`emitSchedulerChainResultEvent` 既有 scheduler_chain 事件零新协议零新 UX 面；webui chain_result goal 键幂等去重兜底双投递）。
- **单测**：卡面四件全绿+HTTP 反例（五个测试函数，`free_state_chat_host_carry_test.go`）：①round-trip 保键（fromAny→字段在→loopMap 回写键在→clone 深拷贝）②merge 语义（复合身份刷新/严格前进过期/等值与不可排序保留/无 revision 不过期/上限 8 FIFO 头删）③吞回执携带断言 ④终局投递断言+HTTP 路径不投递反例。既有测试同步：chat 包全量（128.9s）+全仓 `go test ./...` exit 0 零红。
- **真栈腿 ✓（carryrun1，单组单 run）**：ps1 整体 exit 0+场景腿 COMPLETED（-Groups 1 组过滤亲证）；deepseek-flash 注入亲证（telemetry 94/94 model=deepseek-flash）；kernel=wt_228 F9E3DA0A 只读复用启动前哈希亲核。**三判据全达成**：
  1. needle ✓：r1 记录 32/r2 记录 69 终局轮（TERMINAL TURN 指令在场）`automation_surfaces` 完整行在场（plugin_id "1011"+volume value=**0.740818202495575**（"+0.00 dB"）+pan+min/max 范围+project_revision=5）——**244 勘察的 run 边界 typed round-trip 丢弃形态真栈消失**；r3 全轮零 get_plugin_parameters 调用=模型行为面（零调用→零钉住行是正确语义，只记录不设门）；
  2. 吞回执携带状态 ✓：三轮 ex1 WorkflowData `free_state_summary` 全字段（r1 样例：needs_observation+人话摘要「时间型需求…缺时间轴定位」+观察面 ccb.observation_request/partial+预算 1/6+terminal_turn_locked=true）；
  3. 终局摘要投递可见 ✓：agent log `[chain.terminal-summary] delivered` r1（20:53:28）+r3（21:02:28）双亲证（decision=capability_blocked status=completed）+r2 经 chain-end gate 通道投递（[f6.gate] durable terminal+goal completed→第一分支 deliverSchedulerChainTerminal）。
- **附带观察（如实上交，未定位，不扩大修复面）**：r2 终局 run（20:58:56 完成，stop=capability_blocked）**无 [continuation.arm] 行**=recordGoalResult 未执行的证据，durable 终态=cancelled 而 goal=completed——capability_blocked 边界路由（capabilityBlockedBoundaryResponse/improvementProposalResponse 均调 chatResponseFromAgentLoopResult 理论必经）与实际日志矛盾，未定位；属既有路由面缺口非本卡引入（改动前同在）；r2 终局结果经 gate 通道投递未丢失。建议主管裁定是否转后续卡（needs_experiment 提案终局路径的 durable 记账面）。
- **计费披露（§8）**：94 次计费调用（零错误），in 1,445,971/out 135,898 tokens；粗上界 ≈¥2.53（flash 实际单价更低+cache 折扣，真实成本低于粗上界）——超卡面 ≈¥1.5-2 线如实披露（r2 切片长链 36 调用/308s+三样本全跑；单组单 run 无重跑，止损未触发，no_route×3 为组内判定自然收尾非 env 中断；模型行为面不设门）。
- **栈纪律（§2.2）**：占用 7d5bf94c（登记前 20:25:48 亲核全零+push 后重读 owner 确认+启动前 20:46:48 ≤60s 即时复核全零）→释放 e089cbfe（21:08:59 TEARDOWN_ALL_ZERO 亲核：PROCS=0+TCP 5555/5556/7878/8787/18787/5178=0+UDP 4444/4445=0，owner 手拆 VitAgent 41076/VitApp 27984）；**错峰解锁：246 自 e089cbfe 起可登记占用**。
- 端测覆盖边界声明：本卡改动面=chat 包 Go 两文件+测试；无 webui/渲染面改动（webui chain_result 合成与 goal 键去重为既有面，T1b 复用未改）；真栈腿覆盖 agent HTTP/ZMQ 面+载荷捕获面；webui 渲染面（终局摘要气泡合成）未在本腿目检——由既有 chainResultDelivery.test.ts 单测面与主管裁定覆盖边界。

## 验收

- **pass（实现合入 b9e14639）**；裁定=[2026-11-04-M3-CHAT-HOST-CARRY-FIX-1-pass.md](../../rulings/2026-11-04-M3-CHAT-HOST-CARRY-FIX-1-pass.md)。
- 亲核：面一 diff 亲读（字段+四 helper 语义沿用写侧）/chat 包亲跑 ok 122.1s/needle 亲证（终局轮定位法：r1 conv 行 31+r2 行 35 披露全在场，244 丢弃形态真栈消失；r3 零 get_parameters=无源可携正确语义）/T1a 三轮全字段/T1b 证据等级注（代码面+单测+回执引用，原始 agent log 未入库记工件卫生小瑕疵）。
- 计费双口径入册（¥1.74 in+out / ¥2.53 含 cache 上界）；附带观察（r2 recordGoalResult 缺席+durable=cancelled）裁挂账：绿→并入链收口清理卡，红→并入下次取证包。
