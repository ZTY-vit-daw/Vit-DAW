# PORT-WL-EQ-1：mac 白名单两族补齐——static_eq + broadband_compression（EQ 主体 reprobe+PCA 认证）

- 优先级 / 预估 / 依赖：P2 / 0.5 天 / PORT-WL-1 验收裁定 2026-09-22（回执上交 c 项：两族留空待补齐路径）；C3 reprobe 驱动在 main
- 模型分级：L1 / GLM-5.3 flash 可接（复用 WL-1 的 build_whitelist.py 四源纪律，锚点已实证）
- **目标机器（mac 侧卡）**：`~/.vit/free_state_experiment_plugins.json` 机器本地扩展 + `~/Documents/vit-wl1-artifacts/` 工件；回执写回本 coord（D:\Vit_DAW）
- **背景（WL-1 回执如实留空声明）**：static_eq——C2/C3 实测集六族无 EQ 主体，无实测源不手编（现状：EQ 提案 hop-2 报 not configured）；broadband_compression——C1 comp 实测面为单共享 Threshold，v5 要求 ch1≠ch2 双通道参数形态。两族补齐需先有 EQ/双通道 comp 主体的 reprobe 实测。
- 目标：
  1. 在 mac 上以 EQ 主体（及一枚双通道 broadband comp 主体）为目标跑 C3 reprobe/PCA 校准链，产出参数面快照与认证收据（四源纪律：语义索引身份/attestation 晋升/参数面零漂移/role→param_id 锚定，零手编）
  2. 扩展 `~/.vit/free_state_experiment_plugins.json` 补 static_eq（bands center_hz+gain_param_id）与 broadband_compression（ch1≠ch2）两段；`build_whitelist.py` 溯源表同步扩行
  3. overlay 模式 go test 复验（loader+两段 hop-2 admission 谓词 PASS）+ 一次涉及 EQ/comp 提案面的 ⑤ spot 轮（存在性）
  4. 无法实测的主体如实留空并声明（维持零手编红线）
- 约束：零仓库代码改动预期（如需 probe 驱动参数化最小扩展，独立 commit 列明）；白名单仅机器本地；LLM key 零入工件
- 验收：①两段扩展+溯源表扩行（字段↔实测来源可回指）；②overlay 复验+⑤ spot 轮工件；③回执写回 coord 记两端 HEAD
- 停止条件：mac 实测环境无 EQ 主体可用 → 如实声明留空上交（该族继续 not configured 是诚实边界非缺陷）
- 领取：
- 回执：
- 验收：
