# 手测场取证报告（2026-10-03 09:28–09:45 活栈，决策侧只读探针）

> 背景：用户合并手测场（清单 coord/reports/2026-10-03-manual-test-checklist.md）反馈 7 项问题；现场未关，决策侧按白名单 GET（/agent/runtime/status、/agent/state、/agent/events?conversation_id=）取证。栈：Godot(1796/28780)+VitApp(17180)+VitAgent(18740，09:28:46 启动，D:\Vit_DAW\agent\bin\VitAgent.exe mtime 10-02 22:09 ✓ 新鲜)。全程零写操作。

## 问题 7（A/B 判定后无新回应 + 二轮无输出）——事件流定性

conversation=webui_murptx58，43 事件全量在 `probe_events_webui_murptx58.json`。

**判定链本体：全通（修复生效）**。seq 22-23 尾段记录落地（trajectory.intervention.materiality / trajectory.target.response——Mac 侧 D1-SETTLE-TAIL 的停滞形态在 PC 本场未复现）→ seq 24-27 audition.prepare/candidate.ready → seq 28 **trajectory.user_judgment.requested**（判定通道开启——原 SETTLE-STALL 缺陷"三层全关"已修复）→ seq 31 turn.completed(goal_status=waiting_continue) → 用户点击 → seq 34 **trajectory.user_judgment.recorded**(candidate 1017) → seq 35 round.decision → seq 36 **trajectory.settled** → seq 37 **judgment.settled**(audition:turn:free_state_2c22bedaf4b4acb4:round-1)。结算应用确认文案落工程 checkpoint（01:32:39Z manual："这一步已经应用好了：Track 1017音量 +3 dB（回读 0 dB → 3 dB）…"，probe_state.json project_history.recent_checkpoints[0]）。

**症状 7a（无可见新回应）**：seq 37 之后**无任何消息投递事件**，结算确认只存在于 checkpoint，未进对话流——缺口在 settle→对话消息发布/渲染面（新形态，与原卡"判定通道全关"不同根）。

**症状 7b（二轮输入未执行）**：seq 38-43——turn.started → item.started(mix_observe, mom_intent=action_preflight_observation) → item.completed → turn.completed **stop_reason=project_revision_stale**（completed_steps=0）；语义态 closed，transition_reason="chat turn closed without governed experiment"（task_a438fa11ad71735d，contract kind=diagnostic / observe_and_propose_only，25 秒即终）。判定结算应用变更推高工程修订后，新 goal 撞陈旧修订守卫。

**症状 7c（二轮消息插队显示在首轮输出上方）**：用户目视报告；渲染面（消息序），与 7a 同场景待 RED 编码。

## 其余问题取证状态

- 问题 1/2（容器拉伸角标位置/锚点图钉语义与层级遮盖）：设计修订类，无栈取证需要；用户裁定推翻 10-02"锚点拖动=移动位置"语义（详见 decision）。
- 问题 3/4（note 会话"未命名对话流"/webui 不自动建主对话流）：会话语义设计缺口；note 会话命名并入 NOTESTREAM-2 卡面，主/副会话语义另立卡。
- 问题 5（webui 初启布局挤压，一次对话后自行展开）：截图固化 `webui_initial_layout_squeezed.png`（用户 09:30:12 截屏）。
- 问题 6（执行轨迹不随对话流切换，note 的 "hi" 残留）：渲染绑定面缺陷；本场 note 会话 id 未取证（事件面按 conversation 过滤，note 会话 id 需从 Godot 侧取），卡内留取证步。

## 工件清单

probe_runtime_status.json / probe_state.json / probe_events.json（首探，需 conversation_id）/ probe_events_webui_murptx58.json / webui_initial_layout_squeezed.png / 本报告。
