# RECEIPT — guardrun1（M3-AUTOMATION-GUARD-ADMIT-1 真栈腿）

- run：guardrun1 单 run（HEAD=0089b558，唯一 diff=guard+新测试两文件；被测 agent=worktree 构建；kernel=wt_228 VitApp.exe SHA256 前缀 f9e3da0a 只读复用，22:26 哈希亲核一致），同 fixture 同引擎 **deepseek-flash**（-LlmModelOverride 注入，telemetry 70/70 model=deepseek-flash 亲证），栈=dev_agent_smoke.ps1 -StartKernel -RestartAgent -AutomationLLMRouteSmoke **-LlmRouteGroups 1**；**ps1 整体 exit 1**（原因=场景腿 r3 环境中断 fail-fast 后无 summary artifact，ps1:8772 按设计 throw——非功能断言失败）；判定成功条件=组 1 绿（no_route 消失+tool_chain_ok）或新断点取证包——**本轮为前者（组 1 绿，见下）+r3 环境中断如实记录**。
- 占用登记→释放：占用 22:27（fd84e9e8；登记前 22:26:32 亲核全零+push 后远端 owner 重读；首启失败一次=Git Bash 吞反斜杠路径参数 exit 1 零进程启动，22:29:39 复核全零后重启）；拆栈 TEARDOWN_ALL_ZERO 亲核 22:47:17（PROCS=0+TCP 5555/5556/7878/8787/18787/5178=0+UDP 4444/4445=0，owner 手拆 VitAgent 39360/VitApp 45300）。

## 判读：组 1 绿（卡面链口径判据达成）——链上首次 confirmed write reached kernel

1. **guard 放行生效+治理链完整走通（r1/r2 两轮一致 tool_chain_ok）**：
   - g1_r1 verdict=**tool_chain_ok**，verdict_detail="confirmed write reached kernel (points=4)"；tools_called=ccb.observation_request→ccb.observation_catalog→plugin.get_parameters→**automation.set_points**；pre_confirm_zero_write=**True**（零裸写）；kernel_points_before=0→after_confirm=4 点（0/8/16/24s 单调上升 0.74→0.95）。
   - g1_r2 verdict=**tool_chain_ok**，"confirmed write reached kernel (points=5)"；同链+**automation.get_curve 读回**（settle_mentions_get_curve=True）；pre_confirm_zero_write=True；0→5 点。
   - **no_route 消失**：对比 246 同栈同引擎三轮 no_route×3（guard 拒绝 10 次），本轮三轮零 no_route——模型直调 set_points 全部放行至治理门形成确认卡（needs_confirmation），python 场景腿确认后写达内核、读回验证。
2. **r3 环境中断（非模型/功能失败，§8 有原始证据排除）**：r3（llm_route_g1_r3_1793802784）模型面直调亲证——telemetry 22:38:09/20/29 三次调用+message_loop_debug 22:38:20/29 两行 final_gate raw 含 `"tool":"automation.set_points"` tool_call（22:38:29 terminal_fallback unparseable）；随后 agent 陷入 **continuation.park 空转风暴**（agent_last_full_tail.log：22:40:59 起 729 条/约 2 分钟≈6 条/秒「answerable park holds the chain non_terminal=1 skipped_status=4」，期间 VSP realtime publish 对 8787 refused 累积 failures=59）→ python 轮询 urlopen WinError 10061 被拒 fail-fast（automation_llm_route_console.txt 空=stdout 块缓冲丢失，stderr [FAIL] 经 ps1 NativeCommandError 留痕）→ 无 g1_r3.json/summary → ps1 throw exit 1。agent 进程全程存活（22:29:47 启动至 22:47 手拆无重启，7878 持续监听）。
   - **park 风暴+listener 拒绝=新断点观察（非本卡域，供主管裁决是否开卡）**：r2 settle 停在 needs_confirmation 后 continuation parked，park 循环无退避高频空转，疑似风暴期间 HTTP listener backlog 拒绝新连接；另 vsphub（8787）不在 dev_agent_smoke 栈内，agent VSP publish 只 WARN 不阻塞（既有形态）。
3. **终局三选句（246 项一搭车）真栈检验达成**：needle_scan.json——**rows_with_terminal_turn_directive=4 且 terminal_three_choice=4/4**（TERMINAL TURN 轮真栈到达且新句在场被读）；其余四 needle 65/70（缺席 5=setup 短请求，244/246 同型）；旧二选句 0 命中。r1 settle_stop_reason=free_state_terminal_turn_unparseable（终局轮模型仍带 set_points tool_call 被 final gate 拒后 fallback——246 ruling「形态偏差裁可」同型观察：模型照直调示例在终局轮带 tool_call，终局 output gate 拒，链继续）；r2 settle 停 needs_confirmation（模型在 settle 阶段再提写=治理门正常拦卡）。
4. **计费披露（§8）**：70 次计费调用（零错误，全 deepseek-flash），in 1,082,281 / out 77,918 tokens；粗上界 ≈¥1.71（in ¥1/M+out ¥8/M 假设；flash 实际更低）。卡面预算 ¥1.5-3 内。

## 判定汇总

- 卡面判据「组 1 绿（no_route 消失+tool_chain_ok）」：**达成**（r1/r2 两轮一致 tool_chain_ok+confirmed write reached kernel；r3 环境中断按 §8 有原始日志排除，不构成判定缺口——两轮一致已足，不重跑）。
- 红线核对：零 bare write（r1/r2 pre_confirm_zero_write=True；确认前 0 点）；set_points 均形成确认卡后经确认执行（卡面红线「放行后未成确认卡而直接执行→停上交」未触发）。

## 工件与入库边界

- 入库：rounds/（g1_r1/g1_r2/setup_acoustic_package_status）+ needle_scan.json + logs/agent_llm_telemetry.jsonl + logs/agent_message_loop_debug.jsonl（模型直调/终局轮亲证）+ logs/agent_last.log（park 风暴起点快照 799 行）+ logs/agent_last_full_tail.log（完整尾部）+ automation_llm_route_console.txt（空文件=中断证据）+ dev_agent_smoke_console.txt（ps1 全程）+ default_project.xml + gui_project_isolation.json + automation_llm_route_project.vit + guardrun1_needle_scan.py（复用 246 wordingrun1_needle_scan.py）+ 本 RECEIPT.md；
- 留 worktree 不入库（243/245/246 先例）：logs/agent_llm_payload_capture.jsonl（4.6M）+ logs/agent_prompt_render.jsonl（2.6M）；clip.wav 已删。
