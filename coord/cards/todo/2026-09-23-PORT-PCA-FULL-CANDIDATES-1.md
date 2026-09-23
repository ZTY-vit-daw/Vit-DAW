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
  4. **决策点 A——PC 已裁定（2026-09-23 PC 决策会话）：允许共享阈值单参数形态**。依据先例链完整：static_eq Q10 ch1==ch2 共享（mac 真栈已跑通）+ de_esser FAM1-S1 单共享 threshold（D2-FAM1-S1 ③ 裁定，端口单条目批写同 revision advance 已支持）。schema 微调（broadband 条目接受单 `threshold_param_id` 或 ch2 可选）由 **PC 侧卡 FIX-BROADBAND-SHARED-1** 承接（已开，todo/）；**mac 执行面不受阻**：按共享形态准备数据即可，若本卡先于 PC 卡完成则 broadband 条目暂缓入列待 PC 卡合入（不阻塞其余六族）
  5. **决策点 B——PC 已裁定（2026-09-23 PC 决策会话）：Mono/Stereo 变体全入列，披露面含通道形态字段**。理由：通道形态是结构性事实，交模型按目标轨推理正是自选设计的本意（意图层身份盲不动，选择层信息越完整选择越有据）；构建器**不加去重规则**——去重=回到策展，违背用户"所有 PCA 批准可用插件都可被选择"裁定。验证腿记录模型对变体的选择分布作观察项
- **2026-09-23 23:3x 决策侧发卡前更新（条件全齐）**：①FIX-BROADBAND-SHARED-1 已验收合入 main=`6eed13f`——**broadband 条目解锁，本卡直接按共享单参形态入列 C1 comp M/S**（PC 步 1 盘点已给出 threshold=7 锚，`coord/runs/PORT-PCA-FULL-CANDIDATES-PC-1/step1_pc_pca_inventory.md` 可参照其轴命名映射与记因模式）；②执行前先从最新 main 重建 `agent/bin/vitagent`（含 broadband 形态 loader，22:09 旧构建不含）；③七族全量一次做完（含 broadband），不再有暂缓项；④PC-1 步 1 的四裁定已吸收为本卡派生规则的补充校准输入。
- 文件域：`scripts/`（构建器入仓）+ 机器本地白名单/工件；零 agent 代码改动预期（机制已族无关）；schema 改动=域外上交
- 验收：①七族 v6 白名单+全量溯源（每条目↔认证收据/探测可回指，排除项记因）②构建器入仓（脚本+用法注释+溯源输出形态）③多族自选腿证据链（≥2 非 EQ 族或如实记录未触达）④overlay 全族回归+非成员 spot ⑤回执两端 HEAD+白名单版本/哈希+候选计数表
- 停止条件：某族参数锚定大面积不可派生（>半数主体）→ 记录上交（可能需认证补测另开卡）；构建器入仓遇 scripts/ 域冲突 → 上交
- 领取：
- 回执：
- 验收：
