# PORT-PCA-FULL-CANDIDATES-PC-1：PC 白名单 v6 全候选派生——24 认证主体 → 七族多候选（镜像 mac 卡）

- 优先级 / 预估 / 依赖：P1 / 0.3-0.5 天 / **PORT-PCA-FULL-CANDIDATES-1（mac）的构建器入仓交付**（本卡步 1 盘点可先行）；FIX-BROADBAND-SHARED-1 合入后 broadband 条目才可入列
- 模型分级：L1 / GLM-5.3 flash 可接（数据派生+驱动+记账）
- **执行侧（PC 侧卡）**：白名单/工件机器本地（`C:\Users\timoz\.vit\`）；回执写回 coord
- **背景（用户 2026-09-23 裁定"PC 全单候选也肯定不对"）**：PC 白名单仍 v5 全单候选（bx_hybrid/Vertigo/Pro-DS/SPL/Pro-L 2/Pro-G/Lindell MBC）= 零可自选族。但 **PC 认证库存 24 个 pca_job**（C2-PCR 12 族 24 体晋升，已核含 Waves 成员：rdeesser/l2_stereo 等壳内主体）——派生原材料齐备，此前每族只取一是策展遗留，与 mac 同病。
- 目标（派生规则与 mac 卡冻结版一致：候选=PCA promoted ∧ 认证覆盖含该族实验轴 ∧ 参数锚点四源可诚实派生；不可派生者如实不入列记因；零手编）：
  1. **盘点先行**（不依赖构建器）：解析 24 个 pca_job 的 summary/evidence——主体身份（identifier/路径/厂商）× 认证覆盖轴 × 族归属表；对照七族实验轴得出 PC 可派生候选清单（预期 EQ≥2：bx_hybrid+其他；各族实际以盘点为准，含 Waves 成员按 identifier 独占解析纪律 PLUGIDENT）
  2. **构建器派生**：mac 卡入仓的白名单构建器（v6 全族形态）以 PC 认证库+语义索引为源跑出 PC v6 白名单（**v5 备份同 mac 先例**）；参数锚点缺口主体补跑 probe（先例 0.7-1.1s/主体）。**【2026-09-23 决策侧合并裁定】FIX-PCA-AUTOSWEEP-1 落地后本步与步 3 的派生部分由 sweep 一步取代（全库扫=盘点+认证+派生）；本卡保留步 1 盘点作为 AUTOSWEEP 分类器规则的 PC 校准输入，若 AUTOSWEEP 先行合入则本卡收窄为"盘点校准+PC sweep 首跑+验证腿"**
  3. **验证腿**：PC 真栈 ≥1 个多候选族自选实证（证据链五环口径：披露→自选→membership→绑定→写链；journey1 或 ④ spot 场景按模型动机化触达记录）；Mono/Stereo 变体按 mac 卡决策点 B 裁定全入列
  4. 回执：两端 HEAD+PC 白名单版本/哈希+候选计数表+溯源指针
- 验收：①盘点表（24 主体↔族↔覆盖轴全可回指）②v6 白名单+溯源+排除项记因 ③overlay 全族回归+非成员 spot ④自选腿证据链 ⑤回执齐备
- 停止条件：某族认证覆盖与实验轴大面积不对口（候选=1 的族如实保持单候选并记因——机制上单候选合法，只是无选择面）；构建器在 PC 路径/编码不适配 → 上修补构建器（域内小改列明）
- 领取：2026-09-23 步 1 盘点先行领取（HEAD `2b5b1513`，领取时工作树已有改动=default_project.xml+他卡工件，与本卡无重叠；零代码改动）
- 回执：**步 1 完成**（步 2-4 待 AUTOSWEEP 合并后收窄执行）。工件 `coord/runs/PORT-PCA-FULL-CANDIDATES-PC-1/`（step1_pc_pca_inventory.md 报告 + .json 机读 38 主体全表 + step1_parse.py 可重跑）。要点：24 job 全 pass=12 插件族系×M/S 全 Waves 同壳；晋升真源=attestation store 36 主体全 promoted（22/24 认证主体在册，**C1 comp M/S 认证 pass 未晋升**=compressor 族无 store 消费方，证据实质完整仅收据元数据字段缺失）；严格可派生候选 **27**（de_esser 6/limiter 6/gate 3/transient 2/multiband 10/static_eq 0/broadband 0），歧义 4+决策 2 项上交：①EQ 预期落空（PC 库 0 static_eq 主体，bx_hybrid/Vertigo 非 promoted——legacy 保留待裁）②compressor→broadband 映射+C1 comp 补晋升（+2 候选，threshold=7 锚现成）③attack 轴边界（TransX/SPL，transient 2→5）④Pro-DS 轴判定（de_esser 6→7）；AUTOSWEEP 校准输入=轴命名双层映射+族内角色正例+记因排除模式（族对轴未覆盖三类）+语义桶计数（eq146/dynamics187/unknown356）
- 验收：**步 1 pass（2026-09-23 PC 决策会话；步 2-4 按合并裁定等 AUTOSWEEP，卡留 todo 待续）**——盘点采信：24 job 全 pass、attestation store 36 promoted 真源口径成立、严格可派生 27（de_esser 6/limiter 6/gate 3/transient 2/multiband 10/static_eq 0/broadband 0）、AUTOSWEEP 校准四件套齐备。**四上交裁定**：①EQ 预期落空+legacy（bx_hybrid/Vertigo 非 promoted）=**legacy 条目保留继续可用**（v5 兼容阀保留、真栈旅程已验证），认证补齐并入 AUTOSWEEP sweep 正常流程；②C1 comp M/S 补晋升+compressor→broadband 映射=**批准**（认证证据实质完整仅收据元数据缺字段=数据完整性修复；+2 候选恰逢 BROADBAND-SHARED 已合入解锁，随 sweep/收窄执行一并做）；③attack 轴边界（TransX/SPL 2→5）=**不预放宽**——按 AUTOSWEEP 冻结规则由分类器跑数裁决落 sweep_report；④Pro-DS 轴判定=同①（legacy 保留+sweep 补齐）。另：mac 侧 e387e231 修复后 PC 两笔提交内容完整性已核对（diff 零丢失，仅 PROTOCOL §9 地址修正为预期保留）。
