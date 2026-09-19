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
- 领取：2026-09-19 21:03 / origin/main=c53b920290e61c07aeefe9207eb5478769b10c65 / 分支 port/fe-l3ready-1（前端仓 ~/Documents/vit-daw-frontend，基于 08ecdb3）；主仓领取时 status：` M VitApp/Workspace/Settings.xml`、` M VitApp/Workspace/default_project.xml`（已定性为此前 headless 冒测运行时状态，非实现改动，未触碰）
- 回执（2026-09-19 21:50 自验完成，入 done 待验收）：
  - **定性结论：跨平台潜在缺陷**（非 mac 特有、非时序差异），三定性证据齐：
    ① **非 mac 特有**：`app/kernel/autoloads/telemetry_manager.gd` 自 PC 运输基线 fa1d74b 起字节级零改动（`git diff fa1d74b 08ecdb3 -- <file>` 为空；mac 侧提交 58c6a3e/08ecdb3 全在 CEF/dylib/告警面）——重置点在 PC 同版代码中同样在场；
    ② **非时序差异**：探针 `scripts/l2_realtime_observation_probe.gd` 在单帧同步调用栈内完成 begin_request→levels 更新→ready dump（无 await/帧边界）；写盘+合并唯一入口为 `_exit_tree`（telemetry_manager.gd:186）与 `_process`（253/395），且快照启动无加载（初始默认字典 122-142）→ dump 前合并在任何平台都不可能发生，行为平台无关地确定；
    ③ **PC 同版必同红**：mac 基线 fa1d74b = PC 工作态快照（B4A tarball f5c4fdd4，README §3：PC HEAD c7bcd29+未提交态），即 PC 当前前端同版代码；红基线在本机复现（run 20260919-211448）。PC 历史绿跑必早于该前端态——与 SMOKE-MAC-1 已裁定 A/B/C/F 契约腐化同族。**"PC 保留 spectral_tile_derived" 口径修正：PC 当前代码同样不保留；保留行为属 PC 上次绿跑时的旧前端态。**
  - 重置点锚点：`begin_mixboard_feature_request` 无条件 `_pending_mixboard_feature_row("requested")` 覆写 L3 顶层行（修复前 telemetry_manager.gd:842-845）；叠加根因：prepared 行仅存磁盘、启动无加载、合并只在写盘时——请求时点内存必然是 requested。
  - 修复（平台中立，PC 同步受益；1 文件 +27/−8；**前端仓分支 port/fe-l3ready-1，commit 9044d80**，基于 08ecdb3，本地仓决策侧直审）：① begin_request 入口先与持久化快照对账（复用 `_merge_existing_mixboard_feature_request`）；② 新增 `_reset_mixboard_prepared_row`：同 request_id+目标匹配+keep 规则（复用 `_mixboard_should_keep_prepared_feature_row`）时 prepared 行不降级，新请求/目标变更仍正常重置，realtime live 行路径零改动；③ 写盘合并 `_preserve_existing_mixboard_feature_rows`：prepared 历史数组行豁免目标匹配（数组=跨请求/跨目标历史矩阵；live 行仍受目标匹配约束）。
  - **红绿对照**（真栈 `run_observation_v1_acceptance_smoke_mac.sh`，工件根 `~/Documents/vit-fe-l3ready-artifacts/`）：
    红 `observation_v1_accept_mac_20260919-211448`（修复前，前端 08ecdb3）：FAIL "L3 band summary was overwritten during ready phase"，summary status=failed（step 级记录 Godot 探针进程 exit 0、断言面红——与 runB 同粒度形态）；
    绿 `20260919-213141`（修复后 9044d80，`--skip-dad-smokes --skip-product-path`）：headless parse + go 观察回归 6 包 + **L2 realtime 全断言**（ready 相位 band/stereo source=spectral_tile_derived/source_revision=rev_prepared 保留；live 行物化/质量负证据/unknown_live_meter；停止后 deferred+rev_second_material；五类数组保留；终态顶层行跟随第二素材）+ AB result 全绿，**脚本 exit 0**；中间轮 `20260919-213056`（--skip-godot-headless --skip-go-tests 同款）亦绿。
    live 行为面回归无损：live 断言与 runB 全对的同一验证块全过 + AB result（mixboard/MOM/chat 9 测试）绿。
  - 端测覆盖边界（AGENTS §5）：已覆盖=⑦ L2 realtime 相位 headless 真跑（真 Godot 4.6 + 前端仓真工程）红绿对照 + headless parse + go 回归 + AB result；未覆盖=DAD 两步（POSIX shm 读取器缺口，上交 D 域）、product-path（F 域，PS1-SYNC-1 裁定中）、GUI/渲染面、PC 侧实机复验（前端仓本地无远端，决策侧直审 diff；PC 受益性由代码同源+同步栈确定性论证承载）。
  - 运行账目（§8 同款纪律）：L2 realtime 相位 3 轮（1 红 2 绿）；主仓零触碰（工作树仅领取前既存运行态两文件 Settings.xml/default_project.xml）；前端仓既有未跟踪 `VitApp/`（agent 相对路径日志污染，领取时已记录）未触碰。
- 验收：
