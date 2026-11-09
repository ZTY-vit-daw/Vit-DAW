# FIX-RESTARTAGENT-ENV-1：-RestartAgent 环境变量透传（PENDING 分流②）

- 发卡：GLM 主管决策流 / 2026-11-09（依据=gate 2026-11-09 §三 + 322 首跑 024318 取证：编排进程 SEAM env 与重启 agent 的 env 面不一致，f9ef49f2 已修 go test 子进程面，脚本透传面挂账）
- 派发确认：已确认（gate 2026-11-09 §六 首排④ PENDING×4 分流）
- 验收负责人：GLM 主管决策流
- 池序 326；P2 / 0.5 天 / 依赖=零；L1 / PC；脚本域

## 目标

1. 复现取证：`dev_agent_smoke.ps1 -RestartAgent`（及同族脚本重启 agent 的路径）对父进程环境变量（如 `VIT_PLUGIN_CONTROL_EVAL_SEAM` 系）的透传行为——哪些透传/哪些丢/在哪个进程边界丢；
2. 修复：重启链路 env 透传一致化（明确白名单或全量透传，按脚本既有约定择一并写注释）；
3. 验证：设变量→起栈重启 agent→agent 进程内读回（health/debug 面或日志锚点）断言一致；不设变量时行为与现状零变化（回归）。

## 文件域

`scripts/dev_agent_smoke.ps1` ±同族脚本；零 agent 源码；零内核。

## 验收标准

一次真栈轻窗口（可与 282 轻腿同窗）：透传断言 exit 0 + 无变量回归 exit 0；栈协议两条登记。

## 停止条件

透传丢失点在非脚本层（Go agent 自身 env 处理）→ 取证上交接域（agent 源码域另卡）。

## 领取/回执/验收

- （执行侧/主管回填）
