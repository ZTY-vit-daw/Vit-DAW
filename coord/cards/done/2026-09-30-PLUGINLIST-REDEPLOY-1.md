# PLUGINLIST-REDEPLOY-1：FIX-PLUGINLIST-WINPATH-BLINDSPOT-1 收口腿②——重建含修复演示二进制+leg D 复验+替换演示路径（mac 侧）

- 池序 18；来源=[done 卡待办腿②](../done/2026-09-28-FIX-PLUGINLIST-WINPATH-BLINDSPOT-1.md)验收注记（腿① PC 回归已于 2026-09-30 pass；中期检查已过、演示路径锁定 5fb4585b 已解除）——**本腿落地后 FIX-PLUGINLIST-WINPATH-BLINDSPOT-1 终裁**
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / 无；**目标端=Mac 执行侧**（演示面与 leg 脚本均为 mac 形态：kernel_pluginlist_hygiene_smoke_mac.sh）
- 模型分级：L1 / mac 执行会话可接（配方与验收路径已在案）
- 已核实事实（done 卡与 leg D run hygiene_mac_20260928-112210 在案）：
  - 修复在 origin/port/fix-pluginlist-absolutepath @468ad0e4（未合 main，终裁后决策侧 cherry-pick）；PC 回归腿已 pass（ctest exit 0，coord/runs/.../decision-pc-rerun-20260930/）
  - 现演示二进制=5fb4585b（不含修复）；leg1 配方=仓外 Debug 树构建；leg D 断言全绿口径=HYGIENE_MAC_VERDICT all_green + D_full_scan completed + D_rack_add ok
- 目标：
  1. 按 leg1 配方在仓外 Debug 树**重建演示二进制（基于 468ad0e4，含修复）**；
  2. 用新二进制跑 leg D（kernel_pluginlist_hygiene_smoke_mac.sh --leg D）——**all_green exit 0**（对照上次环境腿所用 5fb4585b 不含修复的边界，这次必须含修复）；
  3. all_green 后替换演示路径（原 5fb4585b 备份留档+sha256 记录）；若 leg D 失败：**不替换**，保留现场取证上交。
  4. 回执后由决策侧对 FIX-PLUGINLIST-WINPATH-BLINDSPOT-1 出终裁+实现 cherry-pick 合 main。
- 文件域：仓外构建树+演示二进制路径+coord/runs/FIX-PLUGINLIST-WINPATH-BLINDSPOT-1/（回执工件）；**零仓库源码改动**。
- 验收标准：leg D run all_green exit 0+演示路径替换回执（新旧 sha256）+回执工件入 runs/。
- 停止条件：新二进制 leg D 失败 → 保留双二进制与 run 工件上交（修复可能未覆盖演示形态，转取证）。
- 领取：2026-10-02（mac 执行侧）/ origin/main=`08fdfe9bc8d59040b5f577f16f2c4060a685eb39`（含决策侧预核锚点所述 cb194c5 之后两条他卡领取提交）/ 分支=main（coord-only，零仓库源码改动卡）；独立 worktree `/tmp/pluginlist-redeploy-wt` 开工（PROTOCOL §3），主工作树不动（停 port/rlm-profile-2 有未提交内容）
- 回执：2026-10-02 执行完成（mac 执行侧，零仓库源码改动）。工件 `coord/runs/FIX-PLUGINLIST-WINPATH-BLINDSPOT-1/redeploy-mac-20261002/`（README+binary_sha256+leg D run_meta/prereq/report）。
  - **重建 ✓**：独立 detached worktree @468ad0e4（子模块 b43974980c 重初始化）+ 仓外全新 Debug build 树，旅程同款 cmake 配方（Unix Makefiles+`VIT_TRACKTION_ENGINE_DIR`），configure/build 双 exit 0；新件 sha256 `8624fe9e162fe7b65ddb5e1fd55c23621020fa11c7b86d961b56944aadcdbe96`（106539960B）。
  - **leg D ✓（all_green exit 0）**：run `hygiene_mac_20261002-105939`，`--kernel-bin` 指新件（run_meta `kernel_sha256=8624fe9e…` 证含修复，消除 9/28 run 用 5fb4585b 的边界）；`HYGIENE_MAC_VERDICT all_green`（ok=3 red=0），D_full_scan state=completed plugin_count=719、D_rack_add status=ok plugin_id=1040（:1153 knownPluginList 解析路径）。真 Settings 跑前跑后 sha 一致零写入。
  - **替换 ✓**：旧件 5fb4585b 备份入 `~/Documents/vit-pluginlist-redeploy-20261002/`（+sha256）；演示路径装后复核=`8624fe9e…` 逐字节一致；双 sha 见 binary_sha256.txt。
  - 边界注记：live Settings 现 719 条全 mac 暖表（0 条 `C:\` 幽灵），启动清理零对象、无摘要行属预期（源码 :85 removed>0 才输出）；修复清理行为由决策侧两复跑腿（单测 mac/回归 PC）覆盖，本腿证演示形态运行链路。
  - 端测边界声明（AGENTS §5）：本卡为二进制重建+既定 leg D 脚本复验，非 agent 侧代码改动；渲染面/用户旅程不在本卡范围。待决策侧终裁 FIX-PLUGINLIST-WINPATH-BLINDSPOT-1+cherry-pick 468ad0e4 合 main。
