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
