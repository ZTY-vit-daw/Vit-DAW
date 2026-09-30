# FS-PARK-TURNFAIL-1：judgment park 存活下新用户消息的三拒终局失败 + turn id 复用语义（M8 手测实测，P1）

- 池序 14（P1 用户面失败，FS-SETTLE-TERMINAL-1 的同族下一案）；来源=M8 手测活栈取证（证据=coord/runs/M8-FORENSIC-20260930/evidence/：事件流/ui-state/message-loop 三件）
- 优先级 / 预估 / 依赖：P1 / 取证 0.5 天+修复 0.5–1 天 / FS-SETTLE 已合入（本卡是其姊妹案：闭包边界已修，轮终局门仍败）
- 模型分级：L2 / GLM 亲自（自由态门语义+LLM 引导设计）
- 已核实事实（2026-09-30 22:04–22:08 活栈，webui 会话 webui_muo6fygb，goal_6da74bd8/run_e5796736，草稿工程 draft_20260930T140416）：
  1. **第一轮**（22:04:59 输入"检查一下当前工程有什么问题"）：turn.started→22:05:04 turn.completed 输出正常；后续 d1 结算切片继续跑 items（22:05:11-22:05:22）→22:06:37 再次 turn.completed（**同一 run_id/turn_id 生命周期翻转：started→completed→items→completed**）；audition A/B 已渲染、trajectory.user_judgment.requested×2、loop 进 judgment park。
  2. **第二轮输入（22:07:20）**：无新 run/turn id——**被续进同一未终局 goal**（复用 run_e5796736）→ 22:07:38 准入门拒绝（needs_experiment 全准入门缺口）→ 22:08:07 加强重试仍不可解析 → `free_state_terminal_fallback: free_state_terminal_turn_unparseable: no admissible final decision after one strengthened retry` → **22:08:07 turn.failed（用户面"输出失败"）**。
  3. **前置同类门拒绝**：22:06:23 gate 拒二次 needs_experiment 准入（"applied experiment round pending settlement...emit the settle report"）——LLM 在 park 场景反复给不出 settle report/合规终局决策（含 22:06:01 一次 parse_failed）。
  4. **50s 回执耗时初判**：22:05:22（末 item 完成）→22:06:37（turn.completed）的 75s 尾段内含 ≥2 次 LLM 往返+解析修复+gate 循环——**耗时主体是 LLM 重试与门循环，非 CCB 数据组装**（用户感知的"CCB 回执清单 50+s"是尾段表象）；精确 per-call 归因（llm telemetry，UTC 时间戳）在本卡内完成。
- 目标：
  1. **取证**：a) park 存活+新用户消息的路由语义——goalrunner 同会话续用未终局 goal 时，新消息应允许"回答用户+保持 park"而非强制 settle/终局决策（锚定 goalrunner_chat 续用条件与 final gate 的交互）；b) 三拒失败链的逐次 prompt/response 回放（telemetry），LLM 实际产出了什么、gate 错误引导为何不收敛；c) turn id 复用对下游（webui 消息序/轨迹动效，关联 WEBUI-MSG-ORDER-1）的语义面。
  2. **修复**（**用户裁定已定方向，2026-09-30，decisions/2026-09-30-park-adoption-and-judgment-card-ruling.md**）：a) park 期间**新用户输入 = 对待裁决段默认采纳**——保留已应用状态、关闭该轮（**新一轮新 turn id**，解决生命周期翻转）、进入新一轮思考；结算记录如实标注 adopted_by_continuation 语义（**不得记为 human_confirmed**——证据链诚实边界）；b) park 不强制终局：门拒绝/超时不得 turn.failed，改为用户可读边界呈现（人耳判断 POST 仍是显式 settle 通道）；c) A/B 卡在新输入后被 settle 为"默认采纳"终态（与 AB-JUDGMENT-CARD-1 联动）。
  3. **回归**：复刻本场景（观察问答→实验→park→新用户消息）——修复前红（turn.failed）/修复后绿（回答落盘+park 保持或合法 settle）。
- 文件域：agent/internal/chat/（goalrunner_chat/free_state gate/continuation 面）+ agent/internal/agentloop/（final gate）——实锚后申报；测试。
- 验收标准：取证报告+修复 diff+回归红绿+全量 0 FAIL+真栈复验（park 中新消息场景 exit 0 或用户可读边界）——烟测扩展按 AGENTS §5。
- 停止条件：取证发现 park 路由在架构层无定义（需要产品设计裁定）→ 上交决策侧。
- 领取 / 回执 / 验收：空行待填
