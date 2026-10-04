# INTENT-WIRE-FIX-1（P1）：范围切分合成提到 LLM 之前——两条生产路径确定性接管（根治确认卡随机序）

- 池序 1（P1 缺陷修复）；目标仓库=D:\Vit_DAW（agent）；来源=[rulings/2026-10-04-SMOKE-SCEN-RANGE-1-pass.md](../../rulings/2026-10-04-SMOKE-SCEN-RANGE-1-pass.md) 场景 B 证据链；**产品缺陷**：真实栈上「把这段拆出来」确认卡约半数呈内核会拒绝的错误切序（先起点）——模型包络随机序，意图层只是兜底
- 优先级 / 预估 / 依赖：**P1** / 0.5-1 天 / REGION-INTENT-WIRE-1 已合 main（clipRangeSplitCommands 复用）
- 模型分级：**GLM 首选**（生产链路时序两路径+回退语义；flash 可接需卡面锚点全实锚）
- 缺陷锚点（SMOKE 卡实锤）：
  1. legacy 路径：意图层坐 `chat/server.go:2630` `if len(env.Commands)==0` 兜底位——模型自吐命令即不接管（run 210955/211446：抢先 3/3，顺序随机含内核错误序）。
  2. agentloop 生产路径（Godot 主对话默认）：**完全不经过意图层**——模型逐刀调 split_clip，顺序自决。
- 目标：
  1. **腿 1（legacy/server）**：`SynthesizeLocalDAWCommands` 的范围切分形态（clipRangeSplitCommands 命中）提前到 LLM 调用**之前**确定性接管（话术+ranges 匹配即合成提案、跳过模型调用；其他意图形态维持现兜底位不变——最小改动面，实锚 server 组装段后申报确切挂点）。
  2. **腿 2（agentloop/生产）**：照 strip_silence 快速意图先例（`agentloop/message_loop.go:1253-1281`）加 clip-range split 快速意图——话术（这段/这个范围/框选+拆出拆开拆分）+`selected_clip_ranges` 在场→复用 clipRangeCutCommands 合成两条 split 提案短路模型循环；无 ranges/口述时间/播放头指涉时零变化。
  3. 回归：①`go test ./internal/chat ./internal/conversation -count=1` 全绿+新用例（两路径接管/不接管边界）；②SMOKE-SCEN-RANGE-1 场景 B **严格判据**（tool-form 确定性、先终点后起点）复跑 exit 0——即 INTENT-WIRE-1 转正判据；③`-Scenario all` 全绿。
  4. 回执：两腿实锚挂点+新用例清单+场景 B 复跑 run 工件。
- 文件域：agent/internal/chat/server.go（组装段）+agent/internal/agentloop/message_loop.go（快速意图段）+测试；不碰 ccb prompt/webui/Godot。
- 约束：其他意图形态路由零变化（回归面）；E2E 真栈泊位排他。
- 验收标准：两腿落地+全量回归绿+场景 B 严格判据 exit 0（决策侧复跑）。
- 停止条件：server 组装段/agentloop 快速意图挂点与先例形态不符（无法最小接入）→ 实锚上交定方案。
- 领取：（时间 / origin/main hash / 分支名）
- 回执：（commit hash / 两腿锚点 / 新用例 / 场景 B run 工件）
- 验收：（裁定文件 / 验收 commit）
