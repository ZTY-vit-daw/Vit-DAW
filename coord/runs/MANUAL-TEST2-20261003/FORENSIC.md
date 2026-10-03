# 手测场二号取证报告（2026-10-03 19:35–19:46 活栈，决策侧只读）

> 背景：手测二号四项反馈；现场未关。取证=白名单 GET（runtime/status、state）+磁盘会话图直读（工程 draft_20261003T113402_2e2a41ac，session_20261003T113402_86baa764 conversation_graph.json，8 节点全量）。全程零写操作。

## 判定链核验（组 1）——三症状修复全部在用户栈实证

- **症状 A 修复实证**：节点 `n_20261003T113721_9aa8dcea`（msg_kind=assistant，logical_message_id=`judgment_settle:judgment-9e0daba728b065d6`）——「A/B 判定已落账并完成结算：已按你的判定保留改动后状态。user preferred B; treatment candidate retained。（判定证据 …）」**结算确认作为正式助手消息落进了对话流**（SETTLE-DELIVER-1 症状 A 在用户栈达成）。
- **症状 B 修复实证**：二轮输入 `n_20261003T113737`「再帮我看看bass轨道的低频有没有什么问题」**正常进入治理链执行**（"前面的工具操作已完成"=工具面跑完；无 revision stale、无 ownership 报错、未 25 秒空转）。
- **症状 C**：二轮 ask 节点 createdAt（11:37:37Z）在结算回复（11:37:21Z）之后，无插队形态。

## 新缺陷（用户反馈 1）——终局回复生成失败：未知或不允许的工具（空名）

- 节点 `n_20261003T113752_f1df6feb`（**msg_kind=error**）：「前面的工具操作已完成，但最终回复生成失败：**未知或不允许的工具：**」——**冒号后工具名为空**。链已跑完工具操作、死在最终回复生成段的工具准入判定。
- 与晨场 revision_stale 不同层（晨场=进链即停；本场=进链、工具跑完、死在收口段）——剥洋葱式下一层，属新缺陷面非 SETTLE-DELIVER 回归。
- 复现口径：同一会话内一轮判定结算后再发观察类问句（19:35 首轮→19:37 判定→19:37:37 二轮→19:37:52 报错）。

## 便签会话取证（用户反馈 3+4）

- 节点 `n_20261003T113827` ask「hi」+ `n_20261003T113832` assistant（问候+工程六轨概况+只读声明）——**便签问答落进了主会话图（同一 session graph）**，非独立会话：NOTESTREAM-2 相位 1 取证问题的直接实锚（note Q&A 会话路由现状=入主流）。
- 用户所报「webui 内新出现的对话流显示报错」的机制未定（webui 侧 sessionFlow 注册表在浏览器 localStorage，agent 面不可见）——候选解释：侧边栏新会话条目的内容预览取到了 error 节点（n_113752）；确证归 NOTESTREAM-2/SESSION-SEMANTICS 取证。
- 命名缺失（便签流与默认流均"未命名对话流"）=已池内两卡的目标面（NOTESTREAM-2 目标 6 / SESSION-SEMANTICS-1 主会话命名）。

## 工件

probe_runtime_status.json / probe_state.json / 本报告；会话图原件在 ProjectHistory drafts（只读未动）。
