# MIXTICK-CONFIRM-CONTRACT-1 取证报告（零 LLM 读档，2026-11-06 晚窗）

- 执行：ZCode GLM-5.3 L1 PC 执行会话（262impl）；worktree D:/Vit_DAW_wt_262impl @ port/mixtick-confirm-contract-1（领取基线 2a801e81，领取提交 0db83f8b @ main）
- 对象：mix_single_tick 失败 run `coord/runs/AF-FAMILY-HEALTH-1/mix_tick_20261105_224459/`（conversation=mix_single_tick_e2e_20261105_224459，goal_34031131163923ef）vs M3 措辞链前通过形态 `coord/runs/FIX-F3-G4-SEMANTICS/run3_mix_single_tick_after_A.log`（2026-09-22，conversation=mix_single_tick_e2e_20260922_192905，基线 166d313a）
- 补充证据源（257 worktree 在机亲读）：`D:/Vit_DAW_wt_257impl/VitApp/Workspace/Logs/agent_message_loop_debug.jsonl`（29 条原始输出）、`agent_prompt_render.jsonl`（32 轮指纹+指令面）、`agent_llm_telemetry.jsonl`（计费面）
- 本腿零 LLM 运行、零栈占用（纯读档）。

## 〇、结论摘要（供主管裁定）

1. **卡前提被证据推翻**：失败 run 中模型**并非**「观察→终局文字建议不挂卡」——29 条 loop debug 原始输出亲证模型自始至终输出结构化 needs_observation 决策（带 tool_calls），从未给终局文字；切片后的续跑 turn 32 **正确产出 needs_experiment + improvement_proposal**，chat server 正常挂确认卡（响应面 stop=improvement_proposal_confirmation_required，needs_confirmation=true，interaction 落库）。needs_confirmation 契约在模型侧与响应面**均成立**。
2. 真实失败链 = 三层叠加（详见 §二）：
   - **L1 行为漂移（M3 措辞链同族背景）**：模型反复重请求已返回过的 project 级 view set（要「两条轨道相对电平」），被 view-set 去重门弹回 27 次；
   - **L2 弹回反馈丢失（agentloop 机制面）**：弹回 appended 的 `<final_gate>` 反馈未进入后续轮重建请求——turn 6-31 指纹恒定 `fd94d36a` ×26、请求逐字节相同，模型面对同一题目被重问 26 次且看不到任何拒绝史 → 180s 窗口排干 → 切片；
   - **L3 settle 投影分裂（ps1 只读 goal.status）**：续跑 turn 32 停对提案面后，runner 级 res=completed/done（observe 环提案记录即终），durable 记录 parked waiting_interaction（f6.gate 亲证），但 `/agent/runtime/status` goal.status 读到 **completed** → ps1 settle 映射 `completed→'done'` → 断言失败。确认面真实存在于 continuations[].pending_interaction，ps1 未读。
3. **裁定选项评估**：
   - **②「纯建议不需卡族→断言收窄」：证据反对**。模型挂卡成功、契约可达且被达成；收窄会把真实成立的契约行为改掉。
   - **①原措辞（提案类话术须挂卡）修补：不对症**。模型不缺挂卡意愿/指引——它缺的是「现有证据足够提案」的判断。逐字有效的句子已存在：`ccb_model_prompt.go:292` 闭合相注入句 **"The evidence you hold is sufficient for a bounded proposal … use needs_experiment with one bounded improvement_proposal.v1 citing the returned observation"**——turn 32 与 turn 6 的 turn_directives 逐字节 diff 唯一差异即此 633 字符，注入后模型**第一轮即改走提案**。
   - 供裁定的两个在域修补选项（文件域二选一或组合）：
     - **选项 A（chat 措辞面）**：把 :292 的「证据足够提案」句在「观察台账中该 view set 已返回 usable 证据」条件（即去重门同判据）下提前进入 turn_directives——不新增话术家族、复用现行句子、needle 面只增不删。配合 A 需注意：L2 反馈丢失不修，措辞只在指令面生效（指令面每轮随状态重建，不受 L2 影响，可行）。
     - **选项 B（scripts 腿断言面）**：settle 合成在 goal.status=completed 但 continuations[].pending_interaction 携带 answerable park（waiting_confirmation/improvement_proposal_confirmation）时，按既有 waiting_confirmation 重建路径恢复确认面（该 walk 已存在于 ps1：needs_clarification fallback 与 waiting_confirmation 重建两处，仅扩触发条件）。B 不改模型行为，修的是投影信息丢失。
   - 域外发现（建议另开卡，本卡不动）：L2 弹回反馈不持久（message_loop.go:10947 append 后被每轮状态重建丢弃）；L3 goal 级投影不保留 interaction park 的确切 Go 翻转点（本报告已括定到 recordGoalResult 终态记录或 settle 后收敛，见 §二.6）。

