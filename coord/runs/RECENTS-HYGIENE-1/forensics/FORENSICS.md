# RECENTS-HYGIENE-1 取证报告（执行侧，候主管验收）

- 执行：freeflash（GLM-5.3-Flash 免费车道子代理）·PC·2026-11-14
- 分支：port/recents-hygiene-1（worktree D:/Vit_DAW_wt_recents1）；取证=只读，清理动作另见 diff/
- 取证时进程核查：VitApp/VitAgent/godot 全零（两轮：取证前+备份前）

## 1. 存储定位（卡面线索修正）

- **recents 存储不在 `VitApp/Workspace/Settings/Settings.xml`**。Settings.xml 顶层结构（ROOT/SETTINGS/KNOWNPLUGINS/DEVICESETUP/MidiProgramManager 等）为插件扫描/音频设备/工程默认态，无任何 recent 字段（全文件 grep 零命中）。
- 真实存储：**`%APPDATA%\Vit-DAW\global_project_config.json`**（`juce::File::userApplicationDataDirectory` → Roaming）。内核源码锚点：
  - `VitApp/Source/Core/VitGlobalProjectConfig.h/.cpp`：`getConfigFile()` = userApplicationDataDirectory/Vit-DAW/global_project_config.json；`maxRecentEntries=10`；`prependRecentProject()` = 读→`normaliseAndDedupe`（大小写不敏感去重、insertFirst 置顶、超限尾删）→`writeRecentPathsToDisk()`（JSON `{"recent_projects":[...]}`，无时间戳字段）。
  - 读出面：`buildGetRecentProjectsReply()` → `get_recent_projects` 命令 → 启动面 recents 列表/greeting（FE-GUI-START-FEED-1 RECEIPT 已记载同一位置）。
- Settings.xml 处置：未提交改动为**既有运行时权威输入**（git status: ` M VitApp/Workspace/Settings/Settings.xml`、` M VitApp/Workspace/default_project.xml`）——本卡零触碰；字节快照留证 `backup/Settings.xml.pre_evidence_*.xml`（sha256 b773c82d…）。

## 2. 清单（清理前，sha256 c3543f45…、1418B、mtime 2026-11-10 00:29:51）

全量 10 条 = 上限满载，**全部**为装置/烟测 fixture，用户真实工程 0 条：

| # | 条目 | 来源 |
|---|---|---|
| 0-3 | `%TEMP%\govttl_327_projects\govttl_{round,warm}_{195753,200213}.vit` ×4 | FIX-GOV-INTERACTION-TTL-1（govttl 327 装置，2026-11-09 晚两轮） |
| 4-9 | `D:\Vit_DAW_wt_309recon\...\WAVESHELL-OFFRENDER-FIX-1\stackleg_20261109_1906\e{1ctrl,2a,2b,3,4,5}*\waveshell_stackleg_fixture_*.vit` ×6 | WAVESHELL-OFFRENDER-FIX-1（309 stackleg 八 run 中的六条 fixture save 残留） |

- 卡面所述「339-340 复跑」条目**不在**当前清单：339/340 装置自身做了 recents 原样还原（哈希亲证，净差零），其自身贡献已清零。
- 用户真实工程仍在盘上但**已全部被挤出 recents**：`$APPDATA/Vit-DAW/projects/` 下手测·小星星乐局.vit、实验·ProQ3 裸控 r1/r2/r3.vit、_320_run8_默认态存档_20261109。

## 3. 泄漏路径定位（读证据，不改内核/脚本）

**内核侧单点（唯一写入口）**：`VitHeadlessService.cpp` 三处无条件调用 `globalProjectConfig->prependRecentProject(currentProjectPath)`：
- `open_project` 成功后（~L1207）
- `save_project` 成功后（~L1353）
- `save_as_project` 成功后（~L1475）

无任何调用方上下文区分（无 env 开关、无 smoke 模式位、无路径白名单）——**凡驱动真实内核走工程生命周期命令的会话（用户旅程与烟测装置同权）都写同一份用户级 recents 文件**。

