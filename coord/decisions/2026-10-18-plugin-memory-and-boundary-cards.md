# 2026-10-18：插件 memory schema 与能力边界 remediation 两卡授权（用户确认）

## 授权

- 用户 2026-10-18 在 REAPER MCP 调研会话中认可两点方向，并明确「可以出卡，放入池中，决策流会负责安排什么时候执行」。
- 本次授权产物（已写入 `coord/cards/todo/`）：
  - [MEM-SCHEMA-1](../cards/todo/2026-10-18-MEM-SCHEMA-1.md)：插件域 memory entry schema 与 advisory 策略（D14 前置规格，零生产代码）。
  - [CAP-BOUNDARY-1](../cards/todo/2026-10-18-CAP-BOUNDARY-1.md)：能力边界结构化 remediation（L1–L3，确定性生成、无 LLM 判定）。

## 范围与边界

- **MEM-SCHEMA-1**：只定义插件域「客观经验」memory 的 entry schema/写入消费策略；存储、检索、失效、consolidation、画像机制归 `AGENTIC_OBSERVATION_ROADMAP_2026-09-25.md` D14 Memory，本卡不建 store、不改生产代码；消费接线待 D14 落地后另卡。
- **CAP-BOUNDARY-1**：只做拒绝面的 L1–L3（已准入替代 / native / 手动指引 / 认证入口），不放宽 PCA 或 typed 准入，不改内核；L4 只留入口，L5 近似写不做。
- 两卡均**明令禁止**：复活 VPS/Profile/SPAL 执行权威（`docs/SPAL_VPS_RETIREMENT_ADR.md`）；把 memory notes 作为参数映射来源、credential 或可调度能力；让模型自由发明 fallback；默认开启近似写。

## 依赖与排序

- 两卡「派发确认=已确认（用户授权）」；**排序与派发时机归主管决策流**（本会话不排池序、不指定执行 owner）。
- CAP-BOUNDARY-1 与池序 54 `FS-BLOCKED-SURFACE-CLOSURE-1` 同触 `agent/internal/chat/goalrunner_chat.go`，主管排序后两卡串行。
- MEM-SCHEMA-1 的运行时消费依赖 D14 Memory，D14 前只交付规格。

## 依据

- 调研结论：`coord/reports/2026-10-18-reaper-mcp-plugin-param-research.md` §5.3/§5.4（PC 工作区，未入库；卡面已摘录所需结论）。
- `docs/AGENTIC_OBSERVATION_ROADMAP_2026-09-25.md` D14（三种记忆三种键、raw+distilled、收尾拼装）。
- `docs/REF_SCHEMA_V1.md` §6（`memory.entry` / `memory.distilled` 预留命名空间）。
- `docs/PCA_PLUGIN_LOAD_GATE_V1.md`、`docs/AUDITION_AB_AND_USER_JUDGMENT_V1.md` §11、`docs/SPAL_VPS_RETIREMENT_ADR.md`。
