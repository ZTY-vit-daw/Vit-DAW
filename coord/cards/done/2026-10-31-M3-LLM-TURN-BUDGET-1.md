# M3-LLM-TURN-BUDGET-1：闸 2 深层——closure 相位 slice 轮次预算与 automation 调度（227 重验腿终验承接，终态架构阶段②）

- 发卡：GLM 主管决策流 / 2026-10-31（依据=227 重验腿定案：闸 1/观察面/指引三前提就位后组 1 仍三连 no_route，根因=chat/audio_closure_controller.go:438-447 closure 相位 slice **MaxTurns 1-2**+异步续跑泄流+观察边界 settle——222「MaxTurns 8」描述修正；222/227 两轮真栈 LLM 计费取证在卷）
- 派发确认：待确认（227 验收转出+终态架构路线图既定——用户 2026-10-30「可以开卡」覆盖阶段②；发卡即提交，领取按池序）
- 验收负责人：GLM 主管决策流
- 池序 229；目标仓库=**主仓 agent chat 域**；P1 / 0.5-1 天 / 依赖=227 已合入（a302d387）✓；L2 / PC 执行会话（FS 状态机调度语义+LLM 判读）

## 标的（227 定案，不重考）

closure 相位对变体话术的决策轮只给 1-2 turn 预算：模型观察一探（读轨/插件）即触预算线→waiting_continue 截停→异步续跑泄流到观察边界 settle（无 LLM 轮）——automation 调用（需先 get_plugin_parameters 读值域再 set_points）在 1-2 turn 内结构上排不完。

## 目标

1. **预算语义改造**：变体话术决策轮的 turn 预算从 closure 相位常数（1-2）改为**意图感知分配**——勘察定方案：automation 诉求识别后该轮预算扩至可完成「读值域→治理门→写→读回」最小链（预算来源/上限/衰减防滥用三面成文）；既有观察 slice 语义（非 automation 诉求）零回退。
2. **异步续跑泄流收口**：waiting_continue 截停后的异步续跑不得绕过治理语义直达观察边界 settle——续跑路径与决策轮同工具面同预算语义（227 上交锚点 438-447 邻域）。
3. **settle 面补轮判定**：闸 3 终判——决策轮预算改造后若「调工具→治理门→执行→读回→汇报」全链可在轮内闭环，settle 补轮不开（终态架构最简形态）；否则最小补轮方案上交。
4. **重验腿（本卡验收门）**：220/222/227 同断言体 v2（automation_llm_route_smoke.py 原口径不得改），组 1「这段让它越来越突出一点」先行——**组 1 no_route 消失+set_points 确认链（确认卡→确认→写入→get_curve 读回）首次端到端全绿**=终态架构阶段②达成；组 2/3 按 220 口径跑满。

## 安全面（硬约束，承 227）

- mutation 恒经治理门零裸写；每轮至多一 mutation 限速语义不变；
- FS 状态机 stop-pending/隔离/权限闩零回退；预算扩面不得成为无治理连写通道（上限+衰减）；
- 完全权限直执承 223 stamp；manual 确认卡形态不变。

## 文件域

`agent/internal/chat/audio_closure_controller.go`（预算+续跑）+FS 调度邻域（勘察定）+测试+scripts（断言体复用）。越域即停上交。

## 停止条件

- 预算改造与 closure 收敛语义冲突不可调和（FS 体系设计约束）→锚点+方案上交；
- 重验组 1 四前提齐（指引+工具+观察+预算）仍三连 no_route→属模型能力面，锚点上交转模型/指引设计讨论；
- LLM 面栈不可用→环境中断如实记录。

## 领取

