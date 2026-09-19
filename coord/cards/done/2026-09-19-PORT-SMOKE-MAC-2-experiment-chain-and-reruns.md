# PORT-SMOKE-MAC-2：实验链 py 组 5 件 + 升级后复跑 ②③⑤⑥ + ④授权补跑 + D 读取器 + ⑦ 全绿收口

- 优先级 / 预估 / 依赖：P1 / 1-1.5 天 / 前置全满足——PS1-SYNC-1 已合入（断言已升级）、FE-L3READY-1 已修复（E 闭，前端仓 `port/fe-l3ready-1` @`9044d80` 已检出）；与 PORT-PS1-SETTLE-1（PC）跨机并行；本卡为唯一 Mac 栈使用者
- 模型分级：L3 / GLM-5.3（py 组真跑语义 + 跨断言复跑判定；关键点可找参谋讨论）
- 目标（五段）：
  1. **实验链 py 组 5 件 mac 真跑**：`b1_group_reset_agent_smoke.py`、`b1_2_source_calibration_agent_smoke.py`、`b1_3_full_gain_staging_agent_smoke.py`、`b1_2_a4_multiclip_agent_smoke.py`、`b4_low_end_relation_agent_smoke.py`——两件套栈（A5/C2/JOURNEY 既有模式；pyzmq `pip3 --user` 先例沿用）；SMOKE_TESTS.md 五条目
  2. **②③⑤⑥ 升级后复跑**：PS1-SYNC-1 已升级断言的四个 `_mac.sh`（preflight=VIT1 magic+重开往返 / stems=queued-run / product_path=CCB 路由别名组 / live_material=mom v1.5）各复跑至绿（mac 脚本自带 `--chat-settle-seconds`，⑤无 PC settle 缺口）
  3. **④ G 授权补跑**（ruling 2026-09-19-SMOKEMAC1-pass G 项）：`run_mix_single_tick_e2e_mac.sh` 补跑**一轮**（提取器排序修复已在 main `639c8af`；needle 命中证据在案）——**仅此一轮授权**，红则取证上交不再加跑
  4. **D：dad_probe.py POSIX shm 读取器**：`mmap(tagname=…)` Windows 专属 → 增 POSIX 分支（`shm_open` 等价；对齐 A2/harness `shm_darwin` 命名规约——段名前缀语义以内核 SharedMemorySegmentPosix 为准），Windows 路径逐字不动；单文件最小 diff
  5. **⑦ 全绿收口**：D+E 双修后 `run_observation_v1_acceptance_smoke_mac.sh` 全量（不带 skip 旗标）复跑——DAD 两步 + L2 realtime + 全断言 exit 0
