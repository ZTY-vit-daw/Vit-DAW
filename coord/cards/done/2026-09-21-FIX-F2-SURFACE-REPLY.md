# FIX-F2-SURFACE-REPLY：turn 级事件承载最终回复（settle 合成丢话修复）

- 优先级 / 预估 / 依赖：P1 / 0.5 天 / 取证定责 FORENSIC-45-FAMILIES（模型零责实证在案）
- 模型分级：L2 / GLM-5.3
- **背景（已破案）**：⑤ R1（conv product_path_vocal_clarify_20260921_201033）stop=needs_clarification 正确，但 settle 合成捕获 reply=「已完成 ccb_observation_request」（工具步标题）。durable 存储实证模型最终问句存在于 continuation trace（goal_continuations/goal_b702…/trace[21]）：**"需要先确认哪条是主唱轨。请告诉我主唱是 Track 几，或把主唱轨重命名为 vocal / 主唱后让我重新观察。"**——缺陷在呈现路径：最终回复落在 trace，turn 级事件 body 未承载。
- 目标：
  1. 代码定位：turn.completed 事件 body 的写入点（chat 侧事件发射），确认 clarify-park 回合（含确定性澄清回退 messageLoopFinalFocusTrackClarification 生成的问句）为何以工具步标题落 body
  2. agent 侧修复：回合终态事件的 body 必须承载最终用户可见回复（含澄清问句）；红测试=构造 clarify-park 回合断言事件 body=问句而非步进标题
  3. 脚本侧兜底（可选双保险）：Wait-ChatTurnSettled 合成时若 stop=needs_clarification 而 reply 无问句特征，回退读 runtime status 的 pending 澄清文本
- §8：断言零改动（双跳断言即被测契约）；红测试修前红/修后绿；真栈 ⑤ 1 轮验证该段（存在性口径）
- 文件域：agent/internal/chat/（事件发射点，定位后收窄）；如做兜底则 scripts/run_vit_product_path_smoke.ps1 settle 段+④ 脚本同款
- 验收：①红测试；②agent 全量+webui 回归绿；③⑤ 真栈该段过（整轮 exit 0 受 F5/F3 修复影响，允许分段验证记录）
- 停止条件：事件发射点跨层（内核侧）→ 上交扩域
- 领取：2026-09-21 晚窗 / PC 执行侧 GLM-5.3 / origin/main=8e6e253d / 分支 fix/f2-surface-reply
- 回执：2026-09-21 晚窗 / PC 执行侧 GLM-5.3 / 分支 fix/f2-surface-reply commit 1177a9c0（**push 遇 GitHub 故障暂挂**：connection reset/443 不通 ×2，本地完好待补推）。
  - **缺陷机制复核（未跨层，chat 侧）**：agentloop 正确 pause（Result.Reply=问句，durable trace[21] 实证）；缺陷在呈现路径两处——① `goalHasLiveContinuationOwner` 把 parked armed checkpoint（recordGoalResult 对 waiting_interaction 记录保持 goalContinuations armed 以便用户答复后续跑）误判为活链 → 投递门 `chainEnded=false` 永远静默，`emitSchedulerChainResultEvent`（turn.completed/chain_result）永不发射，问句死在 chainResp；② `pendingInteractionFromResult` 只认 executed interaction_requests，clarify park 的 durable pending payload 丢 reply（取证 record 逐字复现={status,stop_reason,limit_type}）。
  - **修复**：谓词对齐自身 docstring——armed 且对应 durable waiting_interaction 记录=非自动切片持有者（调度器从不 claim 该状态），park 经 B1-F2 既有分支投递 turn.completed(body=问句) 落会话图且保持可答复；recordGoalResult/chatResponsePendingInteraction 补 pending payload `reply`（runtime status 面可读）。断言零改动（⑤ clarify ask 双跳断言未碰）；content-blind 红线未碰。
  - **①红测试**：2 枚修前红实证（逐字复现取证形态：事件面只剩 item body=「已完成 ccb_observation_request」；pending payload 无 reply）→修后绿：TestClarifyParkSliceEndDeliversQuestionAsTurnEventBody / TestClarifyParkPendingInteractionCarriesQuestion（scheduler_terminal_delivery_test.go）。
  - **②回归**：chat 全包 exit 0；webui 28 文件 324/324 exit 0；agent 全量 go test 仅剩 3 败=F3 会话**未跟踪新红测试** free_state_terminal_adjudication_test.go（其域内红测试先行在途，非本卡回归；chat 包在全量中 ok）。
  - **③真栈 ⑤**：整轮 **exit 0**（"product-path lifecycle + mix smoke passed"，artifact_dir=VitApp/Workspace/Artifacts/smoke/product_path_20260921_221052，console log=coord/runs/FIX-F2-SURFACE-REPLY/run1_product_path_f2fix.log）。F2 段存在性口径命中：vocal_focus conv park 于 waiting_confirmation，settle 后用户可见 reply=「等待确认 需要先确认哪条是主唱轨。请告诉我主唱是 Track 几，或把主唱轨重命名为 vocal / 主唱后让我重新观察。」（问句原文由 turn.completed title+body 承载，工件 chat_vocal_focus.json）；durable pending payload 带 reply=问句（draft_20260921T141102_0babf014 cont_912f…）——**before/after 对照**：同晚 22:04 F5 会话（修前二进制）product_path_vocal_focus_20260921_220450 同形态 payload 无 reply。本轮 clarify conv 走 done/safe-noop 分支未触发 needs_clarification park（概率分支；F2 机制经 vocal_focus park 形态实证）。
  - **兜底**：④+⑤ 脚本 Wait-ChatTurnSettled 同款（needs_clarification 且 reply 无问句特征→读 runtime status continuations[].pending_interaction.reply，码点拼问句特征避 PS5.1 ANSI 陷阱，PSParser 解析过）；⑤ 兜底因共享工作树时间窗被 F5 会话 4e4d850d 一并带入 fix/f5-snapshot-freshness 分支（其卡已 done 待合入，本卡不重复提交）；④ 在本分支 1177a9c0。
  - **并行会话事件如实记录**：领卡 coord commit e36338ac 因共享工作树把 F5 卡移动一并带入（内容=F5 所愿的 todo→doing，无害，提交信息未提及）；执行期间 F5 完成归档（main 0bd92f00）、F3 进行中（工作树含其未提交改动，本卡提交仅含自己 5 文件域）。
- 验收：
