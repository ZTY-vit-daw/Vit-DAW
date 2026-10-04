# FREESTATE-REGION-GOAL-1：自由态范围目标两轮制协议——goal 携带 range 操作目标+轮 1 拆分取证/轮 2 调改判定（设计先行，P2）

- 池序 8（设计卡先行；实现分期另立）；目标仓库=D:\Vit_DAW（PC 执行侧）；来源=[decisions/2026-10-04-region-op-path-and-freestate.md](../../decisions/2026-10-04-region-op-path-and-freestate.md) 裁定 2/3（自由态默认方向+两轮制基线）
- 优先级 / 预估 / 依赖：P2 / 设计 0.5 天 / **REGION-INTENT-WIRE-1 合入后领取**（意图层 ranges 消费是同一上下文源）
- 模型分级：GLM L1/L2（harness 治理域：goalrunner/agentloop prompt 与协议面）
- 设计问题（产出=协议设计文件 `coord/runs/FREESTATE-REGION-GOAL-1/DESIGN.md`，带代码锚点，不开实现）：
  1. **范围操作目标进 goal**：selected_clip_ranges 如何成为自由态 goal 的操作目标（goal 文本携带 vs 结构化字段；与 goalrunner 现有目标形态的兼容面）。
  2. **两轮制协议**：轮 1 拆分（前向变更=split，post 取证=结构正确+听感基线渲染，settle 形态）；轮 2 调改（treatment=A/B 或参数变更，既有判定/结算链复用）。轮间交接=子 clip id 传递（内核 split 回传 left/right_clip_id，RECON A2）；D1 单变更/轮规则零改动下的 prompt 措辞与 goalrunner 状态机适配面。
  3. **用户侧形态**：goal 发起话术（"把这段处理到不闷"式目标+框选在场）；两轮各出的确认/报告卡形态。
  4. **边界**：多 range/跨 clip 场景的显式不支持声明（v1 单 clip 单 range）；与 B2（自动化限段，后续方向）的协议留位——**两轮制仅 a/B1 路径协议，B2 将来单轮原生合规**（决策 2026-10-04 裁定 3 修正），本卡目标形态不锁死"拆分"实现。
- 约束：只读勘察+设计产出；D1 规则文案（ccb_model_prompt.go:131/133）不在本卡改动（两轮制=零规则改动基线）；不动 sealed fixture。
- 验收标准：四问全答+锚点可回查+协议可立实现卡（实现卡粒度+分期建议明确）；决策侧复核后出实现卡。
- 领取：2026-10-04 晚 / 基线 8cd98a60（决策侧亲自收口设计段——先例 CONTAINER-3）
- 回执：[DESIGN.md](../../runs/FREESTATE-REGION-GOAL-1/DESIGN.md)——四问全答+锚点地图 D1-D7（beginChatGoal:32/agentloop 白名单 helpers.go:143-145/runtime.Goal:45/轮状态机 goalrunner_chat.go:280-330+audio_closure_controller/D1 prompt 131-133/split 子 id 回传）；协议=loop 状态增 range_goal 字段（fail-open）+两轮编舞（轮 1 拆分确定性断言 settle→轮 2 treatment 全复用判定链）；D1 prompt 零改动逐字兼容；实现拆两张 flash 级卡（IMPL-1 状态机编舞/IMPL-2 话术分界+用户卡），待决策侧复核后立卡。
- 验收：设计产出待用户过目（决策侧自查锚点全实证）；实现卡 FREESTATE-REGION-IMPL-1/2 待立。
