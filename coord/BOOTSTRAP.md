# BOOTSTRAP：双端口令中枢（权威定义，入仓单一事实源）

- 建立：2026-09-16，用户裁定「双端使用同一套口令中枢」
- 结构：本文件 = 口令与流程的**唯一权威定义**；两端机器本地的工作区级 AGENTS.md（`~/.zcode/workspace/default/AGENTS.md`，各自维护、不入仓）只做一件事——认识口令并路由到本文件
- 设计原则：**用户在对话流中亲自输入的口令才是行动授权**；对端推送只产生汇报，不产生行动

## 口令表（双端通用，语义按角色映射）

| 口令 | Mac 执行侧语义 | PC 决策侧语义 |
|---|---|---|
| 「值班」 | 读 coord/PROTOCOL.md + AGENTS.md → 起 watcher v2（防重复守卫）→ fetch → **只报告**（main 新提交 / coord 变化 / transfer 到达件 / todo 卡建议） | 同左，但报告内容按决策侧职责（done/ 待验收卡 / blocked/ / decisions/ 新项 / transfer 到达件） |
| 「开工 <卡ID>」 | 用户点名即视为领取批准：按卡片状态机领卡，port/* 分支开工（如「开工 A2」） | 用户点名即视为验收批准：审 port/* 分支 diff + 本地复验 → 写 ruling → 归档（如「开工 C1」= 启动 C1 验收） |
| 「收工」 | 执行收工 gate：汇报交付与未竟、确认工作树干净与卡状态已推、停本流 watcher，下一条流由用户新开 | 同左（决策侧 gate 含当日裁定与派卡情况汇总） |

## 各端注记

### Mac 端（详见 [BOOTSTRAP-MAC.md](BOOTSTRAP-MAC.md)）

- 主仓实际路径 `~/Documents/Vit-DAW`（连字符；PROTOCOL §4 写的 `~/Vit-DAW` 与实际不符，watcher 用实际路径）
- watcher 指纹命令用 `shasum`（非 sha1sum）；网络代理已配（git 全局 `http.https://github.com.proxy=127.0.0.1:7890`），无需环境变量
- 2h 心跳自动化为工作区级定时任务（只报告模式，跨流存活，自愈重启 watcher）

### PC 端（决策侧部署时补记）

- watcher v2 命令见 PROTOCOL §4（Git Bash / sha1sum / `/d/Vit_DAW`）；定时任务已移除（2026-09-16 用户裁定），watcher 由口令「值班」自愈
- 口令路由已写入 Windows `~/.zcode/workspace/default/AGENTS.md`（2026-09-16，用户在场）
- **「开工 <卡ID>」PC 侧语义（用户确认 2026-09-16）**：授权启动该卡验收流程，pass/rework 由证据决定，口令≠必须通过；验收流程内的写操作（ruling 文件、卡片归档、合入 main、cherry-pick）均含于该次授权，无需逐项再批
- **多卡规则（用户确认 2026-09-16）**：默认一条对话流一张卡（对齐"一任务一会话"纪律）；仅当用户明确点名多卡（如「开工 A2 C4」）才作为单条流的多卡授权

## 硬性护栏（任何口令下双端共同遵守）

- 先报告再动手：仓库/远端变更不构成行动授权；watcher/心跳自运维与只读命令除外
- 禁止 `git reset --hard` / `git clean` / 改写已推送历史；遵守 PROTOCOL §3 写权限边界
- 双端通信只经 git（主仓 + transfer 仓）；需要真栈或用户在场的验收不自动执行（PROTOCOL §4）
