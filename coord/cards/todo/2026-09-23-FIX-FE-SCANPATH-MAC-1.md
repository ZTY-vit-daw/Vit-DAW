# FIX-FE-SCANPATH-MAC-1：前端设置页 VST3 扫描路径 mac 分支——路线2（设置页扫描→EQ 全链）端到端可用

- 优先级 / 预估 / 依赖：**P1（2026-09-24 导师课演示阻塞，用户直令「直接修复」）** / 0.2 天 / FIX-D1-PLUGIDENT-1 已合入（identifier 链路已通）；手测取证 `~/Documents/vit-handtest-forensics-20260923/` + 用户截图（设置页报错）
- 模型分级：L1 / GLM-5.3 flash 可接（单文件小分支+端到端复验）
- **目标仓库注意**：本卡改动在 **Godot 前端仓 `~/Documents/vit-daw-frontend`**（主仓之外、独立 git 仓，port/* 纪律沿用）；主仓仅 coord 卡片状态变更
- **背景（2026-09-23 17:38 手测取证）**：设置→Plugin Manager→扫描报 `scan_plugins requires at least one existing VST3 directory`。根因=`app/settings/global_settings_modal.gd:1270-1279` 路径列表装载读 Windows 环境变量（`CommonProgramFiles`/`ProgramFiles`），mac 上均为空 → `"".path_join("VST3")` 产出字面量 `"VST3"` 入列表 → 内核 `PluginRackControlService.cpp:2598` 判非存在目录拒扫。**内核扫描机制本身健康**（旅程轮每次都在用）；坏的只是前端这一段路径组装（B3 时代移植漏 mac 分支）。UI 无路径增删控件（列表纯程序生成），用户无法在界面绕过。
- 目标：
  1. 路径列表装载加 **macOS 分支**：标准目录 `/Library/Audio/Plug-Ins/VST3` 与用户级 `~/Library/Audio/Plug-Ins/VST3`（仅添加实际存在的；两者都空时保留一条可读提示而非假路径）；**Windows 分支逐字不动**（环境变量逻辑原样）
  2. **路线 2 端到端**（本卡验收核心，用户直令）：不设任何环境变量、从 Godot 正常拉起 → 打开 `~/Documents/vit-handtest-912/912.vit` → 设置→Plugin Manager → 路径列表显示真实 mac 目录 → 点 Scan → 进度条走完、状态 ok → 插件列表填充（列表区或左侧插件浏览器可见 Waves 主体）
  3. **演示链闭环**：扫描完成后不重启，聊天发 EQ 请求（如「低音轨的低中频有点糊，和鼓的低频区分不清，帮我处理一下」）→ 模型 EQ 全链走完（假设→准入→装载 Q10→参数应用→A/B）——knownPluginList 已由设置页扫描填充，identifier 解析应通过
- 约束：仅前端仓 `app/settings/global_settings_modal.gd`（必要时 `app/settings/settings_modal_rules.gd` 的默认路径常量旁注 mac 语义，不改 Windows 值）；零主仓代码改动、零内核改动；前端仓未跟踪运行时目录（`.vit_agent/`、`.vit_history/`、`VitApp/`）不碰不提交；测试前与用户协调关闭现存 Godot 现场（单栈所有权 §9；取证已固化）
- 验收：① 前端仓分支 diff（mac 分支+Windows 面不变，逐行可审）；② 路线 2 证据：设置页截图（真实路径+扫描完成状态）或前端控制台/内核日志（scan ok+列表填充）；③ EQ 全链证据：装载成功+参数应用回读+A/B 事件（日志指针）；④ 回执：前端仓 commit hash+分支名、两端 HEAD（主仓领取/完成 hash）、§8 纪律（EQ 轮 N=1、模型未动机化 EQ 可加跑一轮、两轮未触发如实记录不硬凑）
- 停止条件：扫描 ok 但 EQ 链在**新**红点失败 → 如实记录红点与形态 mv blocked 上交（不扩域自修）；设置模态 mac 侧暴露更深问题（路径修复后仍不可扫）→ 同样上交
- 领取：
- 回执：
- 验收：
