# RECEIPT — wordingrun1（M3-TERMINAL-ROUTE-WORDING-1 真栈腿）

- run：wordingrun1 单 run（HEAD=40cdd189，唯一 diff=本卡四文件+新测试；被测 agent=worktree 构建 AFDE0E71；kernel=wt_228 VitApp.exe SHA256 前缀 F9E3DA0A 只读复用，21:48 启动前哈希亲核一致），同 fixture 同引擎 **deepseek-flash**（-LlmModelOverride 注入，telemetry 33/33 model=deepseek-flash 亲证），栈=dev_agent_smoke.ps1 -StartKernel -RestartAgent -AutomationLLMRouteSmoke **-LlmRouteGroups 1**，**ps1 整体 exit 0**+场景腿 status=COMPLETED；判定成功条件=组 1 绿（tool_chain_ok/no_route 消失）**或新断点取证包**——本轮为后者（见下）。
- 占用登记→释放：占用 21:49（登记前 21:47:23 亲核全零+push 后远端 owner 重读+启动前 21:49:41 ≤60s 即时复核全零）；拆栈 TEARDOWN_ALL_ZERO 亲核 22:04:51（PROCS=0+TCP 5555/5556/7878/8787/18787/5178=0+UDP 4444/4445=0，owner 手拆 VitAgent 44028/VitApp 26276）。

## 判读：组 1 未绿（verdicts=no_route×3），但断点形态=预期外新基础设施断点，v3 措辞模型面效果完全达成

1. **v3 措辞载荷亲证（needle 全绿）**：`agent_llm_payload_capture.jsonl` 33 条；v3 五 needle 全部在场——终局三选句锚前四项中三项+时变豁免句 28/33（缺席 5 条=setup/smoke 短请求，244 报告同型）；形态示例（`"tool":"automation.set_points"`+读回终态轮）28/33；确认桥接句 28/33；v3 标题 28/33；**旧二选句（"a bounded improvement proposal, or an explicit terminal statement"）0 命中**——被替换形态真栈消失。
2. **行为转变亲证（对比 245 carryrun1 同栈同引擎同组基线）**：245 基线三轮全 no_route 长链（33+36+19=88 LLM 调用，终局轮提案/blocked，**零 set_points 调用**）；本轮三轮全部快速直调（11+11+11=33 LLM 调用）——模型不再提案+软预问，**三轮一致走向 automation.set_points 直调**（`agent_last.log` 7 次 `message_loop.tool tool=automation.set_points confirmed=false`：行 226/260/289（r1）/347/354（r2）/408/420（r3））。**竞争引导消除（244 §2.2 面二）在模型行为面成立**。
3. **新断点（基础设施面，红线区，本卡不修——按停止条件上交取证包）**：三轮同型终结——模型直调 set_points → **chat 宿主 executor 包装层既有 guard 短路拒绝**（ms=0-13 无执行）→ 模型重试 1-2 次同拒 → run status=failed → 回执「前面的工具操作已完成，但最终回复生成失败：active free-state reasoning may use only read-only CCB observations; mutation tool automation.set_points is forbidden」→ smoke 判 no_route（plain-text response, no automation tool face）。
   - **根因锚点**：`agent/internal/chat/goalrunner_chat.go:855-863`（`pluginGrabberWorkflowExecutor.RunToolCall` 在**每个** agentloop 工具执行前先过 `freeStateInvokeMutationReason`）+ `agent/internal/chat/free_state_invoke_guard.go:26-29`（active free-state loop 期间 `tools.DefaultCatalog` 中 `MutatesProject` 工具全拒，automation.set_points 在列）。
   - **设计冲突定性**：agentloop 侧治理链设计（`free_state_automation_surface_test.go` `TestFreeStateSetPointsZeroBareWriteGovernanceChain`：决策轮直调→executor 见 Confirmed=false → needs_confirmation 卡→确认后写）与 chat 宿主 executor 包装层的 mutation 全拒**正面冲突**——guard 注释自述其意图是「closes the HTTP invoke boundary」（防公开 invoke 入口注入直写），但它在 executor 包装层的位置使 agentloop 内部合法治理链同样被封。235-245 全链从未有模型真正发出 set_points（245 基线零直调），故该 guard 从未显形；**本卡 v3 措辞第一次让模型走到直调，暴露此断点**。
   - **修复方向（供主管裁决，非本卡范围）**：guard 面对 automation 直调治理链放行（如 `freeStateInvokeMutationReason` 对 automation.set_points/clear_region 白名单放行走下游治理门，或 agentloop 治理链请求打标绕过 guard）——属准入/治理代码域，须新卡。
4. **终局三选句（项一）真栈未达检验**：`rows_with_terminal_turn_directive=0`——三轮均在直调被拒后 run failed 终止，**未走到 continuation/终局轮**（无 TERMINAL TURN 渲染轮），终局三选句仅有单测面覆盖（`TestTerminalTurnSentenceCarriesDirectCallShape` 渲染+bounce 同源亲证）+ 载荷设计面在场性由渲染路径保证。如实记录：项一的真栈轮检验将在 guard 断点修复后才能到达。
5. **计费披露（§8）**：33 次计费调用（零错误），in 474,640 / out 46,955 tokens；粗上界 ≈¥0.85（in ¥1/M+out ¥8/M 假设；deepseek-flash 实际单价更低）。卡面预算 ¥1.5-3 内。链短（33 vs 245 的 94）=直调路径少轮次所致。

## 附带观察

- r1 workflow_data 带 `admission_rejection_gaps`（G6_target_evidence/G8_target_consistency）+`admission_rejection_count:1`——模型在被 guard 拒绝前还经历过一次 needs_experiment 准入拒绝（提案路径被 G6/G8 拒），随即转直调：**豁免句（项四）引导的方向转变在轮内也可见**（提案被拒→不再重试提案→直调）。
- smoke 判 no_route 的口径注：三轮回执实为 guard 拒绝文本（stop_reason=failed），smoke 的 no_route 桶=「plain-text response, no automation tool face」——分类忠实（无工具面可见），但根因是基础设施拒绝而非模型未找到路径。

## 工件与入库边界

- 入库：automation_llm_route_summary.json / automation_llm_route_console.txt / rounds/（g1_r1/r2/r3+setup）/ logs/agent_llm_telemetry.jsonl / **logs/agent_last.log（245 裁定要求，本轮起入库）** / default_project.xml / gui_project_isolation.json / automation_llm_route_project.vit / wordingrun1_needle_scan.py / 本 RECEIPT.md；
- 留 worktree 不入库（243/245 先例）：logs/agent_llm_payload_capture.jsonl（2.1M）+ logs/agent_prompt_render.jsonl（1.2M）；clip.wav 已删。
