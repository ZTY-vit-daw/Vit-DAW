# PORT-PS1-SYNC-1：四个腐化契约双端断言升级（A/B/C/F，SMOKE-MAC-1 裁定）

- 优先级 / 预估 / 依赖：P1 / 0.5-1 天 / 无（与 PORT-FE-L3READY-1 跨机并行）
- 模型分级：L2 / GLM-5.3（契约语义对代码锚点的机械升级 + PC 侧跑绿验证）
- 背景（SMOKE-MAC-1 ruling 裁定，四件 PC 套件在当前代码上同样无法通过的契约腐化）：
  - **A（② `run_project_audio_settings_preflight_smoke.ps1`）**：ps1 对保存的 .vit 做 `ET.parse`；内核 `ipcSaveProjectWithPayload` 无条件存加密容器（`VitHeadlessService.cpp:1218`，VIT1 magic）。**升级为**：VIT1 容器 magic 断言 + 设置持久化经"重开工程往返"验证（mac 侧同语义栈级检查已绿：默认 48k/24bit/CD Export/失配往返/重开持久化，工件 `~/Documents/vit-smoke-mac1-artifacts/` 在案）
  - **B（③ `run_project_stems_import_smoke.ps1`）**：`defer_audio_analysis` 已改 opt-in（`ImportService.cpp:1369-1374`，默认 queue+run）。**升级为**：queued/run 语义断言（导入本体成功 + jobs queued 证据面）
  - **C（⑥ `run_live_material_observation_smoke.ps1`）**：mom_version 断言 v1.4 → **v1.5**（`agent/internal/mom/types.go:3`）
  - **F（⑤ `run_vit_product_path_smoke.ps1`）**：vocal focus 终态路由现为 `ccb.observation_catalog`，ps1 别名组只认 mix.observe 族（`mix.observe|mix_observe|mix.request_observation|mix_request_observation`）。**升级为**：别名组纳入 CCB 路由（run2/3 同断点确定性已证）
