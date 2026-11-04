# M3-TERMINAL-ROUTE-WORDING-1：终局轮直调形态补全（面二措辞四项，含设计裁决落地）

- 发卡：GLM 主管决策流 / 2026-11-04（依据=244 勘察报告 §2.2/§2.3/§2.4+裁定 [2026-11-04-M3-BREAKPOINT-FORENSICS-1-pass.md](../rulings/2026-11-04-M3-BREAKPOINT-FORENSICS-1-pass.md) 设计裁决）
- 派发确认：已确认（链内续卡，主管发卡；用户口头开工即有效）
- 验收负责人：GLM 主管决策流
- 池序 246；目标仓库=主仓 agent；P1 / 0.5 天 / 依赖=245 已合入（终局轮披露在场是本卡判据的前置）✓ 待 245 验收后领取；L1 / PC 执行会话
- **并行注记（§2.2 ③）**：与 245（M3-CHAT-HOST-CARRY-FIX-1）文件域不相交；真栈腿**错峰**——245 先跑，见其释放登记后再登记启动本卡栈腿。

## 设计前提（主管已裁，用户可推翻）

时变目标**不入 admitted 提案词表**（2026-10-31 vocabulary-full-retirement+automation 工具链设立初衷）；本卡全部为措辞/示例/规则句修改，不新增提案域、不动准入/治理/执行代码。

## 目标（四项措辞，全部句级 add/replace）

1. **终局指令补第三形态**：`freeStateTerminalTurnSentence`（`agent/internal/agentloop/free_state_reasoning.go:792`）+渲染（`ccb_model_prompt.go:420-444`）——终局二选「a bounded improvement proposal, or an explicit terminal statement of where you are blocked」补第三形态：**全写要素在手时直接发起 mutation 调用（automation.set_points），运行时治理门自动拦成确认卡**；与 v2 触发权句同语义同源。
2. **时变形态示例**：`ccb_model_prompt.go:130` v2 Action Path 段邻接示例块——补一个 `final:true`+`automation.set_points` tool_call（plugin_id/param_id/points 合法形态）的时变示例，消除「唯一 final 示例=静态提案」的邻接缺陷。
3. **确认语义桥接句**：v2 句邻接补一句——时变目标走直调后由运行时门拦卡，确认卡内自然可核对时间/数值，**无需文本预问**（桥接提案世界确认门与直调世界不预问的双语义）。
4. **目标级规则句时变豁免**：载荷节区「Once the target-level evidence … return final=true needs_experiment …」（报告 §2.2 #3，落点由执行侧按载荷锚点定位：ccb 组装或 ruleset section）加时变豁免子句（time-varying goals 走 automation 直调，不走该句）。

## 同源锁链纪律（243 先例）

涉 ruleset sections（neutral_family.discipline/chat.discipline.plugins_mixing）与 `agent/internal/chat/server.go` fallback 的同源行按 parity 测试强制同步；`automation_llm_route_smoke.py` needle 同步（新句在场+被顶掉的旧二选句计数）；版本号纪律（涉及段落 CacheKey/version bump）。

## 判据与验收标准

- **单测**：新句锚+被替换旧句负断言（四面同步面）+needle 同步；缺省组行为不变。
- **真栈腿**（-Groups 1 单组一次，deepseek-flash）：判据同链口径——**组 1 绿（no_route 消失+tool_chain_ok set_points 包络形态）**或新断点取证包；载荷亲证（新句/新示例在场+被读：response 引用或行为转变）；栈登记/释放全流程。
- §8 预算 ≈¥1.5-3；计费披露；端测边界声明（无 webui 渲染面改动则声明不适用）。

## 停止条件

- 措辞破坏既有测试（零回退红）→回退两案上交；
- 组 1 仍不绿→按链口径出新断点取证包上交，不原样重跑不自修；
- 目标级规则句落点涉及非预期文件域→停上交。

## 文件域

`agent/internal/agentloop/ccb_model_prompt.go`+`agent/internal/agentloop/free_state_reasoning.go`+`agent/internal/agentloop/neutral_automation_route.go`（如涉）+ruleset sections 同源文件+`agent/internal/chat/server.go` fallback+`scripts/automation_llm_route_smoke.py`（needle）+测试。越域即停上交。

## 领取

- 2026-11-04 晚 / origin/main=f899cfac（=245 验收合入后 HEAD）/ owner=ZCode GLM-5.3 PC 执行会话（246 卡会话，用户口头开工）/ 实现 worktree `D:\Vit_DAW_wt_246impl` @ 分支 `port/m3-terminal-route-wording-1`（基线 f899cfac）；协调 worktree `D:\Vit_DAW_wt_246claim` @ `coord/246-claim`（仅本卡状态提交）。
- 领取基线：主检出 HEAD=f899cfac，既有工作树改动均为运行时工程状态（Settings.xml/default_project.xml）与历史 run 工件/旧卡状态（UX-AB-AUDITION 等），与本卡文件域不相交；`git diff --stat` 摘要 8 files +4469/-4429（全为领取前已有，非本卡 diff）。
- 栈：空闲（245 已释放 e089cbfe，2026-11-04 21:09 TEARDOWN_ALL_ZERO+错峰解锁注记「246 自该条起可登记」）；本卡真栈腿登记见回执。

## 回执

