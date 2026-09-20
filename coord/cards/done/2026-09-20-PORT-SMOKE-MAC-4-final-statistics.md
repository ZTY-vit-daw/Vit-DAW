# PORT-SMOKE-MAC-4：收官统计复跑（④⑤⑥⑦ 各 N=5，协议终态成功率正式测量）

- 优先级 / 预估 / 依赖：P1 / 0.5-1 天（运行时长为主）/ 前置全满足——SYNC-3 已合入（双跳断言+分支容忍+settle 720 双端对齐）、D/E 已修、bash 修复在 main
- 模型分级：L3 / GLM-5.3（统计口径判定 + 失败轮归类语义）
- 目标：**端测时代收官卡**——在干净赛道（全部已知脚本面缺陷已修）上对四个 LLM 参与件做 **N=5 统计复跑**，产出"协议终态成功率"的正式测量（AGENTS §8 重复成功比例条款首次正式适用）：
  1. `run_mix_single_tick_e2e_mac.sh`（④，双跳断言）×5
  2. `run_vit_product_path_smoke_mac.sh`（⑤，vocal focus 提案面正路径+分支容忍）×5
  3. `run_live_material_observation_smoke_mac.sh`（⑥，per-track ready 新断言 mac 首跑）×5
  4. `run_observation_v1_acceptance_smoke_mac.sh`（⑦，**全量不带 skip**，product-path 步携双跳断言）×5
