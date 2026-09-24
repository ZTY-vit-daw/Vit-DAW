# PORT-PCA-AUTOSWEEP-MAC-1：mac 首轮全库认证扫描——autosweep 命令级执行 + 白名单再生长 + 验证腿

- 优先级 / 预估 / 依赖：P1（用户"全量可选"裁定在 mac 的延续）/ 0.3-0.5 天 / **FIX-PCA-AUTOSWEEP-1 已合入**（scripts/pca_autosweep.py 在 main，PC 全库 7→53 先行实证；共享工具的 mac 验证=本卡本体）
- 模型分级：L1 / GLM-5.3 flash 可接（命令级执行+记账，工具七相幂等一条命令）
- **执行侧（mac 侧卡）**：产出全机器本地（~/.vit 认证库+白名单+~/Documents/ 工件）；回执写回 coord 记两端 HEAD
- **背景**：PC 首轮已实证 autosweep 全链（分类→批量认证→派生，719 主体三桶+53 新晋升）；mac 现状=白名单 23 条目（FULL-CANDIDATES 卡产物）+认证库 27 主体。**mac 与 PC 的关键差异预期**：EQ 认证通道（phase3_live_smoke 收据）是 EQ-1 在 **mac** 建的——PC 的"EQ 族 68 命中无通道"（FIX-PCA-EQCHANNEL-1 域）在 mac 可能不成立，mac 的 EQ 候选可能显著多于 PC；分类器在 Waves 719 上的 mac 分桶可能与 PC 报告（static_eq 68/broadband 51/limiter 41/transient 34/multiband 33/de_esser 32/gate 25）逐桶一致（同库）——**不一致即工具平台缺陷信号，停下取证**。
- 目标：
  1. **mac 首轮 sweep**：跑 `scripts/pca_autosweep.py`（先读脚本 usage/PC 卡回执的调用口径；预计 719 主体，认证步零 LLM，时长以 PC 首轮为参照），产出 mac 侧 sweep_report（三桶+七族表）
  2. **与 PC 报告对照**：分桶逐族对照（同 WaveShell 库预期一致）；差异桶如实记录并取证（一致性与否都是工具验证证据）
  3. **白名单再生长**：sweep 派生步（或经 scripts/build_whitelist_v6_full.py 复跑）把新认证主体并入 mac 白名单 v6——四源零手编纪律不变，排除项记因；候选面预期 23→显著增长（EQ 族尤其）
  4. **验证腿**：overlay 全族回归+非成员 spot；journey 一轮原命令原素材（披露面将扩大到新候选数——证据链五环照旧）；§8 纪律照旧
- 约束：零仓库代码改动预期（工具已在 main）；白名单/认证库全机器本地；LLM key 零入工件；agent 二进制无需重建（无代码改动；若 sweep 工具要求则按其口径）
- 验收：①mac sweep_report 三桶+七族表（与 PC 对照结论）②新晋升计数+认证收据抽样可回指 ③白名单新版本+溯源（含 EQ 族增长或如实记录）④overlay+journey 验证腿证据链 ⑤回执两端 HEAD（决策侧发卡=07a90b0 后最新 main）
- 停止条件：分桶与 PC 大面积不一致（>1/3 族）→ 停下取证上交（工具平台缺陷嫌疑）；sweep 认证步崩溃/环境中断 → 按 §8 记录；白名单派生需手编才能进 → 不手编，如实少列
- 领取：2026-09-24 16:09 CST / origin/main=8f76d1ce042bbd6c40a7955ff53dfa863ece45a3（=发卡基线，无入站新提交）/ 分支=main（coord-only 直推，PROTOCOL §3）；领取时工作树残留：M Settings.xml + M default_project.xml（两 Workspace 运行时状态，不碰）+ ?? .zcodeignore + ?? VitApp/Workspace/Artifacts/
- 回执：
- 验收：
