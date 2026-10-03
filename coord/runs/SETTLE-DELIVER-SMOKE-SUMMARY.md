# SETTLE-DELIVER-1 真栈烟测取证索引（2026-10-03，PC 执行侧）

脚本：`scripts/settle_deliver_smoke.ps1`（本卡新增，d1_stall_repro_smoke 模式：kernel+agent 真栈、隔离二进制/draft 根、端口独占、工程哈希前后核对）。§8 运行参数声明在脚本头注释（1 全链尝试 + 环境中断重试；判型 judgment_not_armed/settle_message_missing/round2_stale/round2_failed/timeout/env_failure；同型连败两次止损）。

## 运行台账（全部工件在本机 coord/runs/，未全量入库；入库见下）

| run | 结局 | 定性 |
|---|---|---|
| 110730 | chain_failed | LLM 供给中断（agent_last.log 11:12:38 err=true 60s 超时→续跑片 failed），环境类 |
| 111943 | （中止） | 脚本 ANSI 中文注释致 PowerShell 解析失败（本轮零栈影响） |
| 112607 | chain_failed(409) | 脚本竞态：audition.ready 即 POST，未等 trajectory.user_judgment.requested |
| 113348 | settle_message_missing | 脚本断言面错：/agent/state 为压缩面（设计剥 conversation_messages），改 detail=full |
| 114223 | settle_message_missing | **产品链全通过判定→结算**；脚本 needle 内联中文被 ANSI 损坏致断言漏配 |
| 115939 | judgment_not_armed | 链停 post-action 续跑片（invocations-active 暂滞），120s 等待窗不足 |
| 120857 | judgment_not_armed | 同上形态（capability_blocked，模型随机分支；上午取证注明的 Mac D1-SETTLE-TAIL 家族在 PC 的复现） |
| 121934 | settle_message_missing | **结算节点已落盘**（见下）；needle 仍未修（本轮后修复） |
| 122932 | judgment_not_armed | 链走无 A/B 分支（capability_blocked，模型随机分支） |

## 三症状真栈证据链（关键结论→原始工件回指）

- **症状 A（结算确认进对话流）——真栈已证**：
  - 事件面：run 114223 events_final.json seq 25 `trajectory.user_judgment.requested` → 29 `user_judgment.recorded` → 31 `trajectory.settled` → 32 `judgment.settled`（body=结算报告全文）；run 121934 同形。
  - 持久化面：run 121934 draft 工程图 `agent_drafts/draft_20261003T041936_08c63575/.vit_history/.sessions/vitproj_f9f02a…/workspace/state/conversation_graph.json` 节点 `n_20261003T042445_85c8934e`：kind=vit、message_kind=assistant、logical_message_id=`judgment_settle:judgment-8933972d7a2c1d95`、text=「A/B 判定已落账并完成结算：…（判定证据 …）」。**未入库（21MB drafts）**，关键字段抄录于此，磁盘原件在 121934 run 目录。
- **症状 B（二轮 revision stale）——真栈未复验到**（历轮均未走到结算后二轮断言步）：机制面由 Go 单测钉（TestSettledLoopLedgerDoesNotStaleSettleNextGoalClosure RED→GREEN + 守卫反钉），**留用户手测复验点**。
- **症状 C（渲染序）——webui 单测钉**（settleDelivery.test.ts 取证流身份实形回放，2 钉 RED→GREEN），随用户手测目检复验。

## 入库工件

- 114223/：settle_deliver_report.json、prereq.txt、events_final.json、chat1_response.json、runtime_failed.json、agent_last.log（44K）
- 121934/：settle_deliver_report.json、prereq.txt、events_final.json、chat1_response.json、runtime_failed.json
- （bin/agent_drafts/BridgeReplies/MaterializeMetrics 及其余各轮不入库，磁盘保留）

## 烟测门槛声明（诚实边界）

单场 exit 0 未达成：判定上游链路（本卡明示不动 的已验证工作）在 PC 上模型随机分支导致 2/9 轮到达判定席。三症状中 A 已真栈双面证实、B/C 单测红绿+用户手测复验留点。由决策侧裁定：补跑烟测或以分裂证据+手测复验收口。
