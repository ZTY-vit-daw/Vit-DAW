# Ruling：FIX-FE-SCANPATH-MAC-1 pass（2026-09-23）

- 卡：`coord/cards/done/2026-09-23-FIX-FE-SCANPATH-MAC-1.md`
- 实现：前端仓 `port/fe-scanpath-mac` @`a9890ba`（ff 入前端仓 main）；主仓零代码改动
- 裁定：**pass**——路线 2（设置页扫描→EQ 全链）经两层机器态清污后用户在场端到端全绿，2026-09-24 导师课演示路径打通

## 验收经过（决策侧全程取证，三层问题串）

1. **前端缺陷（本卡目标，已修）**：设置页路径列表读 Windows 环境变量（`CommonProgramFiles`/`ProgramFiles`），mac 上拼出字面量 `VST3` 被内核拒扫——`a9890ba` 加 macOS 标准目录分支（系统级+用户级、存在性过滤），Windows 面逐字保留，`godot --headless --check-only` 过。修复后设置页正确发送 `scan_plugins paths=[/Library/Audio/Plug-Ins/VST3]`（内核日志实证）。
2. **卡外机器态一（决策侧清污）**：`knownPluginList64` 内 994 条 PC 路径幽灵条目按名字遮蔽 mac 扫描（同名列去重先命中 PC 条目，mac 形态标识串 `…-10456661-…` 进不了表，白名单查找必失败）——旅程轮从未暴露因脚本用隔离 kernel_workspace。清空插件表（备份在案）。
3. **卡外机器态二（决策侧清污）**：首次扫描被取消（按钮"扫描中再点=取消"）→ JUCE 崩溃隔离把主壳 `<BLACKLISTED>` 拉黑 + 死蹬 pedal 损坏 → 后续扫描秒跳过（每轮 completed plugins=1，仅 ARA 壳 Sync Vx 入表）。删黑名单条目+清 pedal。
4. **终验（用户在场）**：一次扫描走完（主壳成员入表）→ 聊天 EQ 请求 → **模型 EQ 全链『假设→准入→Q10 装载→参数应用→A/B』在前端入口首次走通**。持久化重启免扫验证未单独执行（可选步，机制已实证）。

## 登记与移交

- **FIX-KERNEL-PLUGINLIST-HYGIENE-1（P2，todo）**：①内核启动时清理 `file` 路径不存在的 knownPluginList 条目（跨平台移植卫生——本次 PC 幽灵条目 994 条的根治）；②扫描"用户取消"与"真崩溃"区分——取消不应触发 `<BLACKLISTED>` 隔离（本次取消即拉黑导致后续扫描静默跳过，开源用户会直接踩）。
- **模型叙事抢跑（decisions 登记）**：装载失败轮模型先称"已加载 EQ"、内核随即失败回滚、GUI 正确显示空——模型叙述先于执行确认，需在呈现层/提示层收敛（成功叙述应以执行收据为准）。
- **修正先前结论**：mac 扫描**有**落盘持久化（`knownPluginList64`，跨工程共享、重启保留）——REALSTEMS 时代"不落盘每进程空表"判断系旅程隔离工作区特例，已在 decisions 更正记录。开源 backlog 原"扫描落盘"关切收敛为"表卫生+取消语义"。
- 演示路径固化：启动→（首次或插件变更时）设置页**一次**扫描走完勿取消→聊天 EQ 请求；扫描成果持久化，重启免扫。
