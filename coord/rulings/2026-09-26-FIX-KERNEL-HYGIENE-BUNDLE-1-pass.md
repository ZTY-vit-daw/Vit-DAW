# Ruling：FIX-KERNEL-HYGIENE-BUNDLE-1 pass（2026-09-26）

- 卡：`coord/cards/done/2026-09-25-FIX-KERNEL-HYGIENE-BUNDLE-1.md`（Mac 决策会话开卡，mac 执行侧执行，Mac 决策会话验收）
- 提交：回执 `1c24e45` / 实现 `9d48736`（port/fix-hygiene-bundle）经决策侧 cherry-pick 入 main=`37ae4f4`
- 裁定：**pass**——mac 阻断缺陷修复：内核启动不再清空 719 有效条目，**扫描一次持久免重跑恢复**

## 决策侧核验

1. **谓词 diff 亲核**：`existsAsFile()`→`exists()`（目录或文件皆真），语义不变量保持（只清路径化 VST3 条目中文件系统确实不存在者；built-in/非路径/黑名单逻辑零改动——diff 仅谓词行+测试）。
2. **红绿**：修前 exit 134（MacBundle.vst3 目录形态被清=719 清空缺陷单元级复现）/修后 exit 0；ctest 6/6 真 exit 0（mac 首跑）；内核重建 exit 0（新 sha `5fb4585b`）。
3. **双重启决定性断言（本卡核心）**：run1c 全量扫描 719 落表（Settings 7168B→301683B/719 条）；run2 `--skip-full-rescan`：**"PluginListHygiene: removed" 行=0 + scan_plugins 命令=0 + 免扫装载 rack_add ok（plugin_id=1040，knownPluginList 解析路径）+ 表终态 719**——缺陷反转的完整证据链。
4. **演示面二进制**：`VitApp/build/…/Debug/VitApp` 已替换为 `5fb4585b`（决策侧亲验 sha，=终验同件；旧件备份在案）。
5. **附带上交处置**：A) JUCE 8.0.12 `File::isAbsolutePath` 仅 JUCE_WINDOWS 认盘符（juce_File.cpp:420-430）致既有 startup 断言 mac 永红——按平台条件期望修正（Windows 分支逐条同原文，生产门 `pluginListEntryIsPathValidated` 零改动）**采信**；**转 PC：合入后复跑其 Windows HYGIENE 烟测确认零扰动（卡面目标③收口）**。B) 单测拆卸序非致命打印（exit 0）记录在案。
6. 作废 run 两例归因（路径漂移/设备管理器超时=环境类）采信。

## 遗留移交

- **转 PC**：①合入后 Windows 形态 HYGIENE 烟测复跑（目标③）；②SCANPOLL 验收已转 mac 两腿（默认旋钮 journey 实证+通过后撤 ebd0428 驱动旋钮补丁）。
- **mac 收尾腿卡（新开）**：①SCANPOLL 默认旋钮实证+journey 旋钮补丁回撤；②腿 5 补件应用（PC 已 scp 投递 telemetry_manager.gd `eb93e210`+probe `ee620e8b` 至 ~/Desktop/pcbatch_leg5/，决策侧已核 sha——应用至前端仓+加载确认+回执闭环）。
- mac 至此：候选面 83（EQ 43）+重启零清理持久化+84 包 0 FAIL——**候选面工程与三缺陷链全部闭环，剩收尾腿卡一张**。