## 一、两形态对照

| 维度 | 通过形态（09-22 run3，166d313a） | 失败形态（11-05，257 batch2） |
|---|---|---|
| 观察回合 | 单 HTTP 回合内完成 | 32 轮 LLM：T1-T3 观察、T5-T31 被弹回 ×27、切片后续跑 T32 |
| 模型行为 | 观察后直接产提案（`ck.pending stored`） | T5 起重复请求已返回 view set；T32 闭合句注入后产提案 |
| 停靠面 | observe stop=needs_confirmation（proposal face）→ hop-1 工具面 → hop-2 d1 评估 | 响应面同为 improvement_proposal_confirmation_required（timing :321），但经切片+settle 后 ps1 读到 'done' |
| system prompt | 14,331 chars 时代（M3 前） | **17,670 chars**（=256 取证 armrun2 同值，M3 措辞链后） |
| 引擎 | （历史 run 未记录，同脚本族） | deepseek-flash（telemetry 亲证） |
| 腿结果 | PASS（断言直接读响应面） | FAIL @ run_mix_single_tick_e2e.ps1:926 got 'done' |

## 二、失败链逐层钉死（全部有原始工件指印）

### 1. 观察段正常（T1-T3）

- T1 fp=20c8305a（in 15457）→ `ccb.observation_catalog`；T1 原始输出 parse_failed 一次后修复（debug #1-2）。
- T2 fp=063f76f7 → `ccb.observation_request` **track.basic_energy(1007)**：返回 Track 1 峰值≈-6 dBFS / RMS≈-9 dBFS / 余量≈6 dB（模型自述可证）。
- T3 fp=f2c2ebc8（887ms 最大返回）→ project 级 set（project.structure + mix.multitrack_relationship）。**TIM 投影在此两腿报 Track 2(1010) 削波**：`clipping_headroom fail value=0`、`level_ceiling fail value=0`（agent_last.log 22:45:21/22:45:42 ×4 行）——「唯一一处小幅音量调整」的判据证据（Track 2 该降）此时已在观察面在场。

### 2. L1 行为漂移：view-set 重复死锁（T5-T31，弹回 ×27）

- T5 起模型输出恒为结构化 `needs_observation` + tool_calls 重请求 `[project.structure, mix.multitrack_relationship]`（T6 起收窄为 `mix.multitrack_relationship`），自述理由=「缺少两条轨道之间的相对电平对比，无法确定调整目标」。原始输出见 debug #3-#29（本报告附件 excerpts）。
- final_gate 判据 `free_state_reasoning.go:1932`：project 级 view set 全部已在台账 available_views → "already returned usable evidence; choose a different cataloged view or target instead of repeating it"，弹回 ×27（debug #3-#29 同错误串）。
- 判读：模型把「建议哪条轨道」的证据门槛自行抬高为「需要显式相对电平披露」，无视既有教学（收敛指引明文 "What wastes the budget is re-requesting views that already returned"）与台账内 Track 2 削波事实。此为 256 已证 system+23% 同族背景下的行为面漂移——**无任何 M3 新增句子合法化该重复**（M3 增量逐条核对：235/243/246 行动路径教学、227 automation 准入行、246 豁免子句，全部针对时变目标直调路径，与本场景（静态音量+禁插件）无语义交叠）。

### 3. L2 弹回反馈丢失：26 次同题重问（机制面缺陷）

- 弹回路径 `message_loop.go:10947` 将 `<final_gate>`+issue append 进 `state.input.Conversation`，但 T6-T31 的请求指纹**恒定 `fd94d36a`**（telemetry 亲证 ×26）、消息形状恒为 3 条 60574 chars（prompt_render 亲证 turn 6-31 逐轮同指纹、turn_directives 逐字节相同 r5==r6）——append 未随重建进入请求，模型看不到任何拒绝史，等效确定性重采样同一回答直至 180s 窗口排干（`limit=timeout`，agent_loop_chat total 186.9s）。

### 4. 切片与续跑（T32 一击即中）

- 22:48:06 切片（stop=limit_reached），durable cont_9474da5d 武装；22:48:07 ps1 收到切片响应进入 settle 轮询。
- 22:48:07-17 续跑 T32 fp=d842ad0e：与 fd94d36a 的 turn_directives diff **唯一差异 = `ccb_model_prompt.go:292` 闭合相句**（"A target-level observation … **The evidence you hold is sufficient for a bounded proposal** … use needs_experiment with one bounded improvement_proposal.v1 citing the returned observation"，633 字符）→ 模型第一轮即产 needs_experiment+improvement_proposal。
- chat server `improvement_proposal_workflow.go:160-190`：resp.NeedsConfirmation=true / GoalStatus=waiting_confirmation / StopReason=improvement_proposal_confirmation_required / 确认卡落库（timing :321 亲证该响应面）。

