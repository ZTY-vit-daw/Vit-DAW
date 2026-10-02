# PLUGINLIST-REDEPLOY-1 · mac 重建部署腿②回执（2026-10-02）

FIX-PLUGINLIST-WINPATH-BLINDSPOT-1 收口腿②：基于修复 commit 468ad0e4 重建演示二进制 → leg D 复验 all_green → 替换演示路径。**零仓库源码改动**（coord-only）。

> **2026-10-02 11:30 补正**：首次 leg D（run `hygiene_mac_20261002-105939`，`--kernel-bin` 指仓外构建树新件）虽 all_green，但被测内核把工作区回退解析到 CWD 仓根新建 `Workspace/`（可执行目录上溯找不到 `VitApp/`），驱动的不是脚本语义要求的真 `VitApp/Workspace/Settings/Settings.xml`。已取证归档、清理副产物，并在替换完成后用**默认演示路径**复跑 leg D（run `hygiene_mac_20261002-112253`，`cached_count_before=719` 证真暖表加载）——**验收基准以复跑为准**。详见 §2/§2b。

## 1. 重建（leg1 配方，先例 PORT-PCBATCH-MAC-LEGS-1 腿① / decision-mac-rerun-20260929）

- 源：独立 detached worktree `/tmp/pluginlist-redeploy-src` @ `468ad0e4a531006da9291be51a64e1629eae7e1e`（= origin/port/fix-pluginlist-absolutepath HEAD，含本修复；工作树干净，`pluginListCarriesAbsolutePath` 在 PluginListHygiene.cpp:13/30/41/73 亲核在位）。tracktion_engine 子模块重初始化 @ `b43974980c`（与决策侧 09-29 复跑先例同 pin）。
- 配方（旅程同款，journey1_demo_journey_smoke_mac.sh:697-705）：

```bash
cmake -S /tmp/pluginlist-redeploy-src/VitApp -B ~/Documents/vit-pluginlist-redeploy-20261002/build/vitapp \
  -G "Unix Makefiles" -DCMAKE_BUILD_TYPE=Debug \
  -DVIT_TRACKTION_ENGINE_DIR=/tmp/pluginlist-redeploy-src/tracktion_engine
make -C ~/Documents/vit-pluginlist-redeploy-20261002/build/vitapp -j"$(sysctl -n hw.ncpu)" VitApp
```

- 结果：configure exit 0（35s，FetchContent libzmq/cppzmq 正常）；build exit 0（`[100%] Built target VitApp`）。日志：`~/Documents/vit-pluginlist-redeploy-20261002/logs/kernel_{configure,build}.log`。
- 新二进制：sha256 `8624fe9e162fe7b65ddb5e1fd55c23621020fa11c7b86d961b56944aadcdbe96`，size 106539960（与 leg1 件 106538696 同量级）。

## 2. leg D 首跑（run hygiene_mac_20261002-105939）——all_green 但有工作区解析边界，降级为链路证据

- 命令：`cd ~/Documents/Vit-DAW && ./scripts/kernel_pluginlist_hygiene_smoke_mac.sh --leg D --kernel-bin <仓外构建树新件>`。
- 结果：`HYGIENE_MAC_VERDICT all_green`（ok=3 red=0）EXIT=0；D_full_scan completed 719（2/2）、D_rack_add ok plugin_id=1040；run_meta `kernel_sha256=8624fe9e…`（含修复）。
- **边界（补正核心）**：被测件位于仓外 `~/Documents/vit-pluginlist-redeploy-20261002/build/vitapp/…`，内核按可执行目录上溯定位 VitApp/ 失败 → 回退 CWD（=REPO_ROOT）**新建** `~/Documents/Vit-DAW/Workspace/{Settings,Logs,Cache}`。内核日志（run 的 kernel_D.log:8-10）明示 `Settings file: /Users/timozty/Documents/Vit-DAW/Workspace/Settings/Settings.xml`（新建冷表，跑中被扫描填成 719 并于 SIGTERM 落盘 296639B）。故本跑**未驱动真暖表**，与 9/28 基线 run（二进制在主树内、无此副产物）形态不同。断言本身（扫描链+rack_add 解析链）与 Settings 位置无关仍为真，但脚本"live-settings behaviours"语义未满足——验收改以 §2b 复跑为准。
- 副产物处置：意外 `Workspace/` 全部 3 文件（Settings.xml/pedal/内核日志）归档至 `~/Documents/vit-pluginlist-redeploy-20261002/accidental_workspace_evidence/` 后整目录删除（该目录时间戳 10:59-11:04 与本跑精确吻合、无他方写入；删除后主树未跟踪态与领取时快照一致）。**教训（供决策侧发卡参考）：`--kernel-bin` 指仓外件会改变内核工作区解析，hygiene 脚本真 Settings 语义要求被测件位于主树 VitApp/ 内。**

