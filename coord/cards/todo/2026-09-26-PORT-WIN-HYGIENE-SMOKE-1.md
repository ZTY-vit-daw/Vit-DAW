# PORT-WIN-HYGIENE-SMOKE-1：Windows HYGIENE 烟测复跑——谓词修正（37ae4f4）后的 PC 真栈验证

- 优先级 / 预估 / 依赖：P2 / 0.3 天 / FIX-KERNEL-HYGIENE-BUNDLE-1 验收转交（rulings/2026-09-26-PORT-CLOSEOUT-LEGS-1-pass.md 遗留①）
- 模型分级：L1 / flash 可接（验证卡，零生产代码改动预期）
- **执行侧（PC 侧卡）**
- 背景：HYGIENE-BUNDLE（谓词 existsAsFile→exists bundle 感知）在 mac 侧已验收 pass（红134/绿0+ctest 6/6+双重启决定性断言=扫描持久化恢复）；实现 37ae4f4 已在 main，**Windows 形态从未复验**。
- 目标：
  1. Windows 构建 VitApp（main 当前 HEAD）+ ctest 跑 `PluginListHygieneTests`（VitApp/Tests/PluginListHygieneTests.cpp，Windows 配置双跑 Debug/Release 按 AUDITION-REL-1 恢复的门）
  2. 真栈双重启决定性断言的 Windows 等价：复用 `scripts/dev_agent_smoke.ps1` 体系（§5 纪律：扩展参数或按同模式新增脚本，不另起机制）——断言：冷扫描落表 → 暖表重启后 plugin 表 removed=0 + 零 scan 命令 + 免扫装载 ok。**断言一律相对化**（removed=0），不得用 mac 的绝对数 719（PC 候选面 100，扫描表数量以 PC 实际为准）
  3. 回执：完整命令+退出码+run ID+HEAD+工作树状态+关键工件（§9 真栈回执契约），新工件目录不覆盖
- 约束：零生产代码改动预期；若 Windows 形态暴露平台差异缺陷→取证上交（代码锚点+最小复现），不越域修；token/key 零入工件
- 验收：ctest 双配置 exit 0 + 真栈双重启断言 exit 0（脚本退出码=门槛）
- 停止条件：同断点两次确定性失败 → 停止重跑，取证上交
- 领取：2026-09-26 16:50 +0800 / main=191aabff（决策会话领取并亲自执行，L2）/ 领取前工作树：M default_project.xml（运行时）+ untracked PCA journey 大目录，均与本卡无关不动
- 回执（2026-09-26 决策会话执行+自验，工件根 `coord/runs/PORT-WIN-HYGIENE-SMOKE-1/`，token/key 零入工件）：
  - **目标 1 ctest 双配置**：Windows 增量构建后 VitPluginListHygieneTests **Debug 1/1 PASS + Release 1/1 PASS**（cmake-build-pcverify1-tests，VS 18 2026 生成器）；含 HYGIENE 卡附带 A 的 Windows 平台条件断言同测覆盖。
  - **目标 2 真栈断言**：新脚本 `scripts/kernel_pluginlist_hygiene_smoke_pc.sh`（mac 版同模式移植到 Git Bash+python pyzmq；范围裁剪有据：mac LEG B/C 为 mac 事件腿（cancel/kill-9 pedal 二分）不复跑，其谓词语义由 ctest 单测在 Windows 双配置覆盖；PC 复跑决定性集=W1/W2/W3）。内核=Release 增量重建（含 37ae4f4，cmake-build-pcverify1/VitApp_artefacts，sha 在 run_meta）。**run hygiene_pc_20260926-172229 三腿 all_green，exit 0（ok=6 red=0）**：
    - W1 暖表启动：plugin_list_available **994/994**（Settings.xml knownPluginList64 994 条全保留，mac 缺陷形态应崩向 0）+ 零 "PluginListHygiene: removed" 行=**37ae4f4 在 Windows 形态零扰动实证**（PC 表全部条目 file 指向真实文件，Windows 形态下新旧谓词等价——语义不变量与预期一致）；
    - W2 免扫装载：knownPluginList 解析 `VST3-Jeesonic EQ Pro-b63d06bf-9620aaa8` → add_track(1007) → **rack_add_node ok plugin_id=1013 plugin_instance_ready=true，全程 scan_plugins 命令=0**（免扫装载经表解析路径成功，Jeesonic 为 bundle 目录内路径条目）；
    - W3 重启持久化：重启后 994==994（**removed=0**）+ 零 cleanup 行 + **零 scan 命令**（表从磁盘被信任）。
  - **§8 运行账目**：三轮不覆盖——run1（17:00）环境退出（pedal 残留为 BOM-only 空文件形态被误判非空；残留已备份 env_state/ 后清理，脚本判定改为 utf-16 解码判空）；run2（17:01）**脚本缺陷两处**（Git Bash MSYS PID≠Windows PID 致 taskkill 杀错对象+wait 阻塞；内核真实日志在 Workspace/Logs/VitHeadlessServer*.log 而 stdout 为空=日志断言不可靠）——均本卡脚本域内当场修复（kernel_winpid 解析+日志源重定向+断言前快照刷新），run2 的 W1 断言绿但日志源不可靠不采信；run3（17:22）全绿采信。run2 卡住时残留 VitApp 进程 taskkill 清理在案。
  - 工件：run 目录含 prereq/hygiene_report.json/run_meta/logs（内核日志拷贝）+replies（大体积插件全量 JSON 留盘不入库）；env_state/pedal 残留备份。
- 验收：**pass（2026-09-26 决策会话；执行与验收同一会话（L2 亲自），证据链完整在案可复核：报告 JSON+退出码+日志+三轮账目）**
