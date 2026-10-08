# FS-CAPABILITY-BLOCKED-SURFACE-1 回执（PC 执行侧，2026-10-08 早窗→晚窗）

任务卡：`coord/cards/doing/2026-10-07-FS-CAPABILITY-BLOCKED-SURFACE-1.md`（RECON-1 修复卡；capability_blocked 显式边界响应——gate 拒绝不再被散文终答吞没）

## 实现与提交

- **实现 commit：`632776c2`** @ 分支 `port/fs-capability-blocked-surface-1`（独立 worktree `D:/Vit_DAW_wt_fs_cap`，并行纪律：决策侧占主树、另一执行侧只读复核，全程未触碰主树；本回执为随附提交）
- base = `477e04cd`（worktree 建自派发指定 commit；领取时工作树净）
- diff 规模：**1 个生产文件 +87/-0（纯插入，既有行零改写）+ 1 个新测试文件**
  - `agent/internal/chat/goalrunner_chat.go`：
    - 路由门 `runAgentLoopChat` plain 分支（audioClosure 子分支之后、`chatResponseFromAgentLoopResult` 兜底 return 之前）加 capability_blocked 显式边界分支（+9 行含注释）
    - 4 个新生产函数：`freeStateCapabilityBlockedStringList` / `freeStateCapabilityBlockedBoundary`（检测）/ `freeStateCapabilityBlockedReply`（边界文案）/ `capabilityBlockedBoundaryResponse`（响应组装）
  - `agent/internal/chat/free_state_capability_blocked_surface_test.go`（新增，4 测试）

## 检测条件与卡面口径对账（实现裁定申报）

卡面条件="LatestDecision.Status=capability_blocked（**或** admission receipt boundary=admission_gate_failed）"。实现取：**LatestDecision.Status=capability_blocked 且 loop 已在边界终态（status ∈ {capability_blocked, blocked}）**。理由：代码普查证实每个 admission_gate_failed 结算形二者必然同现（gate 拒绝出口 `free_state_reasoning_loop.go:1054-1077` 先重写 decision 再落 receipt）；而"独立 receipt 触发"会劫持两个合法面——方案乙 parked loop（awaiting_experiment，receipt 合法保留 gate 拒绝记录）与 judgment boundary（status=blocked + 早期轮 stale receipt）——违反卡面"既有路径零变化"红线。卡面"或"臂的意图（R3 形必被捕获）由主条件完整覆盖；此裁定已由负回归测试钉死（见下）。

响应面 = 卡面三件套：`stop_reason=capability_blocked` + `WorkflowData` 携带 `free_state_admission_receipt`（含 failed_gate_ids 全量；另加 `capability_blocked:true`、`mutation_performed:false` 边界标记）+ 回复文本为确定性边界陈述（点名未过门 ID/模型申报边界，明示"没有执行任何实验或修改，也不会自动重试"）。**durable 信封语义零改动**（`recordGoalResult` 照旧、GoalStatus 沿 res.Status——与既有 terminal inactive-resume 响应 `goalrunner_chat.go:316-321` 同惯例：边界由 stop_reason 载机器面）；deferred `bindSchedulerChainDecisionReceipt` 只加 WorkflowData 键不碰 StopReason（已核）。

## 测试（新边界路径 + 既有路径回归）

| # | 测试 | 断言面 |
|---|---|---|
| 1 | `TestGateRefusalSettlesCapabilityBlockedLoopShape` | **经生产 `recordFreeStateDecision` 真实造形**（R3 忠实夹具：预折叠 frontier 无 track 覆盖+目标轨证据在账本→G6/G8 双拒、G7 过）：loop.Status=capability_blocked、decision 止因=free_state_admission_gate_failed、receipt boundary=admission_gate_failed、failed_gate_ids 含 G6_target_evidence+G8_target_consistency 且不含 G7 |
| 2 | `TestCapabilityBlockedBoundaryResponseSurface` | 三面：stop_reason=capability_blocked；WorkflowData.receipt（boundary+G6/G8 在位）+边界标记；文本点名拒绝+无模型散文泄漏（断言不含"A/B 试听/执行后请"）+无假成功词+明示不执行不自动重试；GoalStatus=durable 信封不变（在案注释） |
| 3 | `TestCapabilityBlockedBoundaryDetectionKeepsExistingPaths` | 四负回归：awaiting_action / awaiting_experiment（方案乙 parked+stale gate receipt 不被劫持）/ plain completed / judgment boundary（blocked+settle report）全部不触发 |
| 4 | `TestCapabilityBlockedBoundaryDetectsModelTerminalConcession` | 模型自申 capability_blocked 终局形（loop.Status=blocked、无 receipt）被检测；不虚构 gate 失败；文本含模型申报边界+不执行声明 |

## 验收门

