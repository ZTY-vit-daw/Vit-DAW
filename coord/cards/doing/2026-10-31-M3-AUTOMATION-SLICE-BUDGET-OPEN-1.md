# M3-AUTOMATION-SLICE-BUDGET-OPEN-1：automation 决策轮片预算放开至天然停止点+smoke 终态等待保险丝（229 验收门第四承，用户裁定 2026-10-31）

- 发卡：GLM 主管决策流 / 2026-10-31（依据=231 验收定案+用户裁定：采样协议与异步执行模型错配；限轮位置从每片挪到总量——slice 预算对 automation 轮放开，MaxClosureRounds 总护栏保留，治理门/限速零改动；裁定全文 coord/decisions/2026-10-31-slice-budget-position.md；事实核查=Claude Agent SDK headless 默认 10 轮同款、交互式主流不限轮、边界控制三轨正交共识）
- 派发确认：已确认（用户 2026-10-31「可以发卡」）
- 验收负责人：GLM 主管决策流
- 池序 232；目标仓库=**主仓 agent chat 预算域+scripts**；P1 / 0.5 天 / 依赖=231 已合入（69aec89b）✓；L2 / PC 执行会话（预算语义+LLM 判读）

## 标的（231 定案+用户裁定，不重考）

231 实测：模型每同步窗口仅完成 2-3 工具调用（checkpoint+catalog+observation）即耗尽 10 轮片预算→limit_reached→异步续跑排队文本→no_route×3+拆栈杀续跑。完整链需约 8-10 工具调用（catalog→观察×2-3→get_plugin_parameters→set_points（治理门暂停）→ResumeAfterConfirmation→get_curve→汇报），deepseek-flash 开销率约 3 轮/工具（231 遥测：9-10 轮/3 工具）→链需约 25-30 轮，片预算 10 结构性不够。**每片 10 轮是切片机制不是安全边界**——边界主体（治理门/每轮一 mutation 限速/MaxClosureRounds 总护栏/「取消」打断）已建齐且零改动。

## 目标

1. **生产面：automation 决策轮片预算放开**——`audioClosureMessageLoopBudget` 对 `audioClosureAutomationRoundContext` 命中轮返回**余量预算**：推导成文（建议区间 40-48 turns / 对应 MaxToolCalls 同比放大；依据=231 遥测链长×1.5 余量，推导过程入回执），使一轮请求可跑到**天然停止点**（确认卡 waiting_interaction 暂停/完成汇报/capability_blocked 申报）。**MaxClosureRounds 总护栏、治理门、每轮至多一 mutation 限速、223 stamp、非 automation 观察 slice 语义全部零改动**（229 测试五面零回退）。
2. **仪器面：smoke 终态等待保险丝**——`automation_llm_route_smoke.py`：①进度/排队文本（limit_reached 续跑提示/automatic_continuation_already_scheduled）不再即判 no_route，改为**有界等待终态**（分钟窗，轮询 goal/task 终态）；②终态分派=waiting_confirmation 确认卡在场→**驱动确认**（「确认」话术或 218/226 断言体先例驱动面）→验治理写入+get_curve 逐点读回=success；capability_blocked→按 230 分型；等待窗尽仍无终态→新型 in_progress_timeout 如实分类（不硬塞 no_route）；③判定逻辑其余原口径不动（三组×5 轮、组级 ≥4/5、同型三连止损承 220）。
3. **重验腿（本卡验收门，229 原口径第四承）**：组 1「这段让它越来越突出一点」先行——**组 1 no_route 消失+set_points 确认链（确认卡→确认→写入→get_curve 读回）首次端到端全绿**；组 2/3 按 220 口径跑满。

## 文件域

`agent/internal/chat/audio_closure_controller.go`（预算面，229 同域）+预算来源邻域（勘察定：mode 基础预算的派生面）+测试；`scripts/automation_llm_route_smoke.py`（仪器面）。越域即停上交。

## 安全面（硬约束）

- 治理门/每轮一 mutation 限速/MaxClosureRounds/stop-pending/隔离/权限闩零回退——**预算放开不得成为无治理连写通道**；
- 放开仅限 `audioClosureAutomationRoundContext` 命中轮（229 识别面复用，不新增识别逻辑）；诊断轮/非 FS closure 轮钳制语义零回退；
- §8：LLM 真实计费，轮窗放大后单轮调用数可能上升，回执披露调用数与 token；止损纪律不变。

## 停止条件

- 预算放开与 closure 收敛/调度语义冲突不可调和→锚点+方案上交；
- 放开后模型超余量预算仍不到天然停止点（如 48 轮仍 limit_reached）→**模型能力面证据上交**（verbose 率过高，转模型/指引设计讨论：决策轮换引擎或指引 nudge——届时带实测数据）；
- 等待终态窗内确认卡出现但驱动确认失败→断言体缺陷域内修；治理链写失败→write_fail 如实分类锚点上交；
- LLM 面栈不可用→环境中断如实记录。

