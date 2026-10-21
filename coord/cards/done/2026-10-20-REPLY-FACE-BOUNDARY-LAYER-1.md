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
- 回执（**实现+自验腿完成 / 2026-10-21 18:5x，实现 4a414158@port/reply-face-boundary-layer-1（领取基线 4ce24e41，代码=171640d5；领取时 worktree 全新检出零既有 diff）**）：
  - **实现**（goalrunner_chat.go +15/−4，函数邻域内）：freeStateCapabilityBlockedReply 按 receipt.boundary 分层——proposal_missing/proposal_invalid 直达 decision.Summary 面（不再以 failed_gate_ids 渲染「提案未过准入门」的事实错误文案、不再压模型可操作原因；缺 summary 落中性面）；admission_gate_failed 维持门文案（freeStateCapabilityBlockedGatePhrase，148④ 契约零变）；capability_boundary 与未知/缺省 boundary 维持分层前行为（gate→summary→中性，decision.StopReason 可变值安全缺省）；boundary 读取与 remediation 消费点同式（firstNonEmpty(firstStringFromMap(receipt,"boundary"),"capability_boundary")）。
  - **测试**：新增 free_state_reply_face_boundary_layer_test.go 五测——TestCapabilityBlockedReplyProposalMissingTakesSummaryFace（146 取证 E5 receipt 形态：summary 面+零门文案+零 codename 泄漏+缺 summary 中性面）/TestCapabilityBlockedReplyProposalInvalidTakesSummaryFace/TestCapabilityBlockedReplyAdmissionGateFailedKeepsGateFace（门文案精确匹配+门面压 summary=148④ 语义）/TestCapabilityBlockedReplyBoundaryDefaultKeepsCurrentBehavior（boundary-less/capability_boundary/双空三态现状）/TestCapabilityBlockedBoundaryResponseProposalMissingFace（端到端 F4 面：经 capabilityBlockedBoundaryResponse 全响应断言 stop_reason+workflow receipt+summary 面文案+零泄漏）。148④ 既有测试零回退（TestCapabilityBlockedReplyRendersGateNamesWithoutCodenames/TestCapabilityBlockedBoundaryResponseSurface/TestCapabilityBlockedBoundaryDetectsModelTerminalConcession 等全绿——boundary-less receipt 走缺省分支维持现状）。
  - **命令与退出码**：`cd agent && go build ./...` exit 0；chat 包边界族 `-run 'TestCapabilityBlockedReply|TestCapabilityBlockedBoundaryResponse|TestGateRefusalSettles…'` 12 测试全 PASS；`go test ./... -count=1` 全仓 **exit 0 零 FAIL**；gofmt 净（LF blob 口径——工作树 autocrlf=true 全包 CRLF，去 \r 后 gofmt -l 空）。
  - **端测边界声明**：本次覆盖=单元+包内端到端响应面（真 Server 面 capabilityBlockedBoundaryResponse）；**真栈 chat 腿未行使（卡面标注可选）**——回复面为纯文案选择零新运行链路，生产消费点唯一（capabilityBlockedBoundaryResponse→resp.Reply），receipt.boundary 由 free_state_reasoning_loop.go:732-744 单点生产（E5/F4 形态经单测复刻）；webui/渲染面零改动不涉渲染烟测；真栈腿若验收侧要求补跑按 §5 登记。
- 验收：2026-10-21 晚 / GLM 主管决策流 / **整卡 pass**：实现 4a414158 cherry-pick 入 main（bbea3445）；亲读三分支实现（proposal_missing/proposal_invalid→summary 面、admission_gate_failed→门文案、缺省/未知回退现状序）与规格逐条对上；五新测+148④ 既有边界族点名复跑全 PASS（boundary-less 走缺省分支现状保持）；集成态全量 go test -count=1 0 FAIL+gofmt CR 剥离口径净；真栈 chat 腿卡面标注可选未行使——裁定单测+包内端到端响应面足以交付（纯文案选择零新运行链路，生产消费点唯一）；验收 commit=本提交
