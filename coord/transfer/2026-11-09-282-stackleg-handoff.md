# 282 EDU-ASK-ANCHOR-1 真栈轻腿转交提示词（2026-11-09 发）

> 用法：整段复制到一个新执行会话（ZCode L2-worker / PC）。实现腿已轻验 pass 并推远端，本腿只补真栈断言+合入。

---

你是 Vit-DAW 仓库 282 卡（EDU-ASK-ANCHOR-1，注册链④）的执行会话，做**真栈轻腿 + 合入**。实现 commit `873c0e9c` 已在分支 `port/edu-ask-anchor-1`（worktree `D:/Vit_DAW_wt_282impl`），不合入 main 归主管——本腿通过后按主管授权合入。

## 必读

1. 卡面：`coord/cards/doing/2026-11-05-EDU-ASK-ANCHOR-1.md`（回执节有全部实现细节与申报事项）
2. 协议：`coord/PROTOCOL.md`；栈登记：`coord/resources/PC-RUNTIME-STACK.md`

## 本腿目标（卡面验收标准原文）

- **chat 驱动 loop 设置/清除可用**：真栈上经 chat 发起 `transport.set_loop`（start_seconds/end_seconds）与 `transport.clear_loop`，断言传输态生效与清除成功（exit 0）；
- 引导段=纯文本新增，既有 pack 行为零变化（回执已附 parity 证据，本腿无需重证，真栈冒测不回退即可）。

## 执行要点

- **便车优先**：309 判别腿今日占栈（见栈登记），本腿候其释放后同日或次日轻窗口；不与 309 并行起栈；
- 轻腿形态：起栈（`dev_agent_smoke.ps1 -StartKernel -RestartAgent` 或既有轻量参数）→ chat 驱动两命令 → 断言 → TEARDOWN_ALL_ZERO → 释放登记；
- 栈协议两条登记（占用/释放）+ 退出码 + run 工件目录；
- 通过后：主管验收 → 合入 main（commit message 含卡 ID 282）→ `git show --stat --oneline HEAD` 核对回执。

## 停止条件

- 真栈上 transport 命令链失败且定位进入内核域（CommandDispatcher 面）→ 取证上交，不越域修内核；
- 与 309 窗口冲突 → 顺延，不抢栈。

## 回执义务

完整命令、退出码、run ID、HEAD/工作树状态、栈两条登记、端测边界声明（AGENTS §5：真栈 agent HTTP/ZMQ 面；渲染面/用户旅程不在断言面）。
