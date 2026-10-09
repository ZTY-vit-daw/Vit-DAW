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

- 领取：（时间 / main 基线 / owner 模型+机器+具体会话 / 分支 / worktree / 领取提交）
- 回执：（报告分支与 commit / REPORT.md 路径 / 命令退出码 / 建议结论）
- 验收：（主管裁定 / 本复核卡验收 commit；与 A 原卡最终裁定分开）