- 时间：2026-10-31 10:27；origin/main=33568012（fetch 亲核=远端一致）
- owner：ZCode GLM-5.3 PC 执行会话（229 卡会话，用户转交提示词指定领取）
- 分支：port/m3-llm-turn-budget-1；worktree：D:/Vit_DAW-229（基于 origin/main 33568012 新建）
- 领取基线：`git rev-parse HEAD`=33568012affd812383f15854fd9a406f5e694458；`git status --short`=空；`git diff --stat`=空（工作树干净，无领取前 diff）
- 前提核对：227 已合入（a302d387 在 origin/main 线）✓；doing 仅 GEN-SUNO-WEB-DESIGN-1（零生产码，域不相交）✓；226（M3-AUTH-RESTORE-CARRY-1）领取时仍在 todo 未领取，无并行域冲突；并行警示已知悉——勘察若触及 server.go / working_session.go 即停上交错峰裁定
- 真栈错峰：226 与本卡并行，真栈腿先到先登记（PC-RUNTIME-STACK.md fetch 重读置顶+进程/端口全零亲核+登记先于启动），未登记者长轮询等待（220 先例）
- §8 口径：LLM 轮真实计费，回执披露调用数与 token；断言体 v2（automation_llm_route_smoke.py）原口径不改

## 回执（2026-10-31，卡留 doing 等主管验收）

### 链路进度（§7 分列一）

- **三面实现全落地（commit 见下，96 包全量回归零失败）**：
  - 面 1 预算：`audioClosureMessageLoopBudget` 新增识别面 `audioClosureAutomationRoundContext`——与 227 `admitFreeStateDecisionRoundToolSurface` **同结构判定**（active FS loop+非诊断专用，loop map 双键诊断排除），命中即保留 mode 基础预算（goal 10 turns/16 calls）不落 1-2 钳制。三面成文：**来源**=工具面同源（结构识别非话术判断，判断权归属原则）；**上限**=mode 基础原样不叠加（settle +4 与 227 执行时 +4 各自独立触发，MaxClosureRounds 契约不放松——轮计数按请求边界 admit，扩面只减片数不增轮耗，**与 closure 收敛语义无冲突，停止条件 1 排除**）；**衰减**=识别面窄+治理门/每轮至多一 mutation 限速（227）不因扩面放松。诊断专用轮与非 FS closure 轮观察 slice 语义**零回退**（既有测试全绿）。
  - 面 2 续跑收口：续跑片（scheduler durable/用户继续）经 `loopBudget(cont.Context)` 同函数同判定（prepareFreeStateReasoningContext 写回 loop map 亲核闭环）——同预算语义+同工具面（agentloop Continue 入口 admit 幂等），无绕行路径，单测钉死。
  - 面 3 settle：**补轮不开**（227 定案维持）——轮内闭环链长核过（catalog→request→读值域→写=治理门暂停→ResumeAfterConfirmation 新片→get_curve→汇报，8-10 turns+3min 足够；治理门暂停=waiting_interaction 交互边界非自动续跑）。
- **预算面真栈验证生效**（run=coord/runs/M3-LLM-TURN-BUDGET-1/llmbudget_20261031_1，ps1 exit 0）：227 每轮 1-4 调用即 limit_reached 截停 → 本轮 **LLM 34 调用**（r1=8/r2=12/r3=14），模型轮内完整走 catalog→视图请求→**轨道选择（Track 2/1010）→轨道级观察**；limit_reached 泄流形态消失；工具面 render 17,045B 恒定（227 骨架零回退）。

### 最终验收（§7 分列二）

