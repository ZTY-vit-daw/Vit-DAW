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
- 回执：
- 验收：
