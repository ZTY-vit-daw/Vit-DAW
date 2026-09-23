# PORT-WL-EQ-1：mac 白名单两族补齐——static_eq + broadband_compression（EQ 主体 reprobe+PCA 认证）

- 优先级 / 预估 / 依赖：**P1（2026-09-23 升格，REALSTEMS-MAC-1 动机面实证：EQ 动机化成立且被 capability_blocked——模型想做的实验做不了=演示剧本含插件场景时硬缺口；rulings/2026-09-23-PORT-REALSTEMS-MAC-1-pass.md 裁定 1）** / 0.5-1 天 / PORT-WL-1 验收裁定 2026-09-22（回执上交 c 项）+ REALSTEMS-MAC-1（旅程回归基线）；C3 reprobe 驱动在 main
- 模型分级：L1 / GLM-5.3 flash 可接（复用 WL-1 的 build_whitelist.py 四源纪律，锚点已实证）
- **目标机器（mac 侧卡）**：`~/.vit/free_state_experiment_plugins.json` 机器本地扩展 + `~/Documents/vit-wl1-artifacts/` 工件；回执写回本 coord（D:\Vit_DAW）
- **背景（WL-1 回执如实留空声明）**：static_eq——C2/C3 实测集六族无 EQ 主体，无实测源不手编（现状：EQ 提案 hop-2 报 not configured）；broadband_compression——C1 comp 实测面为单共享 Threshold，v5 要求 ch1≠ch2 双通道参数形态。两族补齐需先有 EQ/双通道 comp 主体的 reprobe 实测。**2026-09-23 决策侧核实补充**：PCA 获取模型=本机生成管线（扫描→认证 runner→白名单构建器），非自带非找回——EQ 属"设计完成从未供给"占位族（processor_selection.go:13 注记"broadband_compression 先行，static_eq 占位"；v1 存储有 C1 comp 3 条认证，broadband 缺口=形态不合而非丢失）；机器语义索引可见 77 个 EQ（plugin_semantics.json eq:77），主体候选充足。
- 目标：
  1. 在 mac 上以 EQ 主体（及一枚双通道 broadband comp 主体）为目标跑 C3 reprobe/PCA 校准链，产出参数面快照与认证收据（四源纪律：语义索引身份/attestation 晋升/参数面零漂移/role→param_id 锚定，零手编）
  2. 扩展 `~/.vit/free_state_experiment_plugins.json` 补 static_eq（bands center_hz+gain_param_id）与 broadband_compression（ch1≠ch2）两段；`build_whitelist.py` 溯源表同步扩行
  3. overlay 模式 go test 复验（loader+两段 hop-2 admission 谓词 PASS）+ 一次涉及 EQ/comp 提案面的 ⑤ spot 轮（存在性）
  4. 无法实测的主体如实留空并声明（维持零手编红线）
  5. **真实素材旅程回归（2026-09-23 增补，终极验收）**：`scripts/journey1_demo_journey_smoke_mac.sh --stems-dir ~/Desktop/cases/spv1_p01/stems` 原命令原素材重跑——判定标准=基线轮 `~/Documents/vit-realstems-artifacts/run1_20260923_1232/` 里的 `static_eq … capability_blocked` 从模型轨迹消失，EQ 路径走完『假设→准入→装载→A/B』可观察；旅程断言面保持全绿。**注**：模型是否恰好再次提出 EQ 假设有随机性——若本轮模型未动机化 EQ，以 overlay 谓词+⑤ spot 轮为准并如实记录"回归轮未触发 EQ 提案"，不硬凑（§8 纪律），可加跑一轮提高命中率（两轮内未触发也如实记录上交）
- 约束：零仓库代码改动预期（如需 probe 驱动参数化最小扩展，独立 commit 列明）；白名单仅机器本地；LLM key 零入工件
- 验收：①两段扩展+溯源表扩行（字段↔实测来源可回指）；②overlay 复验+⑤ spot 轮工件；③**旅程回归工件**（对照基线的 capability_blocked 消失证据或如实记录未触发）；④回执写回 coord 记两端 HEAD
- 停止条件：mac 实测环境无 EQ 主体可用 → 如实声明留空上交（该族继续 not configured 是诚实边界非缺陷）；认证 runner 无 EQ 覆盖形态且参数化扩展超出最小范围 → 停止上交（processorattestation 已定义 FamilyStaticEQ+upsert/bell+eq_regression_receipt，预期无需大改）；**broadband 形态决策点上交**：若 mac 117 个 dynamics 主体中确无 ch1≠ch2 双阈值形态，留空上交并由决策侧裁定"找主体 vs 演进 v5 schema"（不硬凑）
- **领取注记（2026-09-23 决策侧）**：领取提交应一次含完整 mv（todo 删除+doing 新增同批——REALSTEMS 领取提交教训）；`build_whitelist.py` 在 `~/Documents/vit-wl1-artifacts/` 工件目录复用（入仓产品化是独立 backlog，本卡不做）
- 领取：
- 回执：
- 验收：
