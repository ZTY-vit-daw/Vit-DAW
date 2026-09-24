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
- 回执：（2026-09-24 19:0x CST 自验完成，待决策验收；工件根 ~/Documents/vit-autosweep-mac-artifacts/）
  - **两端 HEAD**：决策侧发卡=8f76d1c；mac 执行侧领取提交 892919e+82555fc（coord-only，main）；
    本回执收口提交=（本次 mv done 提交，见 git log）；仓库零代码改动（工具原样执行）
  - **①mac sweep_report**：run 20260924_161642（16:16-16:47，七相全过）——705 主体
    probe 705/705 零超时；三桶 classified 185 命中/certified 45/exceptions 118 全记因；
    七族表 static_eq 53/broadband 18/limiter 29/transient 29/multiband 23/de_esser 16/gate 17
  - **②PC 对照**（PC_COMPARISON.md）：共享 705 主体（mac 全库=PC 970 子集，PC 独有 265=
    非 Waves 厂商）**逐主体族集合分歧 0/705、七族计数逐桶相等**——卡面 PC 数字（68/51/…）
    为全库口径，增量全来自 PC 独有主体；分类器跨平台确定性验证 pass（非停止条件情形）
  - **③白名单再生长**：23→47 条目（sha 324ee2ac→2407930c），EQ 3→7/broadband 2→13/
    limiter 4→9/multiband 6→8/de_esser 4→6/gate 2→2/transient 2→2；promoted pairs 28→73；
    四源零手编（derive 相=frozen builder 原样复跑+EQ 通道收据派生 +4/排除 0 记因）；
    溯源 sweep_run…/derive/ 全套+backup/ 跑前版哈希
  - **④验证腿**：sweep verify 相 overlay 50 检查零失败（47 成员+3 非成员 spot 拒绝）；
    journey 原命令原素材两轮——run1 exit 1（红点=证据捕获窗口 /agent/ui/state GET 超时+
    占位分支，分类记录），**run2 exit 0 all_green red=0**；五环证据链
    （journey/run2_evidence_chain.json）：披露 45/45 七族零泄漏→自选 Q10 Stereo→零拒绝→
    FrozenPlan 绑定→set_plugin_param succeeded（D2-1 有界 EQ 带调整，applied_revision 4）
    +确定性探针 C1 comp Mono instance_ready+audition 卡挂载；LLM key 零入工件（实值+模式双扫 PASS）
  - **例外/上交项**：static_eq load_gate 47 例与 PC 57 例同机制（server.go:1456 缺省
    source 补 "http"→gate 生效→无 admission 主体循环依赖，FIX-PCA-CERTAUTH-TOKEN-1 域）；
    自举不动点核验：47 例零主体在任一族有晋升，幂等重跑无可吸收——token 入口解锁后
    由决策侧处置；runner 判定不支持 60 例（detector 边界记因，bucket_exceptions 可查）
  - **环境事件**：会话 Bash 工具一度 "spawn /bin/zsh ENOENT"（持久 CWD 指向已迁移的
    run 目录所致，重建路径+cd 后恢复，全程未影响任何运行结果）；无栈级环境中断
  - 端测边界声明：本卡验证=agent HTTP 面+确定性认证通道+journey 脚本断言（含真实栈
    kernel+agent+真实 LLM 轮）；不覆盖 webui 渲染面与 Godot 前端（卡域外）
- 验收：**pass（2026-09-24 Mac 决策会话，[rulings/2026-09-24-PORT-PCA-AUTOSWEEP-MAC-1-pass.md](../../rulings/2026-09-24-PORT-PCA-AUTOSWEEP-MAC-1-pass.md)）**——决策侧独立核证：白名单 47 条目七族计数逐项吻合（sha 2407930c 实核）；**跨平台确定性 0/705 分歧+逐桶相等**（共享集口径化解卡面数字差异=全库口径，分类器平台验证 pass）；journey 两轮分账核（run1 红=回执如实记录的证据窗口超时轮、run2 all_green+五环+零拒绝）；overlay 50 零失败；零仓库代码改动+§8 合规+key 双扫 PASS。EQ 3→7/broadband 2→13；load_gate 47 例自举不动点核验采信归 FIX-PCA-CERTAUTH-TOKEN-1
