# 子代理名册（复用优先）

> 用途：主管发卡前先查此表，优先 `send_message` 复用同一产线子代理；新卡/完结后更新。最后更新：2026-11-19 19:13。

| owner | 产线 | worktree / 分支 | 最近任务 | 状态 |
|---|---|---|---|---|
| `convrail418` | 入口/对话流 UI | pc418-convrail / pc421-cross / pc426-settings / pc428-window / pc429-motion / pc430-motion / pc431-motion | 431 左栏 800ms | inactive（可续） |
| `legameparse423` | 乐局 Go 线（W01） | pc423-parse / pc424-compile / pc425-bind | 425 零栈绑定；真栈腿待栈位 | inactive（可续） |
| `score397stack` | 评分绑定/真栈 | pc397-merge / codex/score-binding-397-integ | 397 真栈 PASS | inactive（可续） |
| `materials420` | 教学材料线 | pc420-materials | 420 材料批2-4 PASS | inactive（可续） |
| `railb1` | 入口壳层 B1 | pc417-rail | 417 B1 PASS | inactive（可续） |
| `convopen419` | agent 组合工具 | pc419-open | 419 conversation_open PASS | inactive（可续） |
| `w02clip` | W02 乐局上下文/clip | pc432-clip / codex/w02-a1a-clip-432 | 432 A1a PASS 66ad1d66 | inactive（可续） |
| `combo422` | 422 跨工作树验证装置 | pc422-combo / codex/conv-cross-combo-422 | 422 真栈 PASS 218dff3c | inactive（可续） |
| `entry413` / `ringfix416` / `presmerge412` / `producer410` / `ready411` | 窗口线旧执行 | 各自 worktree | 已结案 | inactive（一般不再复用） |

## 复用规则

- 同一产线新卡：直接 `send_message` 给 owner，附卡路径+本次新增约束；不重述 AGENTS 全文。
- 小修/连续调参：合并给同一 owner，一次给多步指令。
- 仅当文件域不同/原 owner 阻塞/需并行不相交时新建子代理；新名字按产线命名便于入册。
- 跨会话：名册保留作参考；若原 agent 不可续，用同产线名新建，先读 `NEXT-SESSION-BRIEF` + 卡/裁定，不读旧聊天。