- **组 1 no_route 消失：未达**（三连 no_route 止损，脚本按设计停止，组 2/3 未消耗）；**set_points 确认链端到端首绿：未达**（模型未到执行相）。ps1 exit 0=验证卡完成语义，非验收门通过。
- **新根因（观察证据层，越域上交）**：acoustic package status=**missing**（.vit_derived 亲证，摘录 run evidence/）——裸轨工程（无音频 clip，220 断言体 setup 形态）→派生观察（频段动态/电平 levels/多轨关系）源证据缺失→FS 准入门 G3/G5 不可满足→模型 **capability_blocked 诚实停边**（不瞎写不绕道=终态架构期望行为；脚本 boundary 词表未匹配申报文案故记 no_route）。r1「工程标识不一致」为模型对同根因的归纳表述。**结构矛盾：写 volume 渐变只需工程结构面证据（轨+插件在场），但准入门要求的观察证据在裸轨工程上必然缺失。**
- **上交三选项（主管裁定）**：A. scripts 工程补音频 clip（220 断言体 setup 域）；B. FS 准入门对无音频工程适配（agentloop G 门语义，结构面证据足以准入最小可回退写试验）；C. 220 词表口径（capability_blocked 申报的判定归类）。均越本卡文件域（chat 预算+续跑），§11 越域即停上交。**非停止条件 3**（模型能力面）——模型行为正确，是观察证据结构性缺失；也非停止条件 1（预算与收敛语义无冲突，已排除）。

### 工件与命令

- 实现 commit：port/m3-llm-turn-budget-1（`audioClosureMessageLoopBudget`+识别面+`audio_closure_turn_budget_test.go` 六面）；工件 commit：同分支 coord/runs/M3-LLM-TURN-BUDGET-1/llmbudget_20261031_1（RESULTS.md+rounds+logs+evidence 摘录；派生态剔除按 218/222 先例）。
- 测试：`go build ./...` exit 0；`go test ./internal/chat ./internal/agentloop -count=1` ok；`go test ./... -count=1` **96 包 ok 零失败**；真栈 `dev_agent_smoke.ps1 -StartKernel -RestartAgent -AutomationLLMRouteSmoke -NoChatSmoke -NoStripSilenceSmoke -KernelExe <228 F9E3DA0A>` **exit 0**。
- 栈：占用 0008b5f1（10:59 登记先于启动）/释放见 PC-RUNTIME-STACK.md（拆栈复核进程/端口全零）。

### 计费披露（§8）

- LLM 34 调用真实计费（deepseek-flash；r1=8/r2=12/r3=14；逐调用 logs/agent_llm_telemetry.jsonl）。**限制**：本遥测面 usage 字段空，token 明细以 provider 账单为准（227 曾得 124,621in/11,639out 量级参照，本轮调用数为 227 的 3.1 倍）。失败分类：no_route×3（同型新形态）；零 env_interrupt/confirm_broken/write_fail/lexicon_hit。一次到位无迭代轮，§8 纪律下不原样重跑（根因=结构性观察证据缺失，非偶发）。

## 验收（GLM 主管决策流 / 2026-10-31）

- **实现面 pass 合入 8954b8e1**：识别面同构性主管亲核（audioClosureAutomationRoundContext 与 227 admitFreeStateDecisionRoundToolSurface 同为「active FS loop+非诊断专用」结构信号，非话术判断——判断权归属原则维持）；测试五面+agentloop 包+build 主管分支复跑全绿；预算生效真栈工件亲证（34 调用/零 limit_reached 终局/轨道选择 1010+轨道级观察到达/render 17,045B 恒定）。
- **验收门未达=转出 230 承接**：新根因证据主管亲证（evidence/acoustic_package_status.json status=missing+l1_static 全 missing 亲读；r3 latest_decision capability_blocked 申报原文在卷——「无法确认具体候选目标…要继续需要先恢复观察数据」）。三选项裁定：**A 采**（fixture 补确定性音频 clip，测试环境对齐真实工程——模型在裸轨上的诚实停边是终态架构期望行为，验收门需要真实 fixture）+**C 采**（capability_blocked 诚实申报与 no_route 词表漏配分型，判定面归类）+**B 否**（FS 准入门 G3/G5 语义不为测试便利弱化——证据准入纪律是观察投影+治理链的根基；空轨可写若为产品需求属产品级裁定另立，非测试修复面）。229 验收门（组 1 no_route 消失+set_points 端到端首绿+组 2/3 跑满）原口径由 230 承接。
- 回执更正：计费归因 r1=8/r2=12/r3=14 → telemetry 会话计数 8/11/15（总数 34 一致，轮次归因小误差，非实质）。
