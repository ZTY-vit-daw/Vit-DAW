# FS-NAMED-TARGET-1 真栈 chat 腿判定件（run fs_named_chat_20261021_2035）

- 执行：GLM-5.3 L2 PC ZCode 执行会话（157 卡）；2026-10-21 20:29 登记（e47e09e8）→ 20:35 起栈 → 20:33 拆栈复核（五端口 tcp+udp=0、四类进程=0）
- 被测：agent=worktree D:/Vit_DAW_wt_named1 构建 `bin/VitAgent.exe`（SHA256 前 16=8ca2c8aa01e909d1，HEAD f6e69def 含本卡改动）；内核=K3 只读复用 `D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe` SHA256=722A79D7A3F53797F5713209D812D8CE9F2888CB2A7E743286FA56B99629DFEC（20:29 亲核一致，本卡零内核改动）
- 驱动：`softguide_exp.py`（146 分支原件，随 run 目录留档）`--arm B1 --variants A1,A3`，话术=146 逐字（prompt.txt 在卷）；栈=手动起栈（146 先例形态：内核 5148 + agent 14848，VIT_PROJECT_XML 指 run 目录工程副本，fixture/stems 与内核工作区全在 run 目录内）
- 驱动 exit 0；两轮均捕获（settle=goal_terminal；transport_errors=0；card_hops=0；零拓扑写入 sends 0->0 / fx 0->0）

## 判定面（卡面验收：146 A1/A3 复刻话术不再弹回、到达提案面）

| # | 事实 | 证据 |
|---|---|---|
| 1 | **F-gate 弹回签名零出现**（146 B1 同话术 A1=4 次+A3=4 次） | `logs/agent_last.log`：`free_state_final_gate` 0 次、`candidate track set` 0 次 |
| 2 | **名字解析真栈命中**：A1「人声轨」→ 1032（vocals）、A3「鼓组」→ 1012（drums） | `arm/*/chat_1.json` workflow_data.free_state_reasoning_loop.user_named_target；持久面 `.vit_history/.../agent_runtime_state.json` 同值在卷 |
| 3 | **user_intent 合成 candidate 入 frontier 并持久** | chat_1 frontier.candidates 含 `{"id":"user_named:1032","source":"user_intent","track_ids":["1032"]}`（A3 同款 1012）；终态 closure 状态在卷（fs9_terminal） |
| 4 | **指名轨 track.* 观察被守卫放行并执行** | agent log：ccb.observation_request 执行 4 次全部 status=ok（A1 轮 1 次/goal_097a…，A3 轮 3 次/goal_5ff4…），无一次弹回 |
| 5 | **A3 到达提案面**：needs_experiment 提案在卷（「通过共享的混响返回链增加少量反射与尾音，使鼓组听感更连贯」，可逆小步假设+两枚鼓组观察证据 obs_20261021T123215/123230） | `.vit_history/.../agent_runtime_state.json` loops[softguide_b1_A3…].latest_decision；closure candidate_id=**user_named:1012**（user_intent candidate 被观察选中） |
| 6 | **A1 指名轨成为唯一候选后诚实自报边界**（非弹回）：「用户指定的人声轨（轨道 1032）是本轮唯一候选目标，但其目标级观察返回为不可用……任何量化调整都属于无据猜测」→ capability_blocked | loops[softguide_b1_A1…].latest_decision.summary/limitations；closure frontier 仅 user_named:1032 |

prepare 解析链路真栈在卷：agent log `kernel.vsp_state_snapshot reason=user_named_target_resolution` ×9 + `harness.refresh_shadow reason=user_named_target_resolution` ×9（每次 prepare 实际刷新 shadow 后解析）。

## 诚实边界（非本卡域，如实上交）

- A1：合成 sine fixture 上指名轨目标级视图（声场/时间包络）返回不可用——观察被放行但内容不支撑证据，模型诚实拒绝猜测。属观察供应面（156 RESULTS §① 供应不稳定性同族），非 F-gate。
- A3：提案在 D1-S1 experiment admission 被拒（`free_state_experiment_admission_invalid`：D1-S1 target must match the fresh observation target——提案目标与新鲜观察目标一致性门）。提案面已到达，admission 一致性属 109/151 族邻面，非本卡修复面。

## 工件清单（随 port/fs-named-target-1 入库）

- `arm/session_summary.json`+`arm/arm/{A1,A3}/`（prompt/chat_1/events/runtime_status_final/send_list 前后/project_state_after）
- `logs/agent_last.log`+`logs/driver_console.txt`；`softguide_exp.py`（驱动原件留档）
- `fixture/`（六 stem 合成 fixture+会话史 agent_runtime_state.json——判定面 2/3/5 的持久证据源）；`kernel_workspace/`（隔离工作区）
