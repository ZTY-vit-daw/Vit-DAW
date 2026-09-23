# 2026-09-23 决策记录：REALSTEMS 验收三项裁定（EQ-1 升 P1 / masking 登记 / 开源就绪 backlog）

- 会话：Mac 决策会话（角色跟会话不跟机器，PROTOCOL §3）；用户批准「可以发卡」后落盘
- 上游：PORT-REALSTEMS-MAC-1 验收 pass（[rulings/2026-09-23-PORT-REALSTEMS-MAC-1-pass.md](../rulings/2026-09-23-PORT-REALSTEMS-MAC-1-pass.md)）

## 裁定 1：PORT-WL-EQ-1 升 P2→P1

依据（卡面预置规则"若真实素材确动机化 EQ/comp → EQ-1 升 P1"）：

- 真实素材轮（run1_20260923_1232）static_eq **动机化成立**：假设锚定实测带结构（250Hz=bass 56.5dB→low_mid 42.5dB 台阶），非合成轮的字面基频推断；
- **capability_blocked 实证**：`static_eq experiment is not configured: experiment plugin whitelist: static_eq plugin is not configured`（events.json seq17/18，task_state=capability_blocked）——模型想做的实验做不了=演示剧本含插件场景时硬缺口；
- 根因核实（决策侧独立挖掘）：PCA 获取模型=**本机生成管线**（扫描→认证 runner→白名单构建器，产物全在 `~/.vit/`），非自带非找回。EQ=「设计完成从未供给」占位族（`processor_selection.go:13` 注记"broadband_compression 先行，static_eq 占位"；认证战役 24 主体 PC↔mac 同名单均无 EQ）；broadband=v1 存储有 C1 comp 3 条认证但单共享 Threshold 不合 v5 双通道形态。机器语义索引 eq:77——候选主体充足，补齐路径可行。

卡面已同步修订：P1 + 目标 5 真实素材旅程回归（基线 run1_20260923_1232 的 capability_blocked 消失为判定标准）+ broadband 形态决策点停止条件 + 领取提交完整 mv 注记。

## 裁定 2：masking 不 ready 登记为独立已知缺口、暂不开卡

- 事实：真实素材轮 project_mix_profile 层显式限制 `masking_analysis_not_ready_on_current_project_cut`（MOM bundle 亦 partial：missing_track_count=1）。
- 定性：观察层 project cut 面缺口，**不由 EQ-1 闭合**；本轮频段占用面已就绪（timbre_frequency 六带+band_occupancy 证据引用），masking 缺席未阻塞旅程。
- 处置：登记不开卡；待 EQ-1 完成后按演示剧本是否需要 masking 场景再定。

## 裁定 3：开源就绪 backlog 登记（两件，不开卡）

面向「Vit 开源后用户如何使用 PCA」的产品化欠账（本轮挖掘暴露）：

1. **白名单构建器未入仓**：`build_whitelist.py`（四源程序化提取）活在 `~/Documents/vit-wl1-artifacts/` 工件目录，`git grep build_whitelist` 为空——用户侧无可复用的"从我的插件库生成白名单"入口；
2. **用户侧认证旅程未产品化**：认证战役的主体选择（哪 24 个）是开发期人工决定，无面向用户的"认证我选的插件"流程或文档（`scripts/pca_calibration_chain_mac.sh` 是 mac 开发侧入口）。

处置：登记待办；EQ-1 完成后按发布计划决定是否开产品化卡（可能并入开源发布准备 Epic）。

## 附：流程备忘（不另立卡）

- 领取提交应一次含完整 mv（todo 删除+doing 新增同批）——REALSTEMS 领取提交 9050785 只含 doing 副本、todo 删除悬到回执提交 cdfecb7 才闭合，终态正确但不应复制；已写入 EQ-1 领取注记。