**脚本侧实证（两条泄漏源）**：
- govttl 327 装置：`coord/runs/FIX-GOV-INTERACTION-TTL-1/gov_ttl_replay_driver.py` L109-148：`new_project` → `save_as_project file_path=%TEMP%\govttl_327_projects\govttl_round_<stamp>.vit`（round 副本 B）、warm 工程 A 同法 save_as → `open_project_kernel(project_b)`——每次 save_as/open 都命中内核 prependRecentProject。两轮 stamp（195753/200213）×2 工程=4 条。
- 309 stackleg：`D:/Vit_DAW_wt_309recon/scripts/waveshell_stackleg_smoke.py` L373-378：render 腿 `new_project` → `save_as_project file_path=<run 工件目录>\waveshell_stackleg_fixture_*.vit`——6 个 run 子目录各存一份 fixture=6 条。

**泄漏时间线（哈希对账闭合）**：

| 时刻 | 事件 | 证据 |
|---|---|---|
| 11-09 19:06 | 309 stackleg 六腿 save_as → 6 条 fixture 入 recents，开始挤占 | wt_309recon run 目录 |
| 11-09 19:57/20:02 | govttl 两轮 save_as+open → +4 条；**用户真实工程被 10 条上限挤出** | gov_ttl_replay_driver 工件 |
| 11-09 23:19:33 | 339 装置 `main_A` before 快照 = **已是污染 10 条**（与当前文件逐条一致） | `coord/runs/LAUNCHER-SMOKE-1/main_A_20261109_231933/global_project_config.before.json`（sha256 同 c3543f45） |
| 11-09 23:19-23:51 | 339 种子+运行+teardown **原样还原哈希亲证**（还原基准=23:19 已污染快照） | 339 卡面栈登记 |
| 11-10 00:12 | 340 主腿 before 快照仍=污染 10 条（同 sha256）；主腿开 340 自带小星星 fixture → e1/e2a 两条被顶出、340 fixture 入列 | `LAUNCHER-GATE-FIX-1/main_20261110_001205/before` + `fallback_20261110_001320/before`（1312B，含 340 fixture 无 e1/e2a） |
| 11-10 00:29:51 | 340 teardown 还原至会话起始快照（=污染 10 条）→ **当前文件终态**，mtime 即此还原写 | 当前文件 sha256 == 23:19 快照 |

**结论（卡面问点：「339 还原的覆盖范围与更早泄漏的并存」）**：339/340 的还原机制覆盖**各自装置会话窗口**（会话前快照→会话后还原，净差零亲证），**不是**对文件历史的清洗——还原基准快照本身已含 11-09 晚泄漏，故「装置净差零」与「用户面持续污染」并存且互不矛盾。当前污染 = 339 机制建立**之前**的 309 stackleg + govttl 327 泄漏，被历次「原样还原」忠实保留至今。

## 4. 附带损害（上交候裁定，不在本卡处置范围）

1. **用户真实工程已被挤出 recents**（不可逆，除非重建）：精确被挤出清单无快照可考（全 coord/runs 无早于 11-09 23:19 的 recents 快照）；可按 `$APPDATA/Vit-DAW/projects/` 目录内容候选重建（手测·小星星乐局.vit 等五项），是否重建、以何顺序置顶=主管/用户裁定。
2. **fixture 条目部分指向临时盘路径**（%TEMP%\govttl_327_projects\，文件现仍存但属临时域）与**另一 worktree 工件域**（wt_309recon）——启动面一旦点开即「进入了不知道哪个的工程」（发卡依据截图现象的机理）。

## 5. 停止条件核对

- 存储格式：已明（JSON 单键数组）→ 未触发「格式未知」停止条件。
- 写保护：文件可写（历史多次被内核写）→ 未触发。
- 清理对内核启动影响：recents 仅被 `get_recent_projects` 读出呈现，不参与工程加载/内核启动路径 → 清空数组不构成启动风险（依据：VitGlobalProjectConfig.cpp 全文仅 load/prepend/reply 三功能；空数组 `loadRecentPathsFromDisk` 返回空 StringArray）。
