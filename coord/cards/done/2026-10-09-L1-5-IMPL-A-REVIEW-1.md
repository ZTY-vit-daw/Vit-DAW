# L1-5-IMPL-A-REVIEW-1：PullLoop 骨架独立复核与八项实现裁决取证

- 发卡：Codex 辅助决策流 / 本次共享卡池与 harness 审查会话 / 2026-10-09（Asia/Shanghai）
- 派发确认：已确认发卡（用户本轮授权本会话作为决策流发卡；范围为已完成 harness 交付的独立复核，零生产实现）。执行仍由用户在独立执行流指定“执行 L1-5-IMPL-A-REVIEW-1”后领取。
- 验收负责人：现有主管决策流；Codex 发卡流可核查报告，最终 A 卡裁定与实现合入仍交主管。
- 目标仓库：D:\Vit_DAW；Mac 可使用其同仓 checkout（本卡零真实栈、零 Windows 专属生产行为）。
- 优先级 / 预估 / 依赖：P1 / 45-60 分钟 / 无实现依赖；被审 A 已在 done，但未见最终 ruling。建议首选复核 A，以提供 B 解锁所需的证据；不修改原池序或解锁原卡。
- 文件域：只写 `coord/runs/L1-5-IMPL-A-REVIEW-1/REPORT.md` 与同目录日志、以及本卡状态字段。所有源码只读；测试在本流独立 worktree 内执行。不得改原 A/B/C 卡、docs、rulings 或真实工程。
- 并行与资源：可与 C-REVIEW-1 并行（报告域不同，源码只读，各自 worktree）；不需要真实栈，不启停 VitApp/Godot/agent，不占用 PC-RUNTIME-STACK。

## 已核实依据与待证问题

- 发卡基线：main `2691f9279487e2c4f5018c8d0b143f6de0d38d8b`。主树已有 Settings.xml、default_project.xml、contextruntime/history_refs.go 改动；不得搬入复验环境或处理它们。
- 被审实现：`de1d876c`（port/l1-5-impl-a），执行基线 `4c2f75596e3571ab9f1ef3eccdaeb6fd25bbdbc8`。原卡 `coord/cards/done/2026-10-09-L1-5-IMPL-A.md`；回执在被审提交 `coord/runs/L1-5-IMPL-A/receipt.md`，主树尚未合入该报告。
- 已核实：六个新源码/测试文件，19 个测试申报；零生产入口消费申报；回执末段列出八项实现裁决，均待独立复核。
- 待证：七步循环是否符合 `docs/HARNESS_V1_DESIGN.md` §2-§3；八项裁决是否是合理的接口适配、卡内缺陷，或需主管进一步裁定。不能以执行侧“91 包全绿”替代检查。

## 执行步骤

1. 读取 AGENTS.md、coord/PROTOCOL.md 和原卡；fetch 后重查 A 的验收状态。若 A 已最终验收，或被审提交被修订，先报告新状态，不复审过期交付。
2. 按共享协议领取本卡并同步 owner。建立独立 review worktree/分支（建议 `port/l1-5-impl-a-review-1`），从领取时确认的 main 基线应用 `de1d876c` 用于集成复验。不复用执行作者的 worktree；应用冲突先记录并上交，禁止为过测试修代码。
3. 读 `git show --stat de1d876c` 和 `git show de1d876c -- agent/internal/pullharness`，对照原卡逐项核验，并核查 promptruntime/contextruntime/agentprotocol 与旧生产入口无本卡修改。
4. 报告用表格逐条复核回执八项裁决：LLM 接口、GoalInput/Result、ClassifyTerminal、暂停/取消、T1/T2 时序、预算检查点、Router 短路、非法 env。每条列源码锚点、设计依据、测试覆盖、结论与缺口。
5. 重点区分：无工具调用缺省 judgment_ok 是否仅骨架占位；ProbeCost 批后止损与执行前限额的实际边界；ctx 取消是否仅有会话载体而未证明恢复幂等；T1 是报告产生还是已经出窗。未接线部分不能宣称生产闭环，也不能仅因原卡明确留给 B/D 的槽位为空而判 A 失败。
6. 复跑下列命令，保存原始输出和直接退出码；用实际 diff 列出的六个 Go 文件核对 gofmt。不要修改既有或新增测试来迎合实现。

