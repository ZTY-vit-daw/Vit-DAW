# FIX-BROADBAND-SHARED-1：v6 broadband 条目允许共享阈值单参数形态（决策点 A 承接卡）

- 优先级 / 预估 / 依赖：P2 / 0.2-0.3 天 / FIX-PLUGIN-SELECT-1（v6 机制已族无关）；PORT-PCA-FULL-CANDIDATES-1 决策点 A 裁定（2026-09-23 PC 决策会话：允许共享形态）
- 模型分级：L1 / GLM-5.3 flash 可接（单包结构体+绑定小改，先例齐全）
- **背景**：现 broadband 白名单条目为双通道硬形态（`threshold_param_id_ch1/ch2` 必填，whitelist.go:72-78）；mac 的 12 个 Waves comp 实测全为单共享 Threshold——硬形态导致该族持续留空。裁定允许共享单参形态，先例：de_esser v5 条目即单 `threshold_param_id`（FAM1-S1），端口已支持单条目批写同 revision advance（staticeq_vsp Preflight 注释明示合法形状）。
- 目标：
  1. `BroadbandCompressionPlugin` 支持 `threshold_param_id` 单字段形态（或 ch2 可选——二选一以与 de_esser 字段名对齐为优），loader 双形态收（dual 照旧/单参合法），条目形态歧义（两字段都缺/都有）fail-closed
  2. D1 broadband 准入与绑定支持单参条目（param_id_ch2 省略=单条目批写，对齐 FAM1-S1 语义）；现有 dual 条目（PC Vertigo VSC-2）行为不变
  3. 红测试：单参条目加载+admission+绑定/双形态往返/dual 回归/歧义 fail-closed
- 文件域：`agent/internal/experimentplugins/whitelist.go`（+测试）+ `agent/internal/chat/free_state_d1_plan_table.go`（broadband 段，+测试）
- 验收：①红测试修前红/修后绿；②agent 全量+webui 绿；③PC 现网真栈不回归（现有 dual 白名单行为不变——单测+一次 ④/旅程 spot 即可）
- 停止条件：发现端口写路径对单参 broadband 需改 executionports 才能绿 → 上交（预期不需要——FAM1-S1 同款语义已支持）
- 领取：
- 回执：
- 验收：