| 门 | 结果 |
|---|---|
| `cd agent && go build ./...` | **exit 0** |
| `cd agent && go test ./... -count=1` | **exit 0——90 包全 ok，0 FAIL**（与 L1-4 基线包数一致） |
| gofmt | 本卡两文件 LF 归一后 gofmt **零新增偏差**。基线固有偏差两处（`goalrunner_chat.go` CJK Reply 对齐 @290、continuation 键 @2029，477e04cd pristine blob 同样带）保留未动（不在本卡域）；worktree `core.autocrlf=true` 使 CRLF checkout 的未触碰文件也被 `gofmt -l` 全库标记——checkout 伪象（IMPL-C 回执同结论） |

## 复现轮（§8 口径，真栈泊位）

命令（两轮同）：`powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev_agent_smoke.ps1 -Scenario journey_free_state_nl -StartKernel -KernelExe D:\Vit_DAW\Export\staging\runtime\VitApp.exe -RunArtifactsDir <run>`；LLM 开关 VIT_AGENT_LLM_MODEL/BASE_URL/API_KEY 自 `~/.vit/config.json`（deepseek-v4-flash @ rightapi）注入调用 shell；泊位脚本自起自拆（内核 pid/agent pid 各轮记录于 run 日志；两轮端口预检空闲）。

| 轮 | run 工件目录 | 退出码 | 结局分类 | 关键证据 |
|---|---|---|---|---|
| 1 | `coord/runs/FS-CAPABILITY-BLOCKED-SURFACE-1/repro1_20261008_184329` | **0** | `auto_applied_no_card`（**全链走通**） | receipt boundary=admitted、8/8 门 pass；mix_tick=1、applied_like=1、audition=4、ab_card=True；nl_goal_status_final=completed |
| 2 | `coord/runs/FS-CAPABILITY-BLOCKED-SURFACE-1/repro2_20261008_184543` | **0** | `auto_applied_no_card`（**全链走通**） | receipt boundary=admitted（×2 continuation）、0 门失败；mix_tick 应用、goal completed；**ab_card=False**（audition 卡未挂——脚本记录面方差，按实记，不影响 exit-0 门） |

- 失败分类：两轮均无失败形；**capability_blocked 形未触发**（概率面：RECON N=4 分解中该形占 1/4，两轮采样未命中属预期），无同形失败，止损线未触发。工作树被测=本分支实现后代码（脚本 `go build` 现建 VitAgent.dev-smoke.exe，非 -SkipBuild）；内核=主树 `Export/staging/runtime/VitApp.exe`（Oct 6 22:02 构建，本卡零内核改动，泊位隔离工作区运行）
- 被拒形未真栈复现的边界：新分支的真栈 wire-in 本轮未被概率触发；分支挂点为单 if 且检测函数已被测试 1/3/4 直测（生产检测路径），真栈该形的端到端表现留待该形自然出现或决策侧复跑采样

## 红线对账

1. **G1-G8 门逻辑零改动**：`agentloop` 包零 diff（`git diff 477e04cd -- agent/internal/agentloop` 为空——门评估、审计入口、bounce 语义全部原样）
2. **improvement_proposal_workflow.go 零改动**：该文件零 diff；挂卡路径（awaitingExperiment→improvementProposalResponse）行为不变，方案乙 parked 形由负回归测试 3 显式钉住
3. **awaitingAction/awaitingExperiment/plain 三既有路径零变化**：实现为纯插入（+87/-0），检测不命中时逐字节走原 plain return；负回归测试 3 + 全量 90 包 0 FAIL 背书
4. **语义红线**：无自动重试（分支为终态响应，不发起新 turn）；不绕过 G 门（门拒绝本身原样生效，修的只是响应面"失败形表面化"）；回复文本明示不执行不重试

## 端测边界声明

- 本卡改 **agent 响应组装面**（chat/goalrunner_chat.go），无 webui/渲染面/用户旅程改动；旅程烟测两轮 exit 0（deterministic 子面全过）
- 真栈 capability_blocked 形未被概率触发（见复现轮节）——该形的真栈端到端断言（探针分类面）未采得，如实申告；烟测脚本分类器（`dev_agent_smoke.ps1:3451`）把 capability_blocked 止因归入 no_candidate_found 族为**预存粒度**（RECON-1 §7 待办 a，脚本不在本卡文件域，未动）
- 卡面可选取证腿（nudge 重试腿）**未做**（卡面允许留后续卡）；探针分类修正与 DEBUG 级 prompt 留痕（RECON-1 §7 附带待办 a/b）不在本卡域，留决策侧

## 领取信息（§12）

- 领取 HEAD=`477e04cd`，worktree 领取时 `git status --short` 净；实现期间主树与他侧改动零接触、零 rebase/amend
- 本回执工件：`coord/runs/FS-CAPABILITY-BLOCKED-SURFACE-1/`（两轮 run 目录+本 receipt；run 目录不入库，receipt 随分支提交）
