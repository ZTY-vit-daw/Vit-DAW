# RECEIPT — carryrun1（M3-CHAT-HOST-CARRY-FIX-1 真栈腿）

- run：carryrun1 单 run（HEAD=fae86236，唯一 diff=本卡三面：`agent/internal/chat/free_state_reasoning_loop.go`+`goalrunner_chat.go`+`free_state_chat_host_carry_test.go`），同 fixture 同引擎 **deepseek-flash**（-LlmModelOverride 参数注入，先例=238/240/241/242/243），kernel=wt_228 VitApp.exe SHA256 前缀 **F9E3DA0A**（235-243 同款只读复用，启动前哈希亲核一致），栈=dev_agent_smoke.ps1 -StartKernel -RestartAgent -AutomationLLMRouteSmoke **-LlmRouteGroups 1**，**ps1 整体 exit 0**+场景腿 status=**COMPLETED**（driver console「automation LLM route group filter: -Groups 1」组过滤单 run 亲证）。
- 载荷坐标口径：`logs/agent_llm_payload_capture.jsonl` 共 94 条记录；r1=记录 1–33（33 条）、r2=34–69（36 条）、r3=70–88（19 条），其余为 setup/smoke。每轮终局轮=轮内最后一条大请求（source=message_loop，~60KB，TERMINAL TURN 指令在场）；记录 33（1726B，source=message_loop_repair）为 r1 末尾的修复类短请求，非终局 LLM 轮。

## 判据核验（三判据全达成）

1. **needle=终局轮请求载荷含 1011/0.740818 ✓**：
   - r1 终局轮=记录 32（61770B，TERMINAL TURN 在场）：`automation_surfaces` ledger 行完整在场——plugin_id "1011"、param volume **value=0.740818202495575**（value_text "+0.00 dB"）+pan value=0、min/max 范围、project_revision=5、round=8、track_id=1010；
   - r2 终局轮=记录 69（60170B，TERMINAL TURN 在场）：同完整行（round=18）；
   - 对照 writegate1（244 报告 §1.1）：r1/r3 终局轮三节全无披露→本轮 r1/r2 终局轮 ledger 行在场=**chat 宿主 run 边界 typed round-trip 丢弃修复的真栈实证**；
   - r3 全轮 0/19 无披露：r3 样本全程**零 get_plugin_parameters 调用**（模型行为面，零调用→零钉住行→ledger 无行是正确语义；模型行为只记录不设门）。
2. **「继续」回执携带状态 ✓**：三轮 ex1（stop_reason=automatic_continuation_already_scheduled）WorkflowData 均带 `free_state_summary`（r1 样例：decision_status=needs_observation+decision_summary 人话摘要（时间型需求+缺时间轴定位）+latest_observation（tool/status）+continuation_used/budget=1/6+cycle+observation_receipt_count=6+terminal_turn_locked=true）。
3. **终局摘要投递可见 ✓**：agent log `[chain.terminal-summary] delivered` ×2——r1（20:53:28，conversation=llm_route_g1_r1_1793796475 goal=goal_812317554464b589 decision=capability_blocked status=completed）+r3（21:02:28，goal=goal_8176a764e0605fbb decision=capability_blocked）；r2 经 chain-end gate 通道投递（[f6.gate] chainEnded=true status=cancelled respGoal=completed runtimeGoal=completed→gate 第一分支 durable terminal+goal completed→deliverSchedulerChainTerminal）。

## 附带观察（如实上交，不扩大修复面）

- **r2 终局 run 的 recordGoalResult 缺席**：20:58:56 r2 终局 run 完成（agent_loop_chat total_ms=11678 status=completed stop=capability_blocked）但**无 [continuation.arm] 行**（r1/r3 同点均有）——recordGoalResult 未执行的证据；该 durable 记录终态=cancelled（f6.gate 行）而 goal=completed。候选机制=capability_blocked 边界路由（capabilityBlockedBoundaryResponse/improvementProposalResponse 路径均调 chatResponseFromAgentLoopResult，理论必经）与实际日志矛盾，**未定位**；属既有路由面缺口非本卡修复引入（本卡改动前该路径同样存在）。r2 终局结果经 gate 通道投递未丢失。建议主管裁定是否转后续卡（needs_experiment 提案终局路径的 durable 记账面）。
- 组 1 verdicts=[no_route, no_route, no_route]（三样本同 243 writegate1 形态；模型行为面不设门）。

## 计费与预算披露（§8）

- 94 次计费调用（零错误），in 1,445,971 tokens / out 135,898 tokens；按 in ¥1/M+out ¥8/M 假设粗上界 **≈¥2.53**（deepseek-flash 实际单价更低+cache_read 折扣，真实成本低于粗上界）。超卡面 ≈¥1.5-2 预算线的因素=r2 切片长链（36 调用/308 秒）+三样本全跑；单组单 run 无重跑，止损未触发（no_route×3 为组内判定自然收尾非 env 中断）。

## 工件与入库边界

- 入库：automation_llm_route_summary.json / automation_llm_route_console.txt / rounds/（g1_r1/r2/r3+setup）/ logs/agent_llm_telemetry.jsonl / default_project.xml / gui_project_isolation.json / automation_llm_route_project.vit / 本 RECEIPT.md；
- 留 worktree 不入库（243 先例）：logs/agent_llm_payload_capture.jsonl（6.2M）+ logs/agent_prompt_render.jsonl（3.4M）；clip.wav 已删。
- 拆栈：21:08:59 TEARDOWN_ALL_ZERO 亲核（PROCS=0+TCP 5555/5556/7878/8787/18787/5178=0+UDP 4444/4445=0；owner 手拆 VitAgent 41076/VitApp 27984）。