### 5. L3 settle 投影分裂（腿失败的直接原因）

- 同一秒三面分歧（agent_last.log :319/:321/:322 亲证）：
  - runner 级 res：`[continuation.arm] status=completed stop=done … continuation=false requires_user_interaction=false`（goalrunner_chat.go:3717 打印 res 本体；observe 环提案记录即环终态）；
  - 响应面：`status=waiting_confirmation stop=improvement_proposal_confirmation_required`；
  - durable 记录：f6.gate `current.Status=waiting_interaction`（continuation_scheduler.go:2227-2236 interaction park 路径），随后 `[continuation.park] answerable park holds the chain` 持续在日志。
- `/agent/runtime/status` 的 goal 取 `harness.RuntimeStatus("")`（server.go:837-848）——ps1 轮询读到的终值是 **completed**（console "settle poll: goal=running → goal=completed"）。f6.gate 当刻 runtimeGoal 仍 waiting_confirmation，goal 翻 completed 的确切调用点被括定在 recordGoalResult 对 runner 终态的落账或其后的链尾收敛（本卡文件域外，未继续深挖；如需精确到行另开取证卡）。
- ps1 settle 映射（run_mix_single_tick_e2e.ps1:355-368）：`completed → 'done'`；仅当 settledGoal==waiting_confirmation 才走确认面重建（:398-430，含 continuations[].pending_interaction walk）。→ 断言 :926 got 'done'。

### 6. 判定：行为回归 or 断言过时？

**都不是单一定性**——G6 的二分法前提（模型不挂卡 vs 契约过时）被证据修正为：契约未被违反（提案面最终成立），腿失败=「行为漂移把回合推进切片路径」×「settle 协议在该路径下丢失确认面」的复合结果。修补应对症分层（见 §〇.3）。

## 三、证据索引

| 证据 | 位置 |
|---|---|
| 腿断言与 settle 映射 | scripts/run_mix_single_tick_e2e.ps1:926 / :355-368 / :398-430 |
| 27 次弹回原始输出（结构化 JSON 非纯文本） | D:/Vit_DAW_wt_257impl/VitApp/Workspace/Logs/agent_message_loop_debug.jsonl（本会话已抽 3 条入 excerpts.md） |
| 指纹恒定 ×26 / T32 指纹变化 | coord/runs/AF-FAMILY-HEALTH-1/mix_tick_20261105_224459/agent_llm_telemetry.jsonl + agent_prompt_render.jsonl（257 worktree） |
| T32 唯一 diff=闭合句 633 字符 | 本会话 difflib 比对 t6/t32 turn_directives（脚本见 excerpts.md 附注） |
| TIM 削波证据在观察段在场 | agent_last.log 22:45:21/22:45:42 [tim.assert] ×4 |
| 响应面 vs runner 级 vs durable 三面分歧 | agent_last.log :319/:321/:322 + f6.gate |
| M3 措辞链增量（排除「合法化重复」可能） | git diff 4a439588^..40cdd189 -- agent/internal/agentloop/ccb_model_prompt.go（本会话亲读） |
| 决定性句子现行代码位 | agent/internal/agentloop/ccb_model_prompt.go:292（:288-297 闭合相分支族） |
| view-set 去重门 | agent/internal/agentloop/free_state_reasoning.go:1904-1940（:1918/:1932 判据句） |
| 弹回 append（反馈丢失面） | agent/internal/agentloop/message_loop.go:10905-10948 |
| 提案面构建 | agent/internal/chat/improvement_proposal_workflow.go:22-199（:163 stop/:188-190 卡落库） |
| interaction park / goal 投影 | agent/internal/chat/continuation_scheduler.go:2189-2330、server.go:837-848 |

## 四、验证腿状态

- **HELD（等待主管裁定修补方向后执行）**：真栈 mix_single_tick 单 run（§8 口径：单 run ≈¥1-3、无重跑、成功条件=EXIT=0 且 observe 断言过；失败分类=环境中断/行为再漂移/修补无效三分记录）。占栈前按协议 fetch 重读 PC-RUNTIME-STACK.md 登记即时复核。
- 若裁定选项 A（措辞面）：验证腿=补丁后单 run（预期 observe 回合内直停提案面，不进切片路径）。
- 若裁定选项 B（scripts 面）：验证腿=B 后单 run（预期即便进切片路径，settle 从 pending_interaction 恢复确认面）。
- 若裁定「两选项都不做/另开卡」：验证腿可降级为「现状复跑取证」（证明失败可复现+计费面记录），或按裁定取消。