- **成功口径（预声明，不得倒推）**：每件 **≥4/5** 轮 exit 0（5/5 更佳）；失败轮必须逐轮归类（LLM 合法语义分支失败 / 断言失败 / 崩溃 / 环境中断——环境中断须原始证据）；**<4/5 → 停止取证上交**，不得自行加跑凑数。LLM=flash 口径（现配置不动，key 零入工件）
- 文件域：`scripts/`（**仅当复跑暴露 mac 脚本自身缺陷时**的最小修补，逐处列明；SMOKE_TESTS 若补成功率口径）；`agent/`、`VitApp/`、前端仓零触碰
- 验收标准：①四件 × 5 轮运行账目（run ID/退出码/逐轮归类/成功率）；②每件至少一个绿轮的断言证据（双跳链/提案面/新断言面）；③成功率汇总表（≥4/5 达标判定）；④如成功率达标——**端测时代收官声明**（23 件套件最终记分板：全绿 19 + 已知错配 2 + ②③等既绿件）；⑤工件目录齐备
- 停止条件：任一件 <4/5 → 该件取证上交（其余件继续）；暴露 mac 脚本缺陷 → 修补后该轮作废重跑（修补处列明）；LLM 环境中断按原始证据归类；需改 agent/内核/前端 → 域外上报
- 领取：2026-09-20 21:32 / origin/main `49eb7de`（与本地 HEAD 一致）/ 主工作树直跑（本卡预期零代码改动；若复跑暴露脚本缺陷则切 `port/smoke-mac-4` 分支修补）。领取时工作树已有 diff：`VitApp/Workspace/Settings/Settings.xml`、`VitApp/Workspace/default_project.xml`（运行时状态残留，非本卡产物，不触碰）。领取时 doing/ 残留 SETTLE-1、SYNC-2 两张已验收卡（49eb7de 声称整理但 diff 仅含本卡新增——决策侧遗留，回执上报）
- 回执（**2026-09-20 21:32 领取，当日 00:40 执行完毕**）：
  - **成功率（预声明口径 ≥4/5）**：⑥ **5/5 ✓**、⑦ **4/5 ✓**、④ **2/5 ✗**、⑤ **0/5 ✗（有效轮）**——④⑤ 不达标，**卡面"端测时代收官声明"条件不满足**，取证上交。完整账目/逐轮归类/绿轮断言证据：`~/Documents/vit-smoke-mac4-artifacts/`（`RUN_LEDGER.md` 总账 + `SUCCESS_SUMMARY.md` 汇总 + driver_logs/ 驱动侧逐轮日志 + 各 run 工件）。
  - **红因分布**：④⑤ 共 8 个有效红轮全部涉 LLM 语义分支（诚实终止族：能力边界声明×2（与 MAC-2 G 补轮逐字同源）、「已完成 ccb_observation_request」×2、goal=failed 无可采纳决策×2、vocal focus limit_reached×1、clarify ask done+完整 EQ 提案文本未停提案面×1）；另有复合红 1（⑤ R5：合法 static_eq 提案 × `~/.vit/free_state_experiment_plugins.json` 机器白名单未配置 → hop-2 `d1_execution_blocked`，**上交裁定点**：白名单为 pluginprobe 实测 v5 机器本地配置，执行侧不擅自编造）。环境中断 2 例均带原始证据按 §8 排除+补（⑤ R1 LLM 服务 POST 240s 超时；⑦ R4 kernel CoreAudio 设备管理器初始化卡顿）。崩溃 0、脚本断言失败红轮 0。
  - **绿轮断言证据**：④ R3/R5 双跳链全绿（提案面 `needs_confirmation` → 工具面 `improvement_proposal_native_tool_confirmation_required`+workflow=mix_tick → `d1_post_action_evaluation_required`+mutation_performed+readback_verified+「已应用并回读验证」）；⑦ R1/R2/R3/R5 全链绿（六确定性步+product-path 步）；⑥ 5/5。
  - **交付（分支 `port/smoke-mac-4` 已推）**：两处卡面授权内的 mac 脚本最小修补（逐处列明于 commit 与 RUN_LEDGER）：① ⑥ bridge 行 ready 溯源断言按 provenance 分族收窄 @8650a22（DAD L3 离线分析行=内容指纹族无 request_id，run1 红证驱动；**ps1 同款错配未动，待决策侧双端同步**）；② ⑦ step7 前显式停 DAD/L2 kernel @ce495c9（内嵌 ⑤ 撞 5555 单属主规则，五连红证驱动，MAC-2 时因 kernel 偶发先死未暴露）。SMOKE_TESTS.md ④⑤⑥⑦ 四条目补统计口径与修补说明。`agent/`、`VitApp/`、前端仓零触碰。
  - **引擎口径处理**：卡面 "LLM=flash" = `deepseek-v4-flash`（MAC-2 全部 run_meta + 绿轮遥测实证）。执行时发现 `~/.vit/config.json` defaultModel 已被改为 `deepseek-v4-pro`（mtime 09-20 11:51，非本会话所改）——**config 文件不动**，每轮以 `VIT_AGENT_LLM_MODEL=deepseek-v4-flash` env 恢复口径；各 run 遥测 `model` 字段实证生效引擎全 flash；账目错位点（run_meta.llm_config.model 读文件优先会记 pro）已在 RUN_LEDGER 显式声明。
  - **§8 账目**：总执行 23 轮（⑥ 6 跑 1 作废、④ 5、⑤ 6 跑 1 环境排除、⑦ 11 跑 5 作废+1 环境排除）；有效轮 20（4 件×5）；失败归类全覆盖无未分类轮；全部红因可回指工件。
  - **上交裁定点（3 项）**：① ④⑤ 协议终态成功率不达标的处置（flash 提案面停车率 40%/0%——断言面可满足、失败全在模型诚实终止分支；选项：容忍口径修订/引擎升级实验/工作流强化，决策侧定）；② ⑤ R5 static_eq 实验白名单是否配置（需 pluginprobe 实测）；③ doing/ 滞留 SETTLE-1、SYNC-2 两张已验收卡（49eb7de 声称整理但 diff 未含移动——决策侧遗留）；另 ps1 live_material bridge 断言同款错配待双端同步。
  - **端测覆盖边界**：已覆盖=真实两件套 ×flash 真实 LLM 轮 × 20 有效轮统计 × 双跳链绿证据 × DAD shm/L2/⑦ 全链；未覆盖=GUI/渲染面、vsphub 三件套（本轮未涉）。
- 验收：pass（2026-09-20，ruling [2026-09-20-SMOKEMAC4-pass.md](../../rulings/2026-09-20-SMOKEMAC4-pass.md)：统计交付面合格——⑥ 5/5 ⑦ 4/5 达标、④ 2/5 ⑤ 0/5 诚实上报；红轮全部诚实终止族+断言链绿轮证明链路通；两处脚本修补采认（三 commit 已 cherry-pick）；上交①④⑤处置=用户裁定（推荐 b 对照实验）；②白名单开迷你卡；③滞留卡已归位；**决策侧认账：config"已切回 flash"报告失实，现已真实恢复**；端测收官声明暂缓待裁定）
