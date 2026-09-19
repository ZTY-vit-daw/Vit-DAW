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
- 回执：
- 验收：
