# Ruling: SETTLE-CHAIN-1 — pass（2026-09-28 决策侧）

## 裁定

**pass**。settle 链四断裂修复+烟测时序对齐验收通过，真栈 exit 0 恢复主烟测通道（09-12 以来首次）。实现合入 main（cherry-pick a3f20ec0 → 11ff8984）。**DOSE-AUDIBLE-1 的端测门回补条款随本卡闭合**（见下）。

## 决策侧复跑与亲核

- **exit 0 亲核**：直读 run 20260928_202511 的 d1_smoke_report.json——status=pass、error=None、四旗标全 true、decision=user_judgment_pending；四轮 run 目录（194926/200332/201712/202511）俱在，已归档 `coord/runs/SETTLE-CHAIN-1/`。
- **§8 纪律亲核**：每版本每断点 ≤1 次即取证修复（194926 断裂②→200332 断裂③→201712 断裂④→202511 PASS），无原样重跑。
- **烟测脚本改动审定**：+33 行全部为**带 deadline 的 settle 记录轮询**（poll_persisted_loop_for_settled_round），验证断言零变更——settle 不落地则轮询到 deadline 后 pre-settle 投影诚实失败。"断言不弱化"声明成立。docstring 带 run 取证引用，可回溯。
- §9 口径齐全：agent 二进制 sha16=9E43F171D4C1E8A4（worktree 20:17 构建，含全部修复）；内核 sha16=A29751807DC425A6（11:51 主树，C++ 零 diff 属实）；-SkipBuild 二进制对应关系无瑕疵。
- 合并态 main 全量门与触碰文件 blob 级 gofmt：决策侧复跑（结果记入验收注记）。

## 根因链采信

四断裂（①判定边界悬空致 settle 决策被迁移表拒绝→recordFreeStateDecision 早退；②post-action 观察 booking 单发碰 CCB 旧 revision 竞态；③回放降级破"最后一条必须 post-action"不变量；④烟测 break 条件按 pre-GAP-1 时序写就）各有独立取证 run 与代码锚点，修复与断裂一一对应，红绿四钉先红后绿。取证质量：未重跑既有取证、从工件与锚点切入（卡面要求遵守）。

## DOSE-AUDIBLE-1 端测门闭合（回补条款兑现）

run 202511 的被测栈=main 含 DOSE 全部改动（分支基 1f4bd906）+ 本卡修复，内核同一（A29751807DC425A6）——**该 exit 0 同时构成 DOSE-AUDIBLE-1 的端测门闭合证据**（其 ruling 附条款自动兑现）。DOSE 卡裁定从"附端测门回补条款"转"完全闭合"。

## 遗留观察（不阻塞，转常态观察）

- settle 片 latest_decision 曾短暂呈现 pre-settle 形态后终态正确（202511 终态亲核无误）——记为观察项。
- D2 multi-round 路径未触（单轮门内）——multi-round settle 行为不在本卡验收面。
- 执行侧 worktree 已清（取证工件归档后）。