## 2b. leg D 复跑（run hygiene_mac_20261002-112253）——**验收基准**

- 命令：`cd ~/Documents/Vit-DAW && ./scripts/kernel_pluginlist_hygiene_smoke_mac.sh --leg D`（默认 kernel-bin=演示路径，即已安装的 `8624fe9e` 新件；脚本版本 origin/main 与 468ad0e4 零 diff 亲核）。
- **`HYGIENE_MAC_VERDICT all_green`（ok=3 red=0），EXIT=0（真退出码，直跑非管道）**：
  - `D_full_scan state=completed plugin_count=719 completed_files=2 total_files=2`，且启动即 `cached_count_before=719`——**真暖表加载的直接证据**（可执行目录上溯落回真 `VitApp/Workspace`；跑后仓根无新建 Workspace 复核 0）；
  - `D_rack_add status=ok plugin_id=1040`（identifier `VST3-C1 comp Mono-10456661-65e94c5e`，kernel :1153 knownPluginList 解析路径）；
  - run_meta `kernel_sha256=8624fe9e…`——被测件=已部署演示件本体。
- 与 9/28 基线 run 的形态差异：9/28 从冷表起扫全量（约 4.5 分钟）；本次从真 719 暖表起（当前真实现场态），扫描为全量校验完成（2/2 无跳过）。两形态断言口径一致（state=completed plugin_count=719），本次更贴脚本"live-settings behaviours"设计意图。
- 原始工件：`~/Documents/vit-pcbatch-mac-legs-artifacts/leg2_hygiene/hygiene_mac_20261002-112253/`（run_meta/prereq/report 拷入本目录 `hygiene_legD_run2_112253_*`）。
- 边界注记：真 Settings 现为 719 条全 mac 形态暖表（`C:\` 条目 0、黑名单 0），启动清理零对象、无 `PluginListHygiene: removed` 摘要行属预期（源码 :85 仅 typesRemoved>0 ∨ blacklistRemoved>0 才输出）。修复的清理行为已由 decision-mac-rerun-20260929（mac 单测腿）与 decision-pc-rerun-20260930（PC 回归腿）覆盖。
- run_meta `repo_head=1f566a0`（主树当时 HEAD，上下文字段）；被测二进制真实源=468ad0e4 worktree（见 binary_sha256.txt 头注）。

## 3. 演示路径替换（首跑 all_green 之后执行）

- 旧件备份：`~/Documents/vit-pluginlist-redeploy-20261002/VitApp.binary.bak-5fb4585b-20261002`（+`.sha256`），sha256 `5fb4585b5fd0cb0cddfce9911e8c788096c607288e8b95d4da5f6422c5dda2eb`，mtime Sep 28 11:22:01（9/28 回滚件原貌，cp -p 保留）。
- 安装：新件拷入 `VitApp/build/VitApp_artefacts/Debug/VitApp`，装后 sha256 复核=`8624fe9e…`（逐字节一致），install mtime 2026-10-02 11:04:50。双 sha 记录见 `binary_sha256.txt`。复跑（§2b）直接验证的就是该已部署件。

## 4. 环境与污染核对

- 首跑前：无 VitApp 进程、端口无占用、无 pedal（脚本 prereq 双确认）。复跑前：端口一度被并行会话 D1-EQ-READBACK-550A-1 的内核占用（其自建 kernel_build 二进制，11:14-11:22），按 PROTOCOL §9 端口栈单一所有权等待其自然释放（11:22:50）后复跑，未触碰对方进程；期间一次预检尝试（run hygiene_mac_20261002-111558）被脚本 wait_ports_free 正确拦为 RED（环境类，不计功能轮次）。
- 真 Settings.xml：首跑前/复跑前/复跑后 sha256 均 `1e842d89…`（两次 run 均零写入；SIGTERM 1s 干净退出未落盘，9/28 在案既有形态）。
- 仓根无 Workspace 残留（复核 0）；主工作树未动（停 port/rlm-profile-2，领取前已有改动原样保留）；本卡全部仓库变更=coord/（卡片状态+本回执）。

## 5. 遗留与移交

- 待决策侧：对 FIX-PLUGINLIST-WINPATH-BLINDSPOT-1 出终裁（腿① PC 回归 pass + 腿② 本回执）；468ad0e4 实现 cherry-pick 合 main。
- 构建用 worktree `/tmp/pluginlist-redeploy-src` 与协调 worktree `/tmp/pluginlist-redeploy-wt` 保留待验收后由决策侧清理（PROTOCOL §3）。
- 机器本地工件根：`~/Documents/vit-pluginlist-redeploy-20261002/`（build 树+备份+日志+意外工作区证据）、`~/Documents/vit-pcbatch-mac-legs-artifacts/leg2_hygiene/hygiene_mac_2026100{2-105939,2-111558,2-112253}`（三次 leg D run）。