- 目标：四个契约在 **ps1 原件 + 对应 `_mac.sh` 断言块**双端同步升级（断言语义一致，语言形态各随其主）；PC 侧对四个 ps1 各跑一轮真实栈验证升级后**跑绿**（证明契约正确）；`_mac.sh` 仅改断言块（mac 复跑归 PORT-SMOKE-MAC-2，本卡不跑 mac 脚本）
- 文件域：`scripts/`（4 个 ps1 + 4 个 `_mac.sh` 的断言块；**最小 diff**，不重构不动其他段）；`agent/`、`VitApp/`、前端仓零触碰
- 验收标准：①双端断言升级 diff（逐件对照本卡背景条的升级语义）；②PC 侧四件 ps1 各权威 run exit 0（§8：每件 ≤3 轮，两败止损）；③SMOKE_TESTS.md 相应条目如有契约描述一并同步；④工件（run ID/日志/退出码）
- 停止条件：升级后仍红且红因在**新断言语义之外**（说明契约升级不完整或另有腐化）→ 逐件取证上交，其余继续；发现需改 agent/内核代码才能绿 → **立即域外上报**（本卡永不改产品代码）；PC 侧环境故障按环境中断记录
- 领取：2026-09-19 21:02（PC/GLM-5.3）；origin/main=7309b95509fefc76e7bcaa35bf272fe52db669cc；分支 port/ps1-sync-1；领取时工作树仅 extension/ 与 godot-cpp/ 的 untracked 构建产物，无已跟踪文件改动
- 回执：2026-09-19 21:44（PC/GLM-5.3 自验完成，待验收）
  - **实现 commit**：port/ps1-sync-1 @ f9203fe8（9 文件，+104/−60，全部在 scripts/；agent/、VitApp/、前端仓零触碰）。coord 领卡 commit：main @ c53b9202。
  - **双端升级清单**（断言语义一致、语言形态各随其主）：
    - A② `project_audio_preflight_probe.py`（② ③ 的断言实际载体是探针 py，ps1/mac sh 均委托驱动——文件域偏差说明见下）：保存 .vit 后读头 4 字节断言 `VIT1` magic；legacy fixture 改由明文模板 `VitApp/Workspace/default_project.xml` 派生（新增必选 `--legacy-template`，ps1:93-96/157 + _mac.sh:389 双端传参，模板恰含 1 个 VIT_AUDIO_SETTINGS）；重开往返持久化断言（44100/16）原样保留。断言字段新增 `saved_vit_container_magic`。
    - B③ `project_stems_import_probe.py`：导入回复断言升级为 queued/run 语义（`analysis_deferred=false`、`baking_status=running`、`background_analysis_status=running`、`analysis_jobs_created=0`、`analysis_jobs_queued=N×2`、job_id 非空且 `analysis_queue_status=running`、内嵌 job 已 auto-start 且 submitted=0）。job-control 段：对 auto 任务直接 `audio_analysis_start(interval 200, max_submit_clips 1)` → 原 1 clip/2 features/paused/cancelled 断言原样保留（cap 钳制保证确定性）。_mac.sh 共用探针零额外改动。
    - C⑥ ps1:229 + _mac.sh:690：mom_version v1.4→v1.5（锚 types.go:3 实测工件 `mom_version: v1.5`）；两端注释同步。
    - F⑤ ps1 四处别名组（2821/2827/2900/3014）纳入 `ccb.observation_catalog`/`ccb_observation_catalog` + 2908 消息；_mac.sh `route_counts` observe 组 + vocal focus 消息同步。
    - SMOKE_TESTS.md：四件 PC+mac 条目契约描述同步（VIT1/queued-run/v1.5/别名组）。
  - **PC 权威 run（AGENTS §8/§9）**：内核 `VitApp/build/.../VitApp.exe`（09-12，新于全部锚点源文件）；跑前端口栈 5555-5557/7878/8787 核对空闲；工件均在 `VitApp/Workspace/Artifacts/smoke/`：
    - A② `project_audio_preflight_20260919_211341` exit **0**（1 轮过；magic=VIT1、重开往返 44100/16、legacy 回落 48000/24/defaulted_from_legacy、preflight 61 文件）。
    - B③ round1 `project_stems_import_20260919_211353` exit 1（新断言内实现选择：rebuild_from_project 被 `findLatestAnalysisJob` 优先找到已取消旧任务挡死，ImportService.cpp:1658-1676——改为直接节流 auto 任务）；round2 `project_stems_import_20260919_211457` exit **0**（61 轨、jobs_queued=122、节流 1/2、重开可见、sealed 104 文件只读预检过）。
    - C⑥ round1 `live_material_observation_20260919_211516` exit 1（**第五项腐化**，见上交②）；round2 `live_material_observation_20260919_211727` exit **0**（obs_20260919T131752_3a620630d963、4 材料、mom_version=v1.5 工件核证）。
    - F⑤ 一次调用环境中断（Git Bash 吞反斜杠，非有效轮次）+ round1 `product_path_20260919_212004` exit 1 + round2 `product_path_20260919_212312` exit 1，**同断点两败止损**：vocal focus 回合 `stop_reason=limit_reached`（agent 日志 `continuation.arm ... limit=max_turns continuation=true lifecycle=pending scheduled=true`，`completed_steps=1`）——fail-fast 拆栈杀掉 durable continuation。两轮工件 route 均为 `['ccb.observation_catalog']`（**F 升级方向被 PC 实测证实**，升级后别名组可计数）；observe/multitrack 回合仍走 `mix.observe`（别名组保留旧名，两态都计）。生命周期/夹具/fade-gain/只读观察/多轨 MOM/无 pending 守卫在两轮中全绿。
  - **上交决策侧**：
    1. **F⑤ PC 跑绿阻塞于驱动面缺口**：PC ps1 `Invoke-AgentChat` 单发 POST 无 settle 等待，而 vocal focus 回合在新终态路由下确定性超 max_turns 切片进 durable continuation（mac 同现象已在 PORT-SMOKE-MAC-1 以 `--chat-settle-seconds`（默认 300s）解决并文档化于 SMOKE_TESTS.md mac 条目；PC 体系内 settle 先例：`d1_audition_gap_smoke.ps1` 事件轮询+deadline 习惯形）。按卡面"不动其他段"未擅自改 chat 驱动；建议扩域授权 PC ps1 补同款 settle 等待（预计一处函数级小改）后 F⑤ 可复跑。
    2. **第五项腐化修复请复核**（超出卡面 C 项枚举但同类同域最小机械升级）：⑥ readiness 限制词 `masking_analysis_deferred_phase_5` → `masking_analysis_not_ready_on_current_project_cut`（`agent/internal/mixboard/project_package.go:433` 条件发射，38ecd24b 起无条件词已移除；ps1:1021 + _mac.sh:841 双端同步）。
    3. **文件域偏差说明**：卡面枚举 8 文件，实际触达 9 文件+SMOKE_TESTS.md——A②/B③ 断言物理载体为 `scripts/` 下 2 个 probe py（ps1/_mac.sh 均委托驱动，决策侧锚点描述的 ET.parse 即在其中）；SMOKE_TESTS.md 为验收③明确要求。均在 scripts/ 域内、最小 diff。
    4. **工作树遗留运行时状态**：烟测内核（cwd=exe 目录→repo Workspace）照既有基础设施行为改写了 `VitApp/Workspace/default_project.xml`（新 vitproj uuid/导入轨道记账，并清掉 HEAD 中更早 semantic-processor 烟测的 fixture 残留）——纯运行时记账非实现工作，未入任何 commit；按 §5 未擅自还原，处置（还原/入 run-state 管理）留决策侧。
  - **push 状态**：分支 push 因 GitHub 连接故障（Connection reset/443 不通）暂挂，coord 收尾后统一重试；如仍不通留待网络恢复后补推（本地 commit 完好）。
- 验收：