- **执行侧自验完成，转 done 待主管验收**。实现 commit=40cdd189（分支 `port/m3-terminal-route-wording-1`，基线 8e4722aa；diff 恰好四文件改动+一测试新增=卡面文件域零越域：ccb_model_prompt.go +24-5/free_state_reasoning.go +8-2/ccb_model_prompt_action_path_test.go +6-1/automation_llm_route_smoke.py +27/ccb_model_prompt_terminal_route_test.go 新增 147 行）；工件 commit=67f923a2（coord/runs/M3-TERMINAL-ROUTE-WORDING-1/wordingrun1/ + RECEIPT.md 判读 + **agent_last.log 入库=245 裁定要求**；payload_capture 2.1M+prompt_render 1.2M 留 worktree，clip.wav 已删）。
- **四项措辞全落地 ✓**：①终局句三选（`freeStateTerminalTurnSentence` 补第三形态=全写要素在手→直接 automation.set_points+治理门自动拦卡，与 v2 触发权句同语义；后缀锚保持，既有冻结句测试全绿）②时变形态示例（v3 段补直调轮 final:false+set_points tool_call+读回终态轮 final:true 两行示例）③确认语义桥接句（确认卡=时变写入核对授权面，文本预问不成卡）④目标级规则句时变豁免子句（244 §2.2 #3 对撞消除，与原句同行邻接）。版本纪律：`freeStateTimeVaryingActionPathVersion` v2→v3+243 锚集扩展+smoke needle 三新（终局直调/时变豁免/旧二选句计数必须为 0）。
- **⚠️ 与卡面字面的一处形态偏差（如实上交，主管裁定）**：卡面项二原文「final:true+automation.set_points tool_call 示例」——代码证据证明该同轮组合在运行时不可执行：`normalizeMessageLoopOutput`（message_loop.go:4860-4863）对 final:true **清空 tool_calls**（静默丢写），终局轮 output gate（free_state_reasoning.go `messageLoopFreeStateTerminalTurnIssue`）更拒一切 tool_calls。照字面写示例会把模型教向「自以为写了实际没写」的静默丢写形态。故示例按运行时合法治理链形态写（直调轮 final:false+tool_calls→治理门拦卡→确认→写→终态轮 final:true 读回，与 `free_state_automation_surface_test.go` 既有治理链测试同形态），卡面意图（消除「唯一 final 示例=静态提案」邻接缺陷）完整达成。
- **单测 ✓**：新增四件（`ccb_model_prompt_terminal_route_test.go`）——终局三选新句锚+旧二选句负断言+bounce 同源；v3 示例/桥接句锚+final:true+tool_calls 保守负断言；豁免句锚+旧直断句负断言+同行邻接；directives 层落位+缺省组行为不变。243 锚集更新（v3 锚入集）。同步面=ruleset sections/chat server.go fallback 本卡四项未触碰（v2 触发权句原文保留），既有 parity 测试（TestNeutralFamilyAutomationRouteMatchesChatRulesetLine 等）跑绿=四面同步证明。全仓 `go test ./...` exit 0 零红+py 语法过。
- **真栈腿（wordingrun1，-Groups 1 单组单 run，deepseek-flash 33/33 注入亲证）**：ps1 exit 0+场景腿 COMPLETED；判定=**组 1 未绿（verdicts=no_route×3）→按停止条件出新断点取证包上交，不重跑不自修**：
  1. **v3 措辞模型面效果完全达成**：载荷五 needle 28/33 在场（缺席 5=setup 短请求，244 同型）；旧二选句 0 命中=被替换形态真栈消失；**行为转变亲证**——245 同栈同引擎基线三轮零直调（长链提案/blocked 88 调用），本轮三轮一致快速直调（33 调用，agent log 7 次 `automation.set_points confirmed=false` 行 226/260/289/347/354/408/420）；
  2. **新断点（红线区，须新卡）**：模型直调被 **chat 宿主 executor 包装层既有 guard 短路**（`goalrunner_chat.go:855-863` 每工具先过 `freeStateInvokeMutationReason`+`free_state_invoke_guard.go:26-29` active free-state 期间 MutatesProject 工具全拒）→ms=0-13 无执行→重试同拒→run failed。与 agentloop 治理链设计（决策轮直调→needs_confirmation 卡，`TestFreeStateSetPointsZeroBareWriteGovernanceChain` 先例）**正面冲突**；guard 注释自述意图=防 HTTP invoke 公开入口直写，但 executor 包装层位置使 agentloop 内部合法治理链同被封。235-245 零直调从未显形，v3 首次走到直调暴露。修复方向（供裁决）：guard 对 automation 直调面白名单放行走下游治理门——属准入/治理域非本卡范围；
  3. **终局三选句（项一）真栈未达检验**：三轮均直调被拒即 failed，无 continuation/终局轮（TERMINAL TURN 渲染轮 0），该项真栈轮检验待 guard 断点修复后到达；单测面（渲染+bounce 同源）已覆盖；
  4. 附带观察：r1 轮内提案 G6/G8 被拒后转直调（豁免句方向转变轮内可见）；smoke no_route 桶=分类忠实（无工具面可见），根因是基础设施拒绝非模型未找到路径。
- **计费（§8）**：33 次计费调用（零错误）in 474,640/out 46,955，粗上界 ≈¥0.85（in ¥1/M+out ¥8/M 假设，flash 实际更低）；卡面 ¥1.5-3 预算内（链短=直调路径少轮次）。
- **栈纪律（§2.2）**：占用 21:49（登记前 21:47:23 亲核全零+push 后远端 owner 重读+启动前 21:49:41 ≤60s 即时复核全零）→释放 22:04:51 TEARDOWN_ALL_ZERO 亲核（PROCS=0+TCP 六口=0+UDP=0，owner 手拆 VitAgent 44028/VitApp 26276）。
- 端测覆盖边界声明：本卡改动面=agentloop 措辞三文件+测试+smoke needle；无 webui/渲染面改动（不适用）；真栈腿覆盖 agent HTTP/ZMQ 面+载荷捕获面+agent log 面。

## 验收

- （主管回填）
