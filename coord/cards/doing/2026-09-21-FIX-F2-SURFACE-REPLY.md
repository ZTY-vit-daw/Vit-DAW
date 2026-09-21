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
- 回执：
- 验收：
