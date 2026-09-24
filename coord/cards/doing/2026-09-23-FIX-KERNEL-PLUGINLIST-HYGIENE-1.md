# FIX-KERNEL-PLUGINLIST-HYGIENE-1：内核插件表卫生——陈旧跨平台条目清理 + 取消/崩溃隔离语义区分

- 优先级 / 预估 / 依赖：P2（开源用户必踩面；演示已绕过）/ 1 天 / FIX-FE-SCANPATH-MAC-1 验收登记（rulings/2026-09-23-FIX-FE-SCANPATH-MAC-1-pass.md）；需 PC 决策侧共建（内核 C++ 双端共享）
- 模型分级：L3 / GLM-5.3（内核 C++ 行为语义+双端回归）
- **背景（2026-09-23 手测链两次机器态清污取证，工件 `~/Documents/vit-handtest-forensics-20260923/`）**：
  ① `knownPluginList64`（VitApp/Workspace/Settings/Settings.xml）从 PC 移植继承了 994 条 `C:\Program Files\...` 路径幽灵条目——同名列去重先命中 PC 条目，mac 扫描的 mac 形态标识串进不了表，白名单/装载查找必失败且**表面无错**（扫描"成功"但列表不变）；
  ② 设置页扫描中取消（按钮复用为取消）触发 JUCE 崩溃隔离——正扫的 shell 主文件被 `<BLACKLISTED>` 拉黑+死蹬 pedal 损坏，后续所有扫描**静默秒跳过**（completed plugins=1 形态），普通用户无法自救（需手改 Settings.xml）。
- 目标：
  1. 内核启动加载 `knownPluginList64` 后清理 `file` 路径不存在的条目（含黑名单路径同样校验）；清理动作落日志（`PluginRackControlService` 或引擎初始化层，锚点由执行侧定位）
  2. 扫描协调器区分"用户主动取消"（`plugin_scan_cancel`）与"进程崩溃/看门狗超时"——仅后者进入崩溃隔离/黑名单；主动取消应清理死蹬 pedal 并保留已扫成果（部分成员合法在表）
  3. （顺带评估）扫描取消后 UI 侧文案/按钮状态提示，避免用户误触取消（前端仓小改，可与 2 同卡或拆子项）
- 文件域：`VitApp/Source/Service/PluginRackControlService.cpp`（协调器+列表加载路径）；如涉及 `CommandDispatcher.cpp` 取消命令路径列明；前端仓 UI 提示为可选子项（列明则允许）
- 验收：①构造含不存在路径条目的 Settings 启动→日志显示清理且条目数正确；②扫描中取消→重扫不跳过（黑名单无新增、pedal 干净、部分成果保留）；③取消与 kill -9 崩溃两形态对照——崩溃仍触发隔离（防御不弱化）；④双端回归：mac 真栈扫描全量+装载 spot；PC 侧同测（决策侧安排）
- 停止条件：取消/崩溃语义在 JUCE 层无法区分（隔离在 juce::PluginDirectoryScanner 内部）→ 上交方案选项（pedal 预写策略绕开/升级 JUCE/自管隔离表），不硬改第三方
- 领取：2026-09-24 / origin/main 6774b9fba197386e771f2faf47b9c6d2d2b9c62e / port/fix-kernel-hygiene（独立 worktree D:/Vit_DAW_worktrees/kernel-pluginlist-hygiene，PC 执行会话 B）
- 回执：
- 验收：