## 领取

- 领取时间：2026-10-31；执行 owner：GLM-5.3 PC 执行会话（ZCode 池序 232 会话）；验收负责人：GLM 主管决策流（卡面既有）。
- 领取基线 `origin/main` = `7bee7942`（= 本会话开始时主仓 HEAD；发卡提交本身）。
- 领取时主工作树状态（权威输入，零丢弃）：`git status --short` 显示 6 个修改文件（VitApp/Workspace 运行时状态 2 + coord/runs/UX-AB-AUDITION-IMPL-1/aba174 run 工件 4）+ 未跟踪（.zcodeignore、agent/coord/、agent/webui/design/mondrian-route-* 设计件、coord/reports/3 份、coord/runs/ 多卡工件目录）——全部为其他卡 run 工件/运行时状态，与本卡文件域（agent/internal/chat 预算域 + scripts/automation_llm_route_smoke.py）零重叠；`git diff --stat` 仅上述 6 文件（4465+/4374-，Workspace.xml 运行时抖动为主）。
- 执行分支 `port/m3-automation-slice-budget-open-1`，worktree `D:/Vit_DAW_wt_232impl`（基于 origin/main 7bee7942 新建）；协调 worktree `D:/Vit_DAW_wt_232coord`（分支 coord/232-claim）。
- 文件域冲突核对：doing 池仅 2026-10-16-GEN-SUNO-WEB-DESIGN-1（文件域=coord/runs/GEN-SUNO-WEB-DESIGN-1/**，零生产代码），与本卡不相交。✓
- 真栈腿：PC-RUNTIME-STACK.md 登记承诺（§2.2 全流程：fetch 重读置顶+进程/端口全零亲核+登记先于启动+登记后 60 秒内即时复核）。
- §8 计费披露：见回执节（LLM 真实计费，重验腿全量披露）。

## 回执（2026-10-31，执行侧自验；卡留 doing 待主管验收）

### 实现与提交

- 实现 commit：**96a9b224**（`port/m3-automation-slice-budget-open-1`，已 push）；工件+证据 commit **01204592** + 入库边界修整 **0dd77c20**（运行时工程状态瘦身对齐 231 边界）。
- 生产面（`agent/internal/chat/audio_closure_controller.go`）：`audioClosureMessageLoopBudget` 对 `audioClosureAutomationRoundContext` 命中轮返回**余量下限 48 turns / 80 calls**（max 语义 floor：不剥离 settle/engagement headroom、不缩小更大 base；新常量+推导注释成文在 `audioClosureAutomationRoundMaxTurns`）。**推导**：231 遥测链长 8-10 工具 × 3 轮/工具（deepseek-flash 开销率）= 24-30 轮 ×1.5 余量（用户裁定口径）= 36-45 → 取 48（上沿+确认驱动/汇报缓冲）；MaxToolCalls 同比 48×(16/10)=76.8 → 80（≥ 原 mode 比例）。**Timeout 零改动**（180s slice 接力保留，异步续跑 229 side 2 为传输级活性机制）；MaxClosureRounds/治理门/每轮一 mutation 限速/223 stamp/非 automation 观察 slice 语义零触碰。
- 仪器面（`scripts/automation_llm_route_smoke.py`）：终态等待保险丝三要件落地——①`is_queued_continuation`+limit 文本不再即判 no_route→15 分钟有界窗（`TERMINAL_WAIT_TIMEOUT_SEC=900`）轮询 `/agent/runtime/status`（白名单 GET 姿态）goal+capability receipt；②终态分派：确认卡在场→驱动确认+`kernel_curve` 逐点读回=tool_chain_ok（含异步 settle 后写入的正确归因）；goal `capability_blocked`/receipt→230 分型；终局 completed/failed 无结构信号→按 summary 原口径 boundary/capability/兜底分类；③窗尽=`in_progress_timeout` 新型如实分类并并入止损集。判定逻辑其余原口径不动（三组×5/组级 ≥4/5/同型三连）。

### 测试命令及退出码（领取基线 7bee7942 上）

- `go build ./...` → exit 0
- `go test ./internal/chat -count=1` → **ok 140.434s**（含 229 五面）
- `go test ./internal/agentloop -count=1` → **ok 5.399s**
- `go test ./internal/chat -run 'TestAudioClosureBudget|TestFreeStateTerminalTurn' -count=1` → 全 PASS
- 229 五面处置：主面/续跑面按 232 新语义更新断言（48/80+floor 非钳制断言：60/100 base 不被缩小）；**钳制三面（诊断轮/终态 loop/legacy 观察）原口径零回退**。

### 真栈腿（§2.2 全流程）

- 登记 0a85e615（21:1x 置顶：fetch 重读 main=52f4bf1d 无占用+PROCS=0/TCP_LISTEN=0/UDP=0 亲核+登记先于启动）；启动前即时复核 RECHECK_ZERO（≤60s）；释放登记见 STACK 记录（21:3x，拆栈 VitAgent 6412/VitApp 24460→复核全零 TEARDOWN_ALL_ZERO）。
- 命令：`dev_agent_smoke.ps1 -StartKernel -RestartAgent -AutomationLLMRouteSmoke -NoChatSmoke -NoStripSilenceSmoke -KernelExe <wt_228 VitApp.exe f9e3da0a 只读复用> -RunArtifactsDir <slicebudget_1>`；**ps1 exit 0**（dev agent smoke completed）；kernel 运行前后零漂移。
- run 工件：`coord/runs/M3-AUTOMATION-SLICE-BUDGET-OPEN-1/slicebudget_1/`（随 port 分支入库；EVIDENCE.md=锚点+结构化证据）。

### 链路进度（§7 分列一）

- **两标的真栈生效亲证**：①预算放开——五个 capability_blocked 轮全部**同步终局**（agent timing 行：status=completed stop=capability_blocked，total_ms 80.8k-184.4k，completed_steps 3-6/executed 3-8），**无一轮被 turn 预算切断**；231 形态（2-3 工具即 limit_reached→排队文本淹没）消失；全 run 唯一 limit_reached=g1_r4（Timeout 180s 切片，设计内接力），调度器 250ms tick 正常续跑。②仪器面——g1_r4 全链在卷：limit_reached→「继续」→排队文本→等待窗 3 polls→goal 终局→按终局分类（无误判）；choreo-pending 不干扰腿 group_1=**pending_intact**。
- smoke 覆盖边界声明：本腿为 agent HTTP/ZMQ 接口面冒测（§5 门槛达标）；无渲染面/用户旅程面改动。

### 最终验收（§7 分列二）：229 第四承**未达**——根因转移两个越域生产缺陷，如实上交

- 结果：组1 5 轮 [capability_blocked, no_route, capability_blocked, no_route, no_route] successes=0（**no_route 未消失+set_points 端到端零次**）；组2 3 连 capability_blocked 止损；组3 按原口径跳过；status=COMPLETED。
- **缺陷 A（FS 准入 G6/G8 拒绝，capability_boundary 结构化披露，非模型话术）**：g1_r1/r3、g2_r1-r3 的 capability_blocked 均携带 `free_state_admission_receipt`——G1-G5/G7 全 pass，**G6_target_evidence + G8_target_consistency fail**（proposal_present/valid=true、target_evidence_ref 非空仍被判不可用）；`capability_boundary.unavailable_reasons=[certification_entry_unavailable_for_family, family_unresolved, no_loaded_instance_for_manual_guidance]`。锚点：`agent/internal/agentloop/free_state_gate.go:312`（gateG6）`:444`（gateG8）、`agent/internal/capabilityboundary/resolver.go:18`、`agent/internal/chat/free_state_reasoning_loop.go:3602`。g1_r5 stop=no_candidate_found（frontier 空）为同族邻域表现。
- **缺陷 B（taskstate 重复转移拒绝）**：g1_r4 终局 error=`durable continuation failed: minimal audio closure controller failed: semantic transition capability_blocked is not allowed from capability_blocked`——closure 已 capability_blocked 后续跑再次 settle 同型被状态机拒绝→goal failed。g1_r2 stop=failed（「结算失败」）疑同族。锚点：`agent/internal/taskstate/state.go:515-517`。
- 定性：模型行为面本轮**无反例**（无编造写入/无绕道/确认机制不受扰/能力申报全部有结构依据）——阻塞不在模型判读而在生产准入与 settle 语义；两缺陷均在本卡文件域外（agentloop/capabilityboundary/taskstate），按 §11 越域上交，未越域修。
- 卡面停止条件核对：条件 1（预算与收敛语义冲突）未触发；条件 2（超余量仍 limit_reached=模型能力面）未触发（唯一 limit_reached 是 Timeout 切片非轮预算耗尽）；确认卡驱动/治理写入链本轮未到达（G6/G8 前置拒绝），无 write_fail。

### §8 计费披露

- deepseek-flash **103 调用**，**1,512,840 input / 160,974 output tokens**（轮窗放大后单轮链内调用数上升，229/231 同腿量级约 30 调用；8 个 LLM 样本轮+choreo arm/settle+setup 无 LLM）。概率性口径：一次运行（slicebudget_1），组级结论按 220 口径（≥4/5+止损）判读，未做重复运行。
