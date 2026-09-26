# FIX-KERNEL-HYGIENE-BUNDLE-1：PluginListHygiene 存在性谓词 bundle 感知修复——mac 启动清空 719 有效条目阻断缺陷

- 优先级 / 预估 / 依赖：**P1（mac 阻断：每次内核启动把 knownPluginList 719/719 有效条目全清，扫描免重跑持久化被击穿——修复前 mac 每次重启栈需 5-10 分钟重扫）** / 0.3 天 / PORT-PCBATCH-MAC-LEGS-1 腿 2 发现（取证与量化在该卡工件 leg2_hygiene/）
- 模型分级：L2 / GLM-5.3（内核 C++ 谓词级修复+单测，红测试先行）
- **背景（取证在案）**：FIX-KERNEL-PLUGINLIST-HYGIENE-1 的惰性清理用 `PluginListHygiene.cpp:17` `File::existsAsFile()` 判条目存在性——**mac 的 VST3 是 bundle 目录**（.vst3=目录），`existsAsFile` 对目录恒 false → 每次启动全部条目判"陈旧"清空；PC 的 .vst3 是单文件故 PC 烟测未暴露（跨平台测试缺口）。mac 实测：启动即清 719/719，日志与量化在 leg2_hygiene。
- 目标：
  1. 谓词修复：存在性判定改 `File::exists()`（目录或文件皆真）或显式 bundle 感知（`.vst3` 目录按存在处理）；语义不变量=**只清路径化 VST3 条目中文件系统确实不存在者**（built-in/非路径/黑名单逻辑不动）
  2. 红测试：修前红=mac 形态 bundle 目录条目被清（单测构造 .vst3 目录形态断言保留）；修后绿=保留+陈旧路径仍清（双态断言，Windows 单文件形态回归同测覆盖）
  3. VitApp Release 构建+ctest 全绿；PC 侧 Windows 形态回归（合入后 PC 决策侧复跑其烟测确认零扰动）
  4. mac 真栈终验：重启栈两次——第一次扫描 719 入表，第二次启动**零清理**+免扫装载 spot 成功（持久化恢复，用 scripts/kernel_pluginlist_hygiene_smoke_mac.sh 腿④或同款）
- 文件域：`VitApp/Source/Service/PluginListHygiene.cpp`（谓词行）+`VitApp/Tests/PluginListHygieneTests.cpp`（bundle 形态断言）；零其他
- 验收：①红绿（bundle 双态+Windows 回归）②构建+ctest 绿 ③mac 重启两次零清理+免扫装载证据 ④回执两端 HEAD
- 停止条件：谓词语义外发现 HYGIENE 其他 mac 形态缺陷 → 一并取证上交不扩域
- 领取：2026-09-26 02:54 / origin/main=af4c0459bef57409b75e40cc34288a9b9d08bba5（与发卡基线一致）/ port/fix-hygiene-bundle
- 回执：实现 commit=**9d48736**（port/fix-hygiene-bundle 已推 origin；分支基点 c249e3d=af4c045+领取/coord 提交，代码内容与发卡基线一致）。红绿证据+构建/ctest 退出码+双重启零清理与免扫装载证据全量索引：`~/Documents/vit-fix-hygiene-bundle-artifacts/EVIDENCE_INDEX.txt`（run 目录/快照/日志均在同根 leg2_hygiene/<runid>/）。要点：①红=修前 exit 134（MacBundle.vst3 目录形态被清，719 清空缺陷单元级复现）/绿=修后 exit 0；②Tests 工程 ctest **6/6 真 exit 0**（mac 首跑）+内核重建 exit 0（leg1 树同款配方，新 sha 5fb4585b）；③真栈 run1c（20260926-120834）重启#1 全量扫描 all_green EXIT=0：plugin_count=719、真工作区 Settings.xml 7168B/0条→301683B/**719条**落表；run2（20260926-121315）重启#2 --skip-full-rescan all_green EXIT=0：暖表启动 **"PluginListHygiene: removed" 行=0**（缺陷反转决定性断言）+**scan_plugins 命令=0**＋免扫装载 rack_add ok plugin_id=1040（knownPluginList 解析路径）+表终态 719；④演示面二进制已替换（旧 9e29796b 备份 kernel_binary_swap/VitApp.old-20260926-0400，新 5fb4585b=终验所用同一二进制）。端测边界：内核侧四腿全测，agent/webui 无涉未重建。作废 run 两例已记录归因（run1=leg1 树路径致 Settings 落游离根，证据已归档后清理；run1b=设备管理器初始化 240s 超时，环境类，重跑即过零代码改动）。附带发现两则上交：A) 既有测试平台缺口——JUCE 8.0.12 `File::isAbsolutePath` 仅 JUCE_WINDOWS 认 `X:` 盘符（juce_File.cpp:420-430），既有 startup 断言 mac 永红；已按 #if JUCE_WINDOWS 平台条件期望修正（Windows 分支断言逐条同原文），**生产门 pluginListEntryIsPathValidated 零改动**，PC 合入后请复跑其 HYGIENE 烟测确认零扰动（卡面目标③）；B) 单测收尾有一次非致命 JUCE 拆卸序断言打印（juce_MessageListener.cpp:50，exit 0 不受影响），仅记录。谓词语义外未发现 HYGIENE 其他 mac 形态缺陷。
- 验收：**pass（2026-09-26 Mac 决策会话，[rulings/2026-09-26-FIX-KERNEL-HYGIENE-BUNDLE-1-pass.md](../../rulings/2026-09-26-FIX-KERNEL-HYGIENE-BUNDLE-1-pass.md)）**——谓词 diff 决策侧亲核（existsAsFile→exists 语义不变量保）；红 134/绿 0+ctest 6/6 采信；**双重启决定性断言采信**（run2 "removed"=0+scan 命令=0+免扫装载 ok+表终态 719=mac 持久化恢复）；演示二进制 5fb4585b 就位亲验；附带 A（JUCE isAbsolutePath 平台条件测试修正，生产门零改动）采信+**转 PC 合入后复跑 Windows HYGIENE 烟测**；B（拆卸序打印）记录。实现 9d48736 经决策侧 cherry-pick 入 main=37ae4f4
