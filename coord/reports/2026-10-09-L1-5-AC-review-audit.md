# L1-5 A/C 复核交付核查（辅助决策侧，待主管终审）

- 日期：2026-10-09，Asia/Shanghai；授权：用户交回两张已完成复核卡，要求检查。
- 卡池基线：origin/main=6da1492dc2bf736417eafe56c307b3f65e844be0，主树已安全 fast-forward；原有 XML/history_refs.go 改动保留。
- 角色：Codex 辅助决策流。本文是独立证据核查和建议，不是最终 ruling；未合入 A/C 实现、未解锁 B。

## 结论

两份复核报告均达成发卡目标，建议主管接受其取证交付。A 原实现建议返工；C 旧消费面正常注册等价成立，但须主管确认阶段边界与 D 接线契约后再最终验收。

| 对象 | 原始报告提交 | 核查结果 |
|---|---|---|
| A-REVIEW-1 | 7b567089af522b6006552870bd79d41b5d87a908 @ origin/codex/l1-5-impl-a-review-1-report | 报告 18 文件均在本复核目录；八项实现裁决、原始命令与反例齐全；全量日志实际 91 包 ok/0 FAIL。取消三反例独立复现，建议返工有据。 |
| C-REVIEW-1 | 576c8ca8d26cf8d8cbbc04da9cd24aa8e9c7d4a9 @ origin/port/l1-5-impl-c-review-1 | 报告 63 文件均在本复核目录；首次失败、隔离/整包和确认复跑均保留；机械等价、原调用序、实际接口限制可回查。该分支祖先含被审实现，禁止未经裁定整支合入 main。 |

## 我方独立复跑

新建 managed worktree `C:/Users/timoz/.codex/worktrees/harness-review-evidence/Vit_DAW`，没有复用执行作者 checkout，也未带入主树改动。A 测完确认 tracked diff 为空后切换到 C 复验提交；输出保存到新目录 `coord/runs/L1-5-REVIEW-AUDIT-20261009/`，不覆盖两执行侧工件。未启动真实栈。

| 命令/检查 | 被测 HEAD | 结果 |
|---|---|---|
| go test ./internal/pullharness -count=1 -v | eca150926b47573f012c5c6faffb65ea7e5ebff6（A 原 blob） | exit 0，原 19 测试通过；a-original-tests.txt/a-results.json |
| go test '-overlay=../coord/runs/L1-5-REVIEW-AUDIT-20261009/probe-overlay.json' ./internal/pullharness -run '^TestReview' -count=1 -v | 同上 | exit 1（预期诊断失败），三条取消反例同形复现；a-cancellation.txt/a-results.json |
| go run coord/runs/L1-5-REVIEW-AUDIT-20261009/compare.go | 124a643ddafaf3c81f31772b3c772b301ca27b23（C 集成复验） | exit 0，15/15 函数等价=12 平移+3 副本；10/10 注册顺序；446/446 保留函数相同；c-mechanical-results.json |
| go test ./internal/fastpath ./internal/agentloop -count=1 | 同上 | exit 0，两包通过；c-tests.txt/c-test-result.json |

机械工具亲读：Go parser/scanner 只映射声明的 IDENT token 与 LF 行尾，未忽略任意空白/字符串/注释。副本计数、名单与原调用表达式对照成立。417/8334/83 闭包规模申报仍未独立复现，不能作为已证事实。

全量 build/test 证据来自两执行侧原始工件，已核对直接退出码、包数、被测版本和文件域；本轮没有重复跑全量。以上“我复跑”与“读取执行侧证据”分开陈述。

## A：已证缺陷与后续边界

- 原实现 de1d876c 的 loop.go:169 在检查 ctx 前调用 Router；模型返回成功时未复查取消；工具执行后没有中断出口而立即进行预算/终局处理。
- 已取消入场仍 Router 调用一次，Outcome=fastpath 并 T2；模型返回时取消仍执行工具一次，Outcome=budget_exhausted 并 T2；工具阶段取消也落预算终态并 T2。三条均违反原卡“暂停零 T2”的既有契约，不是要求新增功能。
- 建议先由主管采信取消问题并派修复。待确认修复卡已写入 todo：L1-5-IMPL-A-CANCEL-FIX-1；发卡不解锁领取，不改原 A/B 状态。
- Router probe 预算旁路、nil Prefix 每次重建、终态判断槽、真实退场消费与 continuation 仍需分项归属：前两项交主管定边界；后三项按 B/D 既有职责验证。不把槽位未填整体判作 A 失败，也不宣称骨架已完成生产闭环。

## C：建议接受的阶段边界与主管裁决点

1. 当前形态可称“旧宿主注册面归并+纯 helper 平移”，不称“独立全族执行库”。正常旧消费面没有发现行为回退；实际 12 平移而非原回执 11。
2. 三副本当前相等，主管可接受为临时边界，但要记录双源漂移风险。运行时 panic 是新增配置漂移分支，不能纳入绝对行为零变化；目前无普通用户输入触发证据。
3. **D 前置接口裁决是实质阻塞**：C stopped=true 包含暂停/确认/完成；false 也可能改状态并已执行工具。A Route 命中一律 finish/T2，输入仅原 GoalInput。因此不能仅做签名包装；须约定暂停分类、miss 状态回写、工具执行唯一所有者、终局退场唯一所有者和当前预算/上下文的传递。本文不裁决新接口。
4. C 首次全量失败为 TestAutomaticProposalTextConfirmationKeepsTaskIdentityAndPendingProjection 的 TempDir cleanup，原日志有据；隔离/整包/全量确认绿不抹去首次失败。09-30 FIX-CHAT-TMPDIR-FLAKE-1 已修两名测试，明确将 improvement_proposal_workflow 列为尚未推广暴露面；该历史记录增加取证依据，但不能替代定位本轮后台写者。

## 主管接手顺序

先采信两份复核取证，再裁定 A 取消返工范围和 C 阶段偏差。A 修复验收并合入之后 B 才按原依赖领取；D 开工前定适配契约，避免把生命周期缺陷推迟到真栈 G3 再发现。全程不新增用户手测义务。
