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
