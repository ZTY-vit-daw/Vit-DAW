# Ruling：PORT-PCBATCH-MAC-LEGS-1 pass（2026-09-25）

- 卡：`coord/cards/done/2026-09-24-PORT-PCBATCH-MAC-LEGS-1.md`（Mac 决策会话开卡，mac 执行侧执行，Mac 决策会话验收）
- 提交：领取 `9354964` / 回执 `cd81223`；两 port 分支 cherry-pick 入 main=`3f08251`（HYGIENE 烟测脚本）+`ebd0428`（scan_poll 旋钮）
- 裁定：**pass**——PC 深夜批次四卡 mac 转交腿收口：白名单 47→**83（EQ 7→43）**、84 包 0 FAIL mac 首达、HYGIENE 真栈行为四绿；另带回两缺陷发现（开卡）与一渠道缺失（转 PC）

## 决策侧核验（独立复算）

1. **腿 1**：内核 `9e29796b…`（含 HYGIENE）/agent `ff282540…`（含 CERTAUTH）sha 亲验；旧内核备份在工件。
2. **腿 2**：HYGIENE 四细腿行为全绿（启动清理/取消重扫正向复验=09-23 事故场景闭环/kill-9 二分/719 全量+装载 spot plugin_id=1040）；无效 run 六例归因注记采信。**重大缺陷上交采信并开卡**：`PluginListHygiene.cpp:17` `existsAsFile()` 对 mac VST3 **bundle 目录**恒 false → 每次内核启动清空 719 有效条目（PC 单文件 .vst3 不触发）→ **FIX-KERNEL-HYGIENE-BUNDLE-1（P1）**。
3. **腿 3**：token 武装零泄漏；certify 53 目标处置 39/10/4 全记因（26.4%<止损线）；v1 store static_eq 8→45+45 条 certauth 日志=token 路径真实过闸；**derive 后白名单 83/EQ 43 实核**（sha fcb9d3ec）；overlay 86/0 VERIFY_EXIT=0；journey all_green（§8 三有效轮账目合规）。**第二缺陷开卡**：harness `scanAvailablePluginRows` 250ms 轮询跑在内核 JUCE message thread 饿死出壳收集（双看门狗实证）→ 驱动侧旋钮已入（ebd0428 断言零变化），harness 侧修复 → **FIX-HARNESS-SCANPOLL-1（P2）**。
4. **腿 4**：contextruntime 决策侧亲跑 ok；全量 84 包 0 FAIL 首次达成（full_go_test.log 在案）——CTXSYMLINK 卡 mac 复跑回执闭环。
5. **腿 5**：四路渠道核查（transfer/主仓/org 仓/工件）采信——PC 的 telemetry_manager.gd 确未到达，如实上交不自行重实现正确。**转 PC 投递**（format-patch 经 transfer 或 scp），到达后补腿。

## 遗留移交

- **FIX-KERNEL-HYGIENE-BUNDLE-1（P1，todo 新开）**：mac 阻断——bundle 目录谓词修复+bundle 形态单测；修复前 mac 每次重启栈需重扫（演示注意）。
- **FIX-HARNESS-SCANPOLL-1（P2，todo 新开）**：轮询落点/默认值（agent Go，harness.go）。
- 腿 5 等 PC 投递 telemetry_manager.gd 后补。
- coord/runs/FIX-PCA-AUTOSWEEP-1/20260925_mac/ 未跟踪 sweep 状态：按"journey 工作区机器本地"惯例留本地不入库（PC add -A 事故教训）。
- mac 候选面终态 83（EQ 43）；双端格局 mac 83 vs PC 100。
