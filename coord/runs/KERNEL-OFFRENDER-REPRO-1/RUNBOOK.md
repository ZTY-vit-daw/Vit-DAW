# KERNEL-OFFRENDER-REPRO-1 (301) §8 跑前口径（执行前成文，2026-11-07）

297 取证转卡：守卫留痕插桩（零栈已完成）+M0 复现基线+M2 rack 二分+M3 插件特异性。
全部腿单 run 无重跑；本文件在任一栈腿启动前定稿。

## 0. 被测二进制与源面（三腿共用）

- **被测内核=插桩构建**：`D:/Vit_DAW_wt_301repro/VitApp/build_release/VitApp_artefacts/Release/VitApp.exe`
  SHA256 `cc9fdacc20a544e9e0f884409b9fd330f4b51bc4d71fd20b352e845ecfa6d886`（2026-11-07 22:51:57 构建，31,798,784 B）。
- 构建命令：`cmake -S D:/Vit_DAW_wt_301repro/VitApp -B D:/Vit_DAW_wt_301repro/VitApp/build_release -G "Visual Studio 18 2026" -DVIT_TRACKTION_ENGINE_DIR=D:/Vit_DAW/tracktion_engine` + `cmake --build …--config Release --parallel`，exit 0。
- **源面唯一差异=引擎守卫插桩**：VitApp 源在 wt_228 基线（e10d5a6d）与 origin/main（9180ff89）之间零 diff（亲证 `git diff --stat e10d5a6d origin/main -- VitApp/` 为空）；引擎树 D:/Vit_DAW/tracktion_engine @ b4397498 + 25 行一次性留痕（`modules/tracktion_engine/plugins/external/tracktion_ExternalPlugin.cpp:1336-1367`，两守卫 ×（live/render 相位）各 std::atomic 一次性 exchange，TRACKTION_LOG 出口=juce::Logger→ZmqLogger→`VitApp/Workspace/Logs/VitHeadlessServer_*.log`；**只加日志不改行为**，grep 串 `VIT301_GUARD`）。
- wt_228 既有内核 `F9E3DA0A…ED0D67A`（mtime 2026-10-30 22:23 未动）只读保留不覆盖；新二进制在新目录（本 worktree build_release）。
- 被测 agent=wt_301repro 现场构建（-RestartAgent；Go 树=origin/main 零改动，agent 面非本卡判定面）。
- 内核日志落点：内核 cwd=exe 目录→climbToVitAppRoot→`D:/Vit_DAW_wt_301repro/VitApp/Workspace/Logs/`（Main.cpp initialiseLogger 亲读）。

## 1. M0 复现基线（真栈，含 chat）

- 场景：seg3a_r2 逐字复刻——`-StartKernel -RestartAgent -ArrSrcLoadSmoke -ArrSrcLoadFullAccess`（驱动器=277 分支 verbatim 移植；话术/fixture/断言与 seg3a_r2 相同）；**唯一变量=内核换插桩构建**。
- 命令：`powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_301repro/scripts/dev_agent_smoke.ps1 -RepoRoot D:\Vit_DAW_wt_301repro -StartKernel -RestartAgent -KernelExe D:\Vit_DAW_wt_301repro\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe -ArrSrcLoadSmoke -ArrSrcLoadFullAccess -RunArtifactsDir D:\Vit_DAW_wt_301repro\coord\runs\KERNEL-OFFRENDER-REPRO-1\m0_20261107_<hhmm>\`
- **判定不在 exit 码而在守卫留痕**（预期 py 大概率仍 exit 4 @render=复现静音）：
  - 守卫行 `VIT301_GUARD … rendering=1` 出现于渲染窗 → **候选②证实（物化缺陷定性成立）**；
  - 无守卫行且渲染仍全零 → 候选②被推翻，回并陈（297 结论③维持）；
  - 渲染非静音（py exit 0）→ seg3a_r2 缺陷不可复现（时序/环境敏感），如实记录。
- 跑后采集：内核 `VitHeadlessServer_*.log` 拷贝入 run 目录+VIT301_GUARD 行全文提取；wav 零字节独立核验（python wave）。
- **预算**：chat 一回合+默认 strip-silence 子腿，deepseek-flash，上界 ≈¥1-3（277 两腿实测 ≪¥0.1 量级）。
- 失败分类：py 0=PASS（渲染非静音）/2=环境（含 VSTi 库存空、栈故障）/3=llm_behavior_miss 如实记录/4=装载在场仍静音（预写主分支）；PS1 exit 1=py 4 预期映射。环境中断（端口/进程/传输）只有原始日志在卷才可排除出有效轮次。

## 2. M2 rack 二分腿（真栈，零 LLM）

- 场景：不经 rack、不经 chat——内核 ZMQ `instantiate_plugin`（plugin_identifier 精确匹配 knownPluginList）直挂轨 pluginList → 16 音 clip（与 seg3a_r2 逐字节同形）→ start_render → 计量+守卫留痕采集。插件=Grand Rhapsody（与 seg3a_r2 同件）。
- 命令：`…dev_agent_smoke.ps1 -RepoRoot D:\Vit_DAW_wt_301repro -StartKernel -RestartAgent -KernelExe <插桩内核> -KernelOffrenderReproSmoke -OffrenderReproPlugin grand_rhapsody -NoChatSmoke -NoStripSilenceSmoke -RunArtifactsDir …\m2_direct_grand_rhapsody_<hhmm>\`
- 判定（exit 0=链完成，静音与否均为观测）：
  - **非静音** → 缺陷在 rack 结构渲染路径（候选①/③ rack 子面复活）；
  - **仍静音** → 非 rack 特有（候选②或共有面）；守卫行 rendering=1 → 候选②在直挂路径亦成立。
- **预算 0 LLM**（-NoChatSmoke -NoStripSilenceSmoke 全关；telemetry 零行入 summary 为证）。

## 3. M3 插件特异性腿（真栈，零 LLM）

- 场景：M2 同构（直挂+16 音 clip+渲染）换 **Surge XT**（297 §3 预检清单在卷；298 卡已证 Surge XT 装载/写面健康）。
- 命令：同 M2 但 `-OffrenderReproPlugin surge_xt -RunArtifactsDir …\m3_direct_surge_xt_<hhmm>\`
- 判定：非静音 → Grand Rhapsody/WaveShell 在 non-realtime 大块下的插件特异行为（兼容性缺陷定性）；仍静音 → VSTi 通用路径缺陷。
- **预算 0 LLM**（同 M2）。

## 4. 栈协议与止损

- 候栈排队：307 S1A 占用中（origin/main 9180ff89 登记 22:22）；每腿登记前 fetch 重读资源记录+净空亲核（六口 TCP 5555/5556/5557/7878/8787/18787 listen=0+UDP 4444/4445=0+VitApp/VitAgent/vsphub/Godot/python 进程=0）→登记推 main→≤60s 复核→启动；finally 拆栈 TEARDOWN_ALL_ZERO 亲核+释放登记。
- 三腿同栈串行（每腿独立占用/释放登记，各自新工件目录）。
- 单 run 无重跑；**同型断言失败两轮→止损取证**（跨腿同型亦计入）；插桩构建已 exit 0（构建失败分支未触发）；M0 守卫未命中且 M2/M3 无差异→并陈上交。
- 引擎树插桩变更保留在位（D:/Vit_DAW/tracktion_engine 未提交 diff，+25 行），diff 随卡面回执入卷，处置（landing/回退）交主管。
