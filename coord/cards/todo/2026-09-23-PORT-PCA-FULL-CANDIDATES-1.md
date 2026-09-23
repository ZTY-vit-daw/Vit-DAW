# PORT-PCA-FULL-CANDIDATES-1：候选面全量派生——七族 PCA 晋升主体 → 白名单 v6 全候选（用户裁定 2026-09-23"所有 PCA 批准可用的插件都应该可被选择"）

- 优先级 / 预估 / 依赖：P1（用户设计意图裁定）/ 0.5-1 天 / FIX-PLUGIN-SELECT-1 合入（机制族无关已就位）+ PORT-PLUGIN-SELECT-MAC-1（EQ 三候选已验证自选链路）
- 模型分级：L1 / GLM-5.3 flash 可接（数据派生+驱动+记账，机制零改动预期）
- **执行侧（mac 侧卡，含两个上交 PC 的决策点）**
- **背景（用户 2026-09-23 质询裁定）**：候选宇宙应= PCA 晋升主体全体（凡认证覆盖对口实验轴者），白名单不得做演示性收窄——此前 EQ-only 系决策侧叠账（PLUGIN-SELECT mac 腿锚 EQ+后续降格 P3），非架构限制。白名单层的正当职能=参数锚定绑定，**不是策展**。
- **派生规则（冻结）**：候选 = PCA promoted 主体 ∧ 认证覆盖含该族实验轴（EQ=增益带/de_esser=threshold/transient=attack/limiter=ceiling/gate=range/multiband=band thresholds/broadband=threshold）∧ 参数锚点可四源诚实派生；不可派生者如实不入列+回执记因（EMO-F2 先例）。零手编红线不变。
- 目标：
  1. **七族 v6 化全候选**：本机认证库 27 主体盘点（static_eq 3 已就位/de_esser 6/limiter 4/multiband 6/transient 4/gate 2/broadband 2 待决策点 A）——各族补跑参数锚定探测（probe 先例 0.7-1.1s/主体）+ 白名单构建器全族扩展 + 溯源表全覆盖；其余规则同 PORT-PLUGIN-SELECT-MAC-1
  2. **构建器入仓**：`scripts/` 下新增白名单派生脚本（自 vit-wleq1-artifacts/build_whitelist 系列整理泛化，支持 v6 全族形态+溯源输出）——兑现开源 backlog"白名单构建器未入仓"项；机器本地运行产物仍不入仓
  3. **多族自选验证腿**：≥2 个非 EQ 族的真栈自选实证（如 de_esser 三选一/limiter 二选一——按模型动机化实际触达的族记录，两轮内未触达某族如实记录）；证据链五环口径同 PORT-PLUGIN-SELECT-MAC-1（披露→选择→准入→绑定→写链）
  4. **决策点 A（上交 PC）**：broadband 表单演进——v6 broadband 条目允许共享阈值单参数（对齐 static_eq bands ch1==ch2 先例）使 C1 comp M/S 入列 vs 维持 ch1≠ch2 硬形态维持留空；本卡执行面按"允许共享"准备数据，schema 若需改动=域外上交（属 PC 侧 FIX-PLUGIN-SELECT 后续）
  5. **决策点 B（上交 PC）**：Mono/Stereo 变体是否都作候选——建议入列且披露面含通道形态字段（模型可据目标轨推理），若 PC 裁定只保留主形态则构建器加去重规则；执行面先全入列
- 文件域：`scripts/`（构建器入仓）+ 机器本地白名单/工件；零 agent 代码改动预期（机制已族无关）；schema 改动=域外上交
- 验收：①七族 v6 白名单+全量溯源（每条目↔认证收据/探测可回指，排除项记因）②构建器入仓（脚本+用法注释+溯源输出形态）③多族自选腿证据链（≥2 非 EQ 族或如实记录未触达）④overlay 全族回归+非成员 spot ⑤回执两端 HEAD+白名单版本/哈希+候选计数表
- 停止条件：某族参数锚定大面积不可派生（>半数主体）→ 记录上交（可能需认证补测另开卡）；构建器入仓遇 scripts/ 域冲突 → 上交
- 领取：
- 回执：
- 验收：
