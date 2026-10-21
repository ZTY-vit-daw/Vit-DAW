# REPLY-FACE-BOUNDARY-LAYER-1：能力边界回复面按 boundary 分层选文案（146-L3 修复）

- 发卡：GLM 主管决策流 / 2026-10-20 深夜（依据=146 取证验收 pass，拆卡 D 采纳；L3=boundary=proposal_missing 时 failed_gate_ids 劫持文案+压掉模型 summary）
- 派发确认：已确认（146 卡验收裁定，2026-10-20 晚窗）
- 验收负责人：GLM 主管决策流
- 池序 152；目标仓库=D:\Vit_DAW；P1（最小独立修，先解用户面误导）/ 0.25 天 / **依赖=151 合入后领取（同文件域 goalrunner_chat.go 串行）**
- 模型分级：L1 / flash 可接
- **规格全文=[146 设计稿 §三 卡D](../../reports/2026-10-20-146-timefx-design-and-experiment.md) + [146 取证 L3](../../reports/2026-10-20-146-timefx-bus-forensics.md)（权威）**——freeStateCapabilityBlockedReply 按 receipt.boundary 选分支：proposal_missing/proposal_invalid → decision.Summary 分支（模型自报边界的可操作原因直达用户，如「工程信息没有披露人声轨…」）；admission_gate_failed → 门文案（148④ 已落地的用户语门名，经 freeStateCapabilityBlockedGatePhrase）；capability_boundary 维持现状。

## 文件域

`agent/internal/chat/goalrunner_chat.go`（freeStateCapabilityBlockedReply 邻域）+ 测试；越域即停。

## 验收标准

单测（proposal_missing→summary 分支/admission_gate_failed→门文案/boundary 缺省回退三分支）+ 148④ 既有测试零回退；`cd agent && go build ./...` + chat 包 + 全量 `go test ./... -count=1` 0 FAIL + gofmt 净；真栈 chat 腿可选（如跑按 §5 登记）。端测边界回执声明。

## 停止条件

boundary 分层与响应路由消费点语义冲突（发现第三个 boundary 值的既有用户面契约）→ 锚点上交。

- 领取：2026-10-21 18:24 +0800 / origin/main=a2d3d596（=171640d5+155/156 领取提交，代码基线仍=171640d5）/ owner=GLM-5.3-Flash PC ZCode 执行会话（152/153 同会话顺序执行，用户口令指定领取）/ 分支=port/reply-face-boundary-layer-1（worktree D:/Vit_DAW_wt_rfbl1）+ 协调 worktree D:/Vit_DAW_wt_rfbl_stnd_coord（coord/rfbl-stnd-claim）/ 领取提交=本提交；领取核对：卡在 todo、派发已确认、151 已合入（93df0fcd，文件域串行前提满足）、doing 三卡（GEN-SUNO 设计件/155 send_route_workflow 域/156 取证只读）文件域零冲突
- 回执：（commit / 测试名 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
