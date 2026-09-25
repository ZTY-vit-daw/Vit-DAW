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
- 回执：
- 验收：
