# FIX-PLAN-STALE-STATUS-1：plan 决策面 stale 分支伪造 goal_status=completed（327 域外同型缺陷）

- 发卡：GLM 主管决策流 / 2026-11-09（依据=[327 勘察](../done/2026-11-09-FIX-GOV-INTERACTION-TTL-1.md) 域外登记——resolvePendingPlanDecision stale 分支[修后文件 server.go:5288-5293]与 interaction 面同型：stale 应答伪造 goal_status=completed；r2 走 interaction 面故 327 未动）
- 派发确认：已确认（327 终验候裁定的预登记；327 真栈复现腿通过后本卡可领）
- 验收负责人：GLM 主管决策流
- 池序 330；P2 / 0.5 天 / 依赖=327 合入（语义先例在：tombstone+两态回执同款形态）；L1 / PC；零栈单测为主

## 目标

1. plan 决策面 stale 分支对齐 327 语义：stale 应答回显「未执行+原因」（retired/not_found 两态同款），**不伪造 goal_status=completed**；
2. 单测：plan 面三态（正常/stale 迟到应答/不存在）镜像 327 的 interaction_retirement_test 形态；
3. 既有 plan 面测试断言随语义修订（红转绿记录）。

## 文件域

agent/internal/chat/server.go（plan 决策面）±interaction_retirement.go 复用+测试；工件 coord/runs/FIX-PLAN-STALE-STATUS-1/。

## 验收标准

三态单测 exit 0 + chat 全包不回退 + 回执附 diff。

## 停止条件

plan 面结构与 interaction 面不同构到需重构 → 取证上交候裁定。

## 领取/回执/验收

### 领取（330）

- 时间：2026-11-09 晚窗
- origin/main 基线：`b9b8c76ae85293b3aaa00d01d1e8a0a70e935932`（worktree 亲证：status 干净、diff --stat 空；含 327 合入 c8bb576e 亲证 ancestor ✓）
- owner：GLM-5.3 执行子代理 / PC / Vit-DAW 330 卡 L2 实现会话
- 分支：`port/plan-stale-status-1`（实现）；领取提交=coord/330-claim（仅含本卡状态变更）
- worktree：`D:/Vit_DAW_wt_330impl`
- 前置核对：卡仍在 todo ✓ 派发已确认 ✓ 依赖 327 已合入 ✓；doing 三卡（ARRLOOP-AUTHBRIDGE-1[harness 装载确认面]、CLOSURE-ROUTE-ADAPT-1[audio_closure_controller.go 预算面+测试，已声明不触 server.go 交互域]、WAVESHELL-LOADING-FORENSICS-1[只读取证+scripts]）文件域与本卡（chat/server.go plan 决策面+interaction_retirement.go+测试）不相交 ✓
- 零栈单测卡：不起真实栈、不占 PC-RUNTIME-STACK。

### 回执（330，done=自验候主管验收）

- **实现 commit**：`655ccacc`（分支 `port/plan-stale-status-1`，已推远端，不合入 main）；领取提交 `9d976961`（coord/330-claim）。
- **工件目录**：`coord/runs/FIX-PLAN-STALE-STATUS-1/`（README 口径+红转绿+回归工件，随 port 分支入库）。
- **diff 清单**（文件域内，3 文件+工件，+463/-24）：
  - `agent/internal/chat/server.go`（+~122/-24）：`takePendingPlan` 成功消费记 tombstone（consumed，含无 canonical id 的退化分支按查找键记录）+stale 别名残迹记 stale_alias（按查找键记录，保留真实 workflow 元数据）；`expirePendingPlan` 显式过期记 expired——仅在真实移除条目时记录，未知 id 不伪造存在性；`resolvePendingPlanDecision` 与 `handleConfirm` 两处 stale/404 分支统一为 `pendingPlanDecisionUnanswerableReceipt` 两态回执：`plan_already_retired`（「这个确认已在此前被处理或撤销（原因）；本次重复的决定未执行，未产生新的修改。」）/`plan_not_found`（「未找到待确认计划 …：它可能从未存在，或已随工作区切换/重启丢失；本次决定未执行，未产生任何修改，目标未因此完成。」），两态 goal_status 均空串、WARN 日志留痕、workflow_data 带 decision_executed=false/mutation_performed=false 结构化字段（镜像 327 的 interaction 两态）；`chatResponseFromPendingPlanDecision` 空 goal_status 回填兜底对两态回执豁免（chat 投影不再把空回填成 completed/failed——红跑揭示的第二伪造面）
  - `agent/internal/chat/interaction_retirement.go`（+23）：`recordPendingPlanRetirementLocked`——plan 面复用 327 同一有界 retirement 台账（plan_/interaction_ 前缀无碰撞，最小 diff 选型=复用而非建平行台账）
  - `agent/internal/chat/plan_stale_status_test.go`（新，253 行）：三态八测镜像 interaction_retirement_test 形态
