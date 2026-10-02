# PLUGINLIST-REDEPLOY-1 · mac 重建部署腿②回执（2026-10-02）

FIX-PLUGINLIST-WINPATH-BLINDSPOT-1 收口腿②：基于修复 commit 468ad0e4 重建演示二进制 → leg D 复验 all_green → 替换演示路径。**零仓库源码改动**（coord-only）。

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

## 2. leg D 复验（新二进制，含修复）

- 命令：`cd ~/Documents/Vit-DAW && ./scripts/kernel_pluginlist_hygiene_smoke_mac.sh --leg D --kernel-bin <新件>`（主仓根运行驱动真 Settings；脚本版本 origin/main=cb194c5 与 468ad0e4 零 diff 亲核）。
- **run `hygiene_mac_20261002-105939`：`HYGIENE_MAC_VERDICT all_green`（ok=3 red=0），EXIT=0**：
  - `D_full_scan state=completed plugin_count=719 completed_files=2 total_files=2` ✓
  - `D_rack_add status=ok plugin_id=1040`（identifier `VST3-C1 comp Mono-10456661-65e94c5e`，kernel :1153 knownPluginList 解析路径）✓
  - run_meta 内 `kernel_sha256=8624fe9e…`——**与 09-28 环境 run（5fb4585b 不含修复）不同，本次被测件含修复**，卡面"这次必须含修复"口径达成。
- 原始工件：`~/Documents/vit-pcbatch-mac-legs-artifacts/leg2_hygiene/hygiene_mac_20261002-105939/`（run_meta/prereq/report 已拷入本目录）。
- 边界注记：当前 live Settings 为 719 条全 mac 形态暖表（`C:\` 条目 0、黑名单 0），启动清理无可清对象——kernel_D.log 无 `PluginListHygiene: removed` 摘要行属预期（源码 :85 仅 typesRemoved>0 ∨ blacklistRemoved>0 才输出，TIM-KERNEL-HYGIENE-1 每次运行都打 completedAt 时间戳）。修复的清理行为本身已由 decision-mac-rerun-20260929（mac 单测腿）与 decision-pc-rerun-20260930（PC 回归腿）覆盖，本腿证的是演示形态运行链路。
- run_meta `repo_head=1f566a0`（主树当时 HEAD，上下文字段）；被测二进制真实源=上述 468ad0e4 worktree（见 binary_sha256.txt 头注）。

## 3. 演示路径替换（all_green 之后执行）

- 旧件备份：`~/Documents/vit-pluginlist-redeploy-20261002/VitApp.binary.bak-5fb4585b-20261002`（+`.sha256`），sha256 `5fb4585b5fd0cb0cddfce9911e8c788096c607288e8b95d4da5f6422c5dda2eb`，mtime Sep 28 11:22:01（9/28 回滚件原貌，cp -p 保留）。
- 安装：新件拷入 `VitApp/build/VitApp_artefacts/Debug/VitApp`，装后 sha256 复核=`8624fe9e…`（与新件逐字节一致），install mtime 2026-10-02 11:04:50。双 sha 记录见 `binary_sha256.txt`。

## 4. 环境与污染核对

- 跑前环境：无 VitApp 进程、5555/5556/5557 无占用、无 pedal 残留（脚本 prereq 双确认）。
- 真 Settings.xml 跑前跑后 sha256 均 `1e842d89…`（零写入；SIGTERM 1s 干净退出未落盘，9/28 在案既有形态）。
- 主工作树未动（停 port/rlm-profile-2，领取前已有改动原样保留）；本卡全部仓库变更=coord/（卡片状态+本回执）。

## 5. 遗留与移交

- 待决策侧：对 FIX-PLUGINLIST-WINPATH-BLINDSPOT-1 出终裁；468ad0e4 实现 cherry-pick 合 main。
- 构建用 worktree `/tmp/pluginlist-redeploy-src` 与协调 worktree `/tmp/pluginlist-redeploy-wt` 保留待验收后由决策侧清理（PROTOCOL §3）。
- 机器本地工件根：`~/Documents/vit-pluginlist-redeploy-20261002/`（build 树+备份+日志）、`~/Documents/vit-pcbatch-mac-legs-artifacts/leg2_hygiene/hygiene_mac_20261002-105939/`（leg D run）。