- §8 纪律（逐件预声明）：各件 ≤3 有效轮（④仅 1 轮授权）；同断点两败止损取证；LLM 依赖按 JOURNEY 先验（key 零入工件）；环境中断须原始证据
- 文件域：`scripts/`（dad_probe.py POSIX 分支 + 复跑所需的最小脚本修补 + SMOKE_TESTS.md 条目）；前端仓零触碰（`port/fe-l3ready-1` 检出态直接用）；`agent/`、`VitApp/` 零触碰（接口缺口域外上报）
- 验收标准：①py 组 5 件各权威 run exit 0 + SMOKE_TESTS 条目；②②③⑤⑥ 复跑 exit 0；③④ 补跑结果（绿=收口/红=取证上交）；④dad_probe POSIX 分支 diff（Windows 路径逐字不动）+ mac 侧 DAD 两步过；⑤⑦ 全量 exit 0；⑥§8 运行账目 + 工件目录（run ID/日志/各段 JSON/HEAD 与 dirty）
- 停止条件：py 组某件语义与 mac 栈不兼容 → 单件上交其余继续；DAD POSIX 读取器实现遇段名/权限语义不明 → 取证上交不猜；⑦ DAD 仍红且红因在读取器之外 → 取证上交；需改 agent/内核/前端 → 域外上报
- 领取：2026-09-19 / origin/main 86eb6661615551ee6756bc69fc67704d62b4a8aa / 分支 port/smoke-mac-2（独立 worktree ~/Documents/Vit-DAW-smoke-mac2；主工作树仅存已入账的 VitApp 两文件烟测遗留改动，非本卡 diff；前端仓 port/fe-l3ready-1 @9044d80 检出态已核实；639c8af 已在 main 已核实）
- 回执（**终版 2026-09-20 01:20——五段全执行：②③复跑绿 + D 真栈双验绿 + ⑦前六步全绿（D/E 双验）+ ①三件绿两件上交；④⑤⑥按 §8 止损/授权口径取证上交**）：
  - **交付**：分支 `port/smoke-mac-2` @`e6c493b`（已推）——`scripts/dad_probe.py` POSIX 读取器（Windows tagname 路径逐字不动）+ 5 个 `_mac.sh` 退出码卫生修复 + ⑥ settle 合成修复 + ⑤ waveform 断言语义修复 + 新增 `run_experiment_chain_py_group_smoke_mac.sh`（① 驱动，py 本体零改动）+ SMOKE_TESTS.md py 组条目。`agent/`、`VitApp/`、前端仓零触碰。工件根 `~/Documents/vit-smoke-mac2-artifacts/`（总账 `RUN_LEDGER.md`：run ID/逐段账目/双 bin sha256/HEAD+dirty；内核复用主仓 9-18 构建 fd0d3d84…；LLM key 零入工件）。
  - **①py 组（5 件）**：01 b1_group_reset **exit 0**、03 b1_3_full **exit 0**（B1.1→B1.2 链式全过）、05 b4_low_end **exit 0**；02/04 上交（见下）。SMOKE_TESTS 条目含首跑定性。
  - **②③⑤⑥ 复跑**：② run2 **exit 0**（VIT1 magic+44100/16 重开往返）；③ run2 **exit 0**（deferred=false/8 jobs queued/running/reopen）。run1 双双"功能绿 exit 143"→根因=trap cleanup 内 `wait` 在 set -e 下 errexit（5 脚本同构修复在案；SMOKE-MAC-1 后②③首次进绿路径才暴露）。⑤⑥ 见上交。
  - **④ G 授权补跑（1 轮）**：红，取证上交——goal=completed、route=[ccb.observation_catalog]、reply=「处理完成 任务已到达明确的能力边界；没有把能力不足解释为改善完成」：**G 的提取器修复证实有效**（本轮取到模型真实终态回复，对照 SMOKE-MAC-1 的 trajectory 垃圾行），红因是模型语义分支（能力边界声明而非确认请求）。
  - **D**：绿——self-test 绿 + **⑦ DAD L3 package smoke / L2 render probe 两步真栈 exit 0**。开发中两个实测锚点：ctypes 直调 shm_open 在 darwin arm64 变参 ABI 下 mode 读垃圾（0600 段 reopen EACCES，C 原生程序对照通过）→ 改 `multiprocessing.shared_memory`；CPython 无条件 prepend `/`（`//VAF_x` 与内核 `/VAF_x` 不同对象，⑦ run1 的 ENOENT 证据+C 对照实验）→ 传裸名。段名规约对齐内核 SharedMemorySegmentPosix（发布名无前缀）与 harness shm_darwin（读取时补斜杠——py 侧经 CPython 行为等价达成）。
  - **⑦**：run2 **前六步全绿**（godot parse/go 回归/**DAD L3（D 验证）**/**L2 render probe（D）**/**L2 realtime（E 修复验证）**/AB result），product-path 步红=⑥同因传导（同脚本同断言面，§8 同断点两败禁原样重跑故不跑 run3）。**卡面"D+E 双修后应达全绿"的差距点不在 D/E（双双验证通过），在 product-path 步内嵌 vocal focus 轮的 LLM 行为**。
  - **上交裁定点（4 项）**：
    - **⑥（两败止损）**：run1 路由别名组断言绿（observe=2，ccb.* 命中——PS1-SYNC-1 升级被证实工作）+ 修 settle 合成缺陷（needs_confirmation 布尔从 runtime/status 的 pending_interaction 派生，mac 切片锚与 ps1 同步响应对齐；修复本身未再获绿轮检验——如实声明）；run2/3 同断点两败：vocal focus 轮 settle 后 goal=failed（`terminal turn produced no admissible final decision after one strengthened retry`，前沿为空、提案未获采纳）——LLM 轮语义失败（run1 同代码曾到 waiting_confirmation，分支随机性在案）。
    - **⑤（三轮用尽）**：kernel-prepared 断言块（ps1 同源 1:1）与 feature_snapshot 快照 schema **错配族**——rows=每 request 历史轨迹行（partial 1/20 + ready 20/20 + missing 并存，SMOKE-MAC-1 与本卡两代工件同形）；"全称 ready"断言已修 per-track ready 并验证推进；第三层错配=DAD L3 行（band_energy_summary 等）无 request_id 键（provenance=source_identity/evidence_ref 族）。**该断言块双端从未执行过**（SMOKE-MAC-1 被 mom_version 红挡前；PC 绿轮走 else 分支）——ps1 同款待双端同步升级，建议整块静态审读后一次修齐。
    - **①-02（代码级确定，禁重跑）**：B1.2 单发轮被容量评估路由到治理 project_mix_workflow，`projectMixWorkflowV1Available` 恒 false（orchestration_controller_host.go:127）→ capability_unavailable 无 pending；03 证明同语义链式 context 走通。py 会话设计 vs 当前 agent 能力面错配，PC 同版必红。
    - **①-04（三轮两形态）**：裸代号消息「进行B1.2」×2/「执行B1」×1 稳定 semantic_entry unresolved（waiting_clarification，confidence 0.3-0.4）；a4 fixture 确定性构造本身成功（2 轨+1 多 clip 轨）。同为 py 会话设计 vs semantic_entry 面错配。
  - **§8 运行账目**：②③各 2 轮（run1=退出码卫生 bug 非功能败）；⑥3 轮（run1=驱动合成缺陷已修）；⑤3 轮（逐层断言错配）；④1 轮（授权上限）；⑦2 轮（run1=D 名字 bug 已修）；①01/03/05 各 1 轮、02 1 轮、04 3 轮。全部红因均有工件证据（RUN_LEDGER.md 逐段回指）；环境中断 0 例。
  - **端测覆盖边界（AGENTS §5）**：已覆盖=真实两件套（内核+agent 真实起停）×真实 LLM 轮×py/脚本逐段显式断言×DAD shm 面（D）×L2 realtime（E，⑦ 内 Godot headless）；未覆盖=GUI/渲染面、vsphub 三件套路径、稳定性比例（绿=存在一条成功路径）。
- 验收：
