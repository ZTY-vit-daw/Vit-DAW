# PORT-FE-L3READY-1：前端 ready 相位 L3 顶层行重置——取证与修复（SMOKE-MAC-1 上交 E）

- 优先级 / 预估 / 依赖：P1 / 0.5-1 天 / 无（与 PORT-PS1-SYNC-1 跨机并行；本卡为第一波唯一 Mac 栈使用者，独占栈窗口）
- 模型分级：L3 / GLM-5.3（跨端 telemetry 生命周期语义 + 行为差异根因；关键点可找参谋讨论）
- 背景（SMOKE-MAC-1 ⑦ runB 证据）：mac 前端 `VitTelemetryManager` 在 ready 相位把 L3 顶层行**重置为 requested**（PC 保留 `spectral_tile_derived`），触发 L2 realtime 断言 "L3 band summary was overwritten during ready phase" 红；**live 行断言全对**（l2_probe_req ready/live_level_meter_spectrum|stereo/realtime_playback/unknown_live_meter/质量负证据/停止后 deferred+rev_second_material/五类 rows 保留）——差异仅此一处。工件：`~/Documents/vit-smoke-mac1-artifacts/`（⑦ runA/runB summary 与 step 账目）
- 目标：
  1. **取证**：在前端仓定位重置点（VitTelemetryManager 相关 telemetry 链）；对照 PC 行为回答——是 mac 分支特有、时序差异暴露的跨平台潜在缺陷、还是 PC 同代码但时序不触发？三种定性证据齐备
  2. **修复**：按定性选路——跨平台缺陷则平台中立修复（PC 同步受益）；确属 mac 特有时平台分支并说明；修复最小化，不动 live 行为面（runB 已证全对）
  3. **验证**：headless DAW 场景复现 ⑦ 的 L2 realtime 相位（runB 同款探针面）——修复前红/修复后绿的红绿对照；live 行回归无损
  4. 证据=前端仓分支 commit（本地仓，决策侧直审）
- 文件域：前端仓（`~/Documents/vit-daw-frontend` telemetry 相关 .gd）；主仓零触碰；若根因在 agent/内核侧 → 域外上交叉证据停止
- 验收标准：①三种定性证据（重置点代码锚点 + PC 对照结论）；②修复 diff（最小化）；③红绿对照证据（修复前红/后绿）+ live 行回归无损；④前端仓分支与 commit hash
- 停止条件：根因在 agent/内核 → 停止域外上报；修复必须改 PC 行为且无平台分支解 → 上交方案裁定；headless 无法复现该相位 → 取证后上交（转编辑器手测口径裁定）
- 领取：
- 回执：
- 验收：