- **红转绿记录**（卡面目标③，工件在卷）：新增测试对修前代码 8/8 FAIL（red_run_prefix.txt——含登记缺陷活体逐字复现：stale 分支 200+「这个交互已处理或已过期。」+goal_status=completed 伪造）；修复后 8/8 PASS。存量断言核对：全树 `*_test.go` 无测试钉 plan 面旧罐头文案或 plan-not-found 404（grep 亲证；webui interactionGuard 按状态词根做客户端台账、不依赖被删文案）——无存量断言需修订，红转绿以新增测试在卷替代。
- **测试命令与退出码**（worktree D:/Vit_DAW_wt_330impl，go1.26.2，全部确定性零 LLM 零概率面）：
  - `go test ./internal/chat/ -run "TestPlanDecision|TestNewPlanAfterRetirement|TestHandleConfirmUnknownPlan|TestChatResponseFromPlanDecision" -count=1 -v` → **8/8 PASS，exit 0**
  - `go test ./internal/chat/ -count=1` → **ok 141.838s，exit 0**（chat 全包不回退，含 327 六测与全部存量）
  - `go test ./internal/chat/ -run "TestInteractionRespondStale|TestInteractionRespondTimely|TestInteractionSurvivesWorkspace|TestInteractionOlderThanNewer|TestInteractionGuardExpiry|TestNewProposalAfterRetirement" -count=1` → ok，exit 0（327 族跨卡无回退）
  - `go build ./...` → exit 0；`go test ./internal/pendingmanager/ -count=1` → ok exit 0；`go vet ./internal/chat/` exit 0（semantic_treatment_strategy.go 两处 unreachable code=领取基线既有、本卡未触该文件，域外不动）
- **三态对应**：①正常=TimelyApprove 执行+goal completed、重复 approve 得 already_retired 幂等回告；NewPlanAfterRetirement 退后再立新卡可应答不受污染；②stale 迟到应答=ExpiredPlan（guard 形态得 expired 原因回执）/StaleAliasRemnant（旧 stale 窗口得 already_retired）/LostToWorkspaceReplace（r2 形态快照替换得 not_found 显式回执，目标未因此完成）；③不存在=UnknownPlan+HandleConfirm HTTP 面（修前 404 裸错→修后 200 显式回执）。
- **零栈声明**：未起真实栈、未占用 PC-RUNTIME-STACK、无 run ID 无计费。
- **端测边界声明（AGENTS §5）**：本卡自验=Go 单测面（chat 全包绿）；真实栈面/渲染面/用户旅程未测——两态回执与 327 已真栈活体证成的 interaction 面同构（确定性服务端逻辑），webui 呈现新两态文案未起栈验证，候主管裁定是否需搭后续栈窗口便车腿。
- **域外同型登记（不扩修）**：interaction 面三个 workflow 专属恢复兜底（修后 server.go:4834「这个段落包络确认已处理或已过期。」/4885「这个混音单步确认…」/4956「这个混音确认…」）仍伪造 GoalStatus=completed——形态=恢复路径找到 interaction 但 workflow 续体未被处理时的回退（327 interaction 域内遗留、非本卡 plan 域），均未触碰，候主管决定是否另开小卡。
- 跨重启持久化语义不变（retirement 台账为 in-process，重启后行为同今天）；六条 payload 恢复路径语义不变。
