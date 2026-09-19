# PORT-SMOKE-MAC-1：产品路径端测组 mac 化（7 件，两件套栈，达"PC 类似实验端测"基线）

- 优先级 / 预估 / 依赖：P1 / 1.5-2 天 / 无（与 PORT-VSPHUB-1 并行；两件套栈即可，不等 hub）
- 模型分级：L2 / GLM-5.3（脚本移植机械面为主 + 个别脚本平台语义判断）
- 目标：SMOKE_TESTS.md 现行套件中 **PC-only 产品路径组 7 件**在 mac 两件套真实栈各 exit 0，mac 等价脚本落 `scripts/`（命名对齐 `_mac.sh` 惯例），SMOKE_TESTS.md 逐件补条目：
  1. `run_ab_result_smoke.ps1` → mac 等价（**健康检查 §6 核心件**，MOM/mixboard/ABResult 全模式——优先做，作为组的验收锚）
  2. `run_project_audio_settings_preflight_smoke.ps1`
  3. `run_project_stems_import_smoke.ps1`
  4. `run_mix_single_tick_e2e.ps1`
  5. `run_vit_product_path_smoke.ps1`
  6. `run_live_material_observation_smoke.ps1`
  7. `run_observation_v1_acceptance_smoke.ps1`
- 移植口径：语义对照 PC 原件（ps1 段→mac 段对照表入回执，JOURNEY-1-MAC 同款）；起栈复用 A5/C2/JOURNEY 既有 mac 模式（真实内核+agent、端口归属、隔离临时目录、AGENTS §10）；机器态差异（如 Windows 专属 fixture 路径）按 JOURNEY 前例做 mac 适配并逐项声明；**不另起测试机制**（AGENTS §5）
- **§8 纪律（每脚本预声明）**：各脚本 ≤3 有效运行；成功=单轮 exit 0 且该脚本断言全过；失败分类分记（环境中断须原始证据）；同一脚本同断点两败止损，先取证/补锚点再跑；LLM 依赖脚本按 JOURNEY-1-MAC 先验模式（config 已就位，key 零入工件）
- 文件域：`scripts/`（新增 ≤7 个 `_mac.sh`）+ `scripts/SMOKE_TESTS.md`（补条目）；不改 ps1 原文；`agent/`、`VitApp/` 零触碰（接口缺口→域外上报）
- 验收标准：①7 件各权威 run exit 0 + 断言证据入工件；②§8 运行账目（逐脚本轮次/分类/止损状态）；③语义对照表；④SMOKE_TESTS.md 7 条目；⑤工件目录（run ID/日志/各段 JSON/双 bin sha256/HEAD 与 dirty）
- 停止条件：某脚本语义与 mac 栈接口现状不兼容 → 单件上交证据（其余件不受阻继续）；同断点两败止损；发现需改 agent/内核 → 域外上报；PC 侧脚本本身已腐化（跑不动/与现实现不符）→ 记录上交裁定
- 领取：2026-09-19 / origin/main e84027945142f063dd478664ca758d5d87f5e67a / 分支 port/smoke-mac-1（独立 worktree ~/Documents/Vit-DAW-smoke-mac1；主工作树另有并行流 PORT-VSPHUB-1 在跑两件套栈占 5555/5556/5557/7878，本卡栈运行窗口待端口空窗）
- 回执（**终版 2026-09-19 20:30——1 绿 6 红全部定性：4 例跨平台探针/断言腐化 + 1 例 mac shm 读取器缺口 + 1 例 mac 前端行为差异 + 1 例驱动侧一行修复待授权补跑**）：
  - **交付（文件域内）**：分支 port/smoke-mac-1 @8fbba03（已推）——7 个 `_mac.sh`（4827 行含 SMOKE_TESTS 增量；ps1↔mac 段级语义对照表内嵌各脚本头部）+ `scripts/SMOKE_TESTS.md` 7 条目。ps1 原文零改动；`agent/`、`VitApp/` 零触碰。工件根 `~/Documents/vit-smoke-mac1-artifacts/`（run_meta 记 HEAD=e840279+dirty；内核复用主仓 9-18 23:47 Debug 构建 sha256=fd0d3d84…（journey 先例，VitApp 自 9-17 未变）；agent 每轮 worktree 新建；LLM key 零入工件（仅 has_api_key/model/host 布尔与名称））。
  - **权威 run 账目（§8，逐件）**：
    - ①ab_result：run1 **exit 0**（mixboard/mom/chat 9 测试全绿，19:14）。**1/3 轮，组验收锚绿**。
    - ②preflight：run1 红——探针 legacy-fixture 步 `ET.parse` 解析保存的 .vit 失败。取证：内核 `ipcSaveProjectWithPayload` 无条件容器化（VitHeadlessService.cpp:1218「A .vit file is always an app-bound encrypted container」；hexdump 首 4 字节 VIT1）。**跨平台探针腐化（PC 同版必败）→ 上交 A**。我栈前 8 步内核往返全 ok（默认 48k/24bit/CD Export/失配往返/重开持久化，工件在案）。
    - ③stems：run0 harness 中止（bash3.2 set -u 空数组，修）；run1 红——`analysis_deferred=false`。取证：`ImportService.cpp:1369-1374` defer_audio_analysis 已改 opt-in（默认 queue+run）。**跨平台探针腐化 → 上交 B**。导入本体 4 轨 4 clip 成功、8 jobs queued（工件在案）。
    - ④mix_single_tick：3/3 轮耗尽。run1 红（observe 轮 limit_reached——mac LLM 单 POST 切片耗尽，durable continuation 异步续跑；JOURNEY 同形态）；run2 红（chat_settle 锚已落定 goal=waiting_confirmation→映射 needs_confirmation ✓，但回复提取器误取 trajectory 生命周期行）；run3 红（提取器候选排序反了——turn.completed 应后置优先）。**模型真实回复含「等你确认后我再执行」（执行 needle 命中，events 证据在案）；驱动侧一行修复已 staged（排序已改正入分支）但预算耗尽，按预declare口径不跑第 4 轮 → 上交 G：授权补跑一轮即可终判**。
    - ⑤product_path：3/3 轮。run1 红（提取器垃圾+import 参数错位（fallback 救回语义）+vocal focus goal 7 轮烧尽 failed）；run2/run3 **同断点两败止损**：vocal focus 到达 ps1 接受终态 needs_confirmation（真实 pending），但路由=`ccb.observation_catalog`，ps1 别名组（mix.observe|mix_observe|mix.request_observation|mix_request_observation）observe=0 → **ps1 路由契约漂移（当前 agent 观察走 CCB 面）→ 上交 F，确定性已证**。前段全绿清单（run2/run3 双证）：栈起停/健康检查/fixture/clip fade-gain 闭环（含 0.15/0.25s 往返）/只读 observe（无 pending 无确认+acoustic bridge readiness+权威 mixboard 快照）/中文多轨 MOM（intent/coverage≥2/do_not_include_raw_package/goal_text）/no-pending guard。
    - ⑥live_material：run1 红——`mom_version=v1.5` vs ps1 断言 v1.4。取证：`agent/internal/mom/types.go:3 const Version = "v1.5"`（ps1 函数名 V13 检 v1.4 的历史漂移模式）。**断言漂移 → 上交 C（决策侧一行升级 ps1/mac 断言即可）**。前段全绿：双素材导入（100hz wav+edm_song.ogg）/immediate acoustic lifecycle v0 断言/artifact 落盘/track-acoustics-ready 重试环收敛（MOM 断言前全部通过）。
    - ⑦acceptance：runA（全量）红 @DAD L3——**预期内 mac 接口缺口证据**：dad_probe.py:54 `mmap(tagname=…)` Windows 专属，mac TypeError 原文入 summary（`shared_memory_read_failed: 'tagname' is an invalid keyword argument`）；同 run 中 `l3_acoustic_summary=ready`（纯 JSON 面）证明内核侧 L3 特征完好、仅 py shm 读取器缺口 → **上交 D（scripts/*.py 不在本卡域；需 POSIX-shm 读取器）**。runB（--skip-dad-smokes）红 @L2 realtime「L3 band summary was overwritten during ready phase」——**live 行全对**（l2_probe_req ready/live_level_meter_spectrum|stereo/realtime_playback/unknown_live_meter/质量负证据/停止后 deferred+rev_second_material/五类 rows 保留），唯 ready 相位种子 L3 顶层行被 mac 前端 VitTelemetryManager 重置为 requested（PC 保留 spectral_tile_derived）→ **上交 E（vit-daw-frontend 仓库外）**。godot headless parse + go 回归 6 包 + ab_result 步骤在 runA/runB 均绿（step 级账目在 summary.json）。
  - **机器态适配（逐项声明，语义不变）**：pyzmq 装 `pip3 --user`（系统 python3 3.9.6）；训练目录默认合成 4 stems（PC E:\ 素材不在机，--training-folder 可覆写）；素材组 100hz+edm_song.ogg（Paper Crown.mp3 PC-only）；工件根 `~/Documents/vit-smoke-mac1-artifacts`（PC 写 repo Workspace，mac 按 §10 隔离）；mixboard/acoustic-status 经 VIT_MIXBOARD_ROOT 平移；busy 重试 5s×12；chat_settle 锚（waiting_continue 续跑等待+events 面读取）；⑤⑦ 起栈为两件套直起（hub 未移植，PORT-VSPHUB-1 并行中；interaction_path 如实改报 agent_http_after_direct_stack_lifecycle，agent 侧仅 blind_project_smoke 键控）；⑦ 子件 dad_probe.py 直驱（ps1 薄包装内联）+ 新增 --skip-dad-smokes 证据粒度旗标。
  - **端测覆盖边界（AGENTS §5）**：已覆盖=真实两件套（内核+agent 真实起停、ZMQ/HTTP 面）×真实 LLM 轮（rightapi.ai/deepseek-v4-flash）×逐段显式断言；未覆盖=GUI/渲染面、vsphub 三件套路径（⑤按卡面适配声明）、稳定性比例（单绿=存在一条成功路径）、DAD shm 面（上交 D）。
  - **上交裁定点汇总**：A=② .vit 加密容器 vs ET.parse；B=③ defer 默认翻转；C=⑥ mom_version 断言漂移；D=⑦ dad_probe POSIX-shm 读取器；E=⑦ mac 前端 ready 相位 L3 行重置；F=⑤ vocal focus CCB 路由别名组；G=④ 预算耗尽+一行修复 staged 待授权补跑。A/B/C/F 为 PC 侧同版必败的契约腐化（PC 套件在当前代码上无法绿），建议决策侧统一裁定 ps1 升级口径。
- 验收：pass（2026-09-19，ruling [2026-09-19-SMOKEMAC1-pass.md](../../rulings/2026-09-19-SMOKEMAC1-pass.md)：交付面完整+组锚 ab_result 绿+§8 诚实（含⑤止损）；六件红无一为移植质量问题；**A-G 七项已裁定**——A/B/C/F=契约腐化成立开 PS1-SYNC-1、D=POSIX 读取器卡、E=前端取证卡、G=**授权④补跑一轮**；8fbba03 已 cherry-pick 639c8af）