## 验收命令

在 review worktree 的 `agent/` 下运行：

```powershell
go build ./...
go test ./internal/pullharness -count=1 -v
go test ./... -count=1
gofmt -l internal/pullharness
```

- build、包级与全量测试期望 exit 0；包级须核对实际测试数量和名称。gofmt 输出须区分提交内容偏差与本机行尾转换，报告证据，不写回文件。
- 报告至少含：被审提交与复验 HEAD、领取状态/实际集成 diff、命令/退出码、八项裁决表、原卡验收覆盖、问题清单、建议结论、B/D 接线前必须核对的事项。
- 建议结论为“建议通过 / 建议补证 / 建议返工 / 阻塞”；每个问题分为 blocker、应修或建议，给可复查锚点。报告可建议后续卡，不创建或实现它们。
- 本卡完成只表示独立复核完成，不等于 A 最终验收，也不解锁 B；主管核查报告后另行裁定。

## 停止条件与交付

- 版本/设计前提改变、集成冲突或源码缺陷导致复跑失败：保存证据，交付建议返工/阻塞报告，停止继续实现；诊断报告可据有效失败证据完成本卡。
- 不稳定测试按 AGENTS §11 留首次输出、隔离复跑和整包复跑再分类；不得仅凭重跑成功抹去失败。环境故障单列，不改写为功能通过。
- 回执与报告按既有分支协议提交，回填实际 commit、基线和命令退出码；本卡状态按共享 main 同步，报告分支交主管核查。禁止在 main 合入被审实现或出最终 ruling。
- 端测边界：本卡纯审查/复跑，无生产变更，不新增烟测豁免；原 A 的零生产消费边界需实证，真实 G3 收口继续归后续卡。

- 领取：2026-10-09 13:35 Asia/Shanghai（原领取提交手填 13:38，按实际移动时间 13:35:35+08:00 纠正） / origin/main=a25d3b9d71f38eb8ffbec7019e5f986ac9871b83 / owner=Codex GPT-6 / Windows PC ZTY / 会话 01a11f26-bbcd-7d71-8a9d-909d10107633（用户明确指定执行本复核卡）/ 复验分支 port/l1-5-impl-a-review-1 / 独立 worktree C:/Users/timoz/.codex/worktrees/l1-5-a-review-coord/Vit_DAW / 协调分支 codex/l1-5-impl-a-review-1-coord；领取提交 2986df4f（已推共享 main 并核对远端 owner）。领取基线 HEAD=a25d3b9d，status --short 与 diff --stat 均为空；主树已有改动未带入。本次 fetch 确认 A 无最终 ruling、origin/port/l1-5-impl-a=de1d876c8b8e3e453d69b138e22bf7425df4f558；doing/blocked 无在飞卡。
- 回执：独立复核执行完成，待主管审阅。报告分支 codex/l1-5-impl-a-review-1-report，commit 7b567089af522b6006552870bd79d41b5d87a908（仅本复核目录 18 份报告/证据；已推送，未合入 main）。报告 coord/runs/L1-5-IMPL-A-REVIEW-1/REPORT.md。被审 de1d876c；main 集成复验 HEAD=eca150926b47573f012c5c6faffb65ea7e5ebff6（只在本地 port/l1-5-impl-a-review-1）。build exit 0；原包级 19/19 PASS exit 0；全量 91 包 ok/0 FAIL exit 0；gofmt 六提交 blob 均净，本机列名仅 CRLF；只读 overlay 取消反例三项失败 exit 1，已保留原始输出及诊断输入。建议返工（取消检查遗漏导致继续执行/错误预算终态/T2）；Router 预算及 nil Prefix 跨轮问题单列建议。无源码或原测试修改、无真实栈、无最终 ruling、不解锁 B。
- 验收：**pass（2026-10-09 主管决策侧）**——三反例独立复现坐实 A 缺陷、"建议返工"被 CANCEL-FIX-1 闭环采纳=本链最有价值一环；单列三点（取消文档化/Router 预算契约/nil Prefix）归 D 输入集。见 rulings/2026-10-09-EVENING-BATCH-rulings.md §2
