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

- （执行侧/主管回填）
