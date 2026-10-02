# Ruling：PLUGINLIST-REDEPLOY-1 — pass；FIX-PLUGINLIST-WINPATH-BLINDSPOT-1 终裁 pass（2026-10-02 决策会话）

## PLUGINLIST-REDEPLOY-1 验收

1. **工件亲读**（coord/runs/FIX-PLUGINLIST-WINPATH-BLINDSPOT-1/redeploy-mac-20261002/ 五件全）：
   - leg D `hygiene_mac_20261002-105939`：`HYGIENE_MAC_VERDICT all_green`（ok=3 red=0，D_full_scan completed 719/D_rack_add ok plugin_id=1040），prereq 双确认+PID/日志在案。
   - **被测件=含修复新件硬证据**：run_meta `kernel_sha256=8624fe9e…`＝构建件＝装后复核件（binary_sha256.txt 三段一致，逐字节级）——与 9/28 环境 run 用 5fb4585b 的边界消除，卡面"这次必须含修复"口径达成。
   - **repo_head=1f566a06 疑点澄清**（README §2 明示）：主树当时 HEAD 上下文字段，被测二进制真实源=detached worktree@468ad0e4（工作树干净+`pluginListCarriesAbsolutePath` PluginListHygiene.cpp:13/30/41/73 亲核在位+构建日志机内留档）——构建源链条采信；**注记**：构建日志未拷入仓内工件（机内路径在 README），后续重建腿建议 build log 一并入 runs/。
   - 污染核对：真 Settings 跑前跑后 sha 一致零写入；prereq 无进程/端口占用；主工作树未动（他卡改动原样）。
   - 边界如实申报：live Settings 现全 mac 暖表（0 条 `C:\` 幽灵），本腿不判别清理行为本身（已由 09-29 mac 单测腿+09-30 PC 回归腿覆盖），证的是演示形态运行链路+含修复件部署。
2. **我方核验（PC 侧可核面）**：468ad0e4 在 origin/port/fix-pluginlist-absolutepath 内容亲读（PluginListHygiene.cpp +29/tests +34-14，与父卡 09-29 代码复核注记一致）；替换收口（旧件备份+sha、新件装后复核）记录完整。
3. **零仓库源码改动核对**：本卡仓库变更=coord/（卡片+回执）。

## FIX-PLUGINLIST-WINPATH-BLINDSPOT-1 终裁

三收口条件全齐：①决策侧代码复核 pass（09-29 注记）+mac 单测腿 pass（decision-mac-rerun-20260929）②PC 回归腿 pass（decision-pc-rerun-20260930，ctest exit 0）③重建部署腿② pass（本裁定）。**终裁 pass**；468ad0e4 由决策侧 cherry-pick 合 main（验收 commit 见卡内）。

## 遗留移交

- Mac 侧两个 worktree（/tmp/pluginlist-redeploy-src、/tmp/pluginlist-redeploy-wt）待清理——Mac 机器侧物理操作，随下一张 Mac 卡（AUTH-RESTORE-LOGSPAM-1）转交提示词附带清理指令。

## 补正追记（2026-10-02 晚，执行侧自查披露后决策侧复核）

- **验收基准改为复跑 run `hygiene_mac_20261002-112253`**：首跑（105939）`--kernel-bin` 指仓外构建树件，内核工作区回退解析到 CWD 仓根新建 `Workspace/`（kernel_D.log:8-10 明示）——断言（扫描链+rack_add）仍真，但脚本 live-settings 语义未满足，降级为链路证据。
- 复跑工件亲读：默认演示路径本体（kernel_bin=主树 VitApp/build/...，sha=8624fe9e）+真 Settings 路径+`cached_count_before=719` 证真暖表加载+`HYGIENE_MAC_VERDICT all_green`（ok=3 red=0）真退出码 0——**复跑直接验证已部署演示件本体，验收证据较原裁定更强**。
- 意外 `Workspace/` 3 文件归档后清理、主树未跟踪态复原；真 Settings 三时点 sha 一致零写入；复跑前端口被并行 D1 会话内核占用按 §9 等待释放（一次预检 RED 为环境类拦截，不计功能轮次）——处置全部合规。
- **裁定不变：pass+终裁 pass 维持**；cherry-pick 468ad0e4（=df5ca286）不受影响。教训入档：hygiene 脚本真 Settings 语义要求被测件位于主树 `VitApp/` 内，`--kernel-bin` 仓外件会改变工作区解析——后续 Mac 真栈腿提示词注意此点。
