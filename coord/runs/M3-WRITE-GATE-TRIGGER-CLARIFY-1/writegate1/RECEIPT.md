# RECEIPT.md — M3-WRITE-GATE-TRIGGER-CLARIFY-1（243）真栈单轮工件判读（writegate1）

- run：writegate1 单 run（HEAD=b6cbc007，唯一 diff=本卡措辞五面+组过滤），同 fixture 同引擎 **deepseek-flash**（-LlmModelOverride env 注入），kernel=wt_228 VitApp.exe SHA256 前缀 **F9E3DA0A**（235-242 同款只读复用，本次哈希重核一致），栈=dev_agent_smoke.ps1 -StartKernel -RestartAgent -AutomationLLMRouteSmoke **-LlmRouteGroups 1**，ps1 exit 0（dev agent smoke completed）+场景腿 status=**COMPLETED**（groups_requested=[1] 组过滤单 run 亲证：driver console「automation LLM route group filter: -Groups 1」）。
- 引擎三面亲证：driver_console env 行 VIT_AGENT_LLM_MODEL=deepseek-flash；telemetry 162/162 行 model=deepseek-flash；payload_capture 168 行（含 6 行 setup/strip 腿）model=deepseek-flash。

## 组 1 判据：门未过（0/3 成功，三连 no_route 同型止损）——旧断点形态消失+新真断点成立（接线问题取证包）

| 样本 | verdict | 形态 |
|---|---|---|
| r1 | no_route | 42 轮长链：40 轮 needs_observation 观察请求循环（携带 plugin.get_parameters×4+automation.get_curve×3 并行），终局轮 final=true capability_blocked（申报 missing_resolved_parameter_identity_and_time_span）；之后 durable continuation 挂起（reply「我还在继续处理这个任务，完成后再向你汇报」） |
| r2 | no_route | 零工具；前台首答即「任务已在自动续跑队列中；这条消息不会创建新任务」automatic_continuation_already_scheduled（r1 goal 未终止→轮间污染）；84 行 payload=后台 continuation 续跑 r1 goal 的调用 |
| r3 | no_route | 同 r2 形态（34 行后台续跑调用） |

- **242 断点形态消失**：三轮无任何「弹出确认卡/提交后在确认卡里等你确认/你确认后即可」纯文本征询话术（242 为五样本统一形态）；三轮均未发起 set_points 调用——不是被治理门拦（门拦会形成确认卡），是从未到达调用面。
- **新指引句载荷亲证在场+被读**：payload_capture 162 计费行中 157 行携带 v2 触发权句（"never forms a confirmation card"，96% 覆盖，5 行差=setup/非中性装配轮）+157 行携带标题 needle；**旧句零残留**（"before any write" 0/162）；prompt_guidance_rendered=True。被读行为转变证据：r1 第 4 轮 reply「把这条轨道音量控制的参数范围读出来，**好把渐强包络的点落在有效区间内**」+tool_calls reason「**写入前确认**该轨音量参数的**可写范围与标识**」——模型按新指引直接以写曲线为目标收集要素（observe→get_parameters→规划包络→写入），不再停在文本征询。
- **新真断点双形态（接线问题取证，上交不自修）**：
  1. **观察回执不满足循环**：r1 中途 summary 显示模型已消费观察回执（「已确认轨道整体峰值约 -4.2 dBFS、可用余量约 4.2 dB，两个活动段覆盖 0-6 秒」）却持续申报「还缺」（r4 缺活跃结构+参数范围；r5 缺峰值细读+参数范围——尽管 4/5 轮都带 get_parameters 调用；r6 缺起止时间）；40/42 轮 needs_observation 后终局轮申报「缺可写标识+时间跨度」**与中途持有 0-6s 区间矛盾**。归因候选：get_parameters/activity_structure 回执面在续轮间的可见性（终局轮看不到此前回执？）或回执内容不满足模型自设标准——需勘察 pending candidate 形成路径与观察回执跨轮投影。
  2. **durable continuation 轮间污染**：r1 goal 未终止挂起→r2/r3 前台对话被 automatic_continuation_already_scheduled 吞（零 LLM 决策轮）+后台 84/34 调用空转→三连 no_route 同型止损。smoke 逐轮新对话的设计与 durable continuation 的 goal 存活窗口交互是断点 2 的直接机制。
- **与卡面停止条件 3 的关系**：非「仍停确认面」（该形态已消失）但同属「组 1 判据未达+新断点取证」路径——按停止条件精神收口：不原样重跑、不自修 pending candidate 形成路径，取证包（本 RECEIPT+summary+payload）上呈主管。
- §8 计费：162 调用 in 2,555,882 / out 252,212 ≈ **¥3.1 上界**（242 同口径换算 ¥1.103/百万 token）——超卡面 ¥3 上界如实披露：r1 单轮 43 调用+continuation 后台空转 118 调用为超预算主因；三连止损自动止损封住后续花费，无重跑。g2 观察腿不跑（卡面条件「门过且预算余量」双不满足）。

## 工件清单

- 入库（随 port/m3-write-gate-trigger-clarify-1）：driver_console.txt（含 PS1_EXIT=0）/automation_llm_route_console.txt/automation_llm_route_summary.json（1.1M 含 exchanges 全文）/automation_llm_route_project.vit/default_project.xml/gui_project_isolation.json/head.txt（b6cbc007）/ps1_exit.txt（0）/logs/agent_llm_telemetry.jsonl（162 行）/logs/agent_llm_payload_capture.jsonl（168 行 12M）/rounds（g1_r1-r3+setup_status）/本 RECEIPT.md。
- 不入库留 worktree：logs/agent_prompt_render.jsonl（6.2M，与 payload 重复度高，242 先例）。
- 已删除（240/242 教训）：automation_llm_route_clip.wav+.vit_agent/.vit_derived/.vit_history 运行态目录。

## 拆栈与登记

- 占用登记 0930b61e（登记前 fetch 重读 main=ad44370e+亲核全零 18:24；push 后 18:28:01 ≤60s 即时复核 PROCS=0+TCP 全零）；拆栈 TEARDOWN_ALL_ZERO 亲核（**19:04:13** PROCS=0+TCP 5555/5556/7878/8787/18787/5178=0+UDP 4444/4445=0，ps1 不自拆由 owner 手拆）；释放登记随本批推 main。
