# L1-5-ADAPTER-DESIGN-1：A/C 生命周期适配契约提案（D 前置，零生产代码）

- 发卡：Codex 辅助决策流 / harness 共享池审查会话 / 2026-10-09（Asia/Shanghai）。
- 派发确认：已确认。用户本轮“可以继续推任务”授权后续发卡；本卡仅完成接口取证与可评审提案，独立执行流按用户指定卡 ID 领取。
- 验收负责人：现有主管决策流；接口变更、C 阶段边界及 D 排程由主管最终裁定。提案不自动成为现行契约。
- 目标仓库：D:\Vit_DAW（或 Mac 同仓 checkout）。
- 优先级 / 预估 / 依赖：P1 / 45-60 分钟 / 无实现依赖；A/C 已在分支提交、复核证据齐全。可与取消修复并行；不等 A/C 合入、不解锁 D。
- 文件域：`coord/runs/L1-5-ADAPTER-DESIGN-1/PROPOSAL.md` 及同目录最多一份机械取证工具/输出、本卡状态。源码、现行 docs、原 A/B/C 卡、rulings、资源记录均只读。
- 并行与资源：与 CANCEL-FIX-1 文件域不相交，使用各自 worktree；零真实栈，不启停进程、不占 PC-RUNTIME-STACK。

## 已核实事实

- 发卡基线 main=d45f6b17；A 原实现 de1d876c，C 实现 c488b811+89fee703。复核报告分别在 7b567089、576c8ca8，原始分支未合入 main。用 git show 读取，不能仅搜索主树后误判实现不存在。
- A 的 FastPathRouter 为 Route(ctx,GoalInput)→FastPathOutcome,bool；命中走 finish/T2。C Router 的 TryMatch(ctx,*Runner,*runState) 返回 stopped/agentloop.Result/name。
- C stopped=true 可能是确认、暂停、失败或完成；false 可能已执行观察、改 Conversation/Context/trace。handler 已执行工具，不能由 A 的 Tools.Execute 再执行一次。
- A 的局部 history 与传给 Router 的原 GoalInput 分离，Prefix/窗口/预算/continuation 也未证明可跨暂停恢复。上述不是名字映射能解决的差异。
- 依据：docs/HARNESS_V1_DESIGN.md §5/§9/§10；C-REVIEW-1 报告 §6；coord/reports/2026-10-09-L1-5-AC-review-audit.md。取消修复卡只解决局部中断，不决定本卡新接口。

## 一个目标

形成主管可直接选择和拆实现卡的最小适配提案：明确状态、执行、终局各由谁拥有，使旧 preflight 资产进入 pull 时保留确认/暂停、miss 副作用和证据纪律。零生产实现、零新领域编排。

## 必交内容

1. **状态与结果矩阵**：逐项列旧 agentloop.Result 状态及产生源码锚点，映射到继续/暂停待确认/真实终局/失败/取消；写出是否有 pending 工具、是否需要 continuation、T1/T2 谁触发。查真实返回点，不能只照已有报告抄矩阵。
2. **两方案与推荐**：比较“agentloop 持有 state 的 adapter”与至少一个替代方案（接口化共享状态或最小结果/状态协议）；说明循环 import、私有类型、文件域和迁移成本。推荐必须含具体签名草案、调用顺序及每个字段权威源，代码片段仅在 PROPOSAL.md 中。
3. **五项不可丢失语义**：miss 时已更新状态传给下一模型轮；confirmation/pause 不触发 T2；已执行工具不重放；真正终局只有一个 retain/EndTurn owner；diagnostic-only 每轮从同一个 root/nested 判定源刷新。每项给最小反例与拟测断言。
4. **成本和恢复边界**：Router 已执行的成本如何计入同一预算；何处检查、何处结算；暂停携带哪些 state、成本和 cycle 身份。真实权限/证据准入/工程版本/VSP 幂等继续落在哪些既有调用点。未知或字段不足明确列待裁定，不能宣称现接口已经支持。
5. **给 B/D 的文件域地图**：列需要的最小生产文件与测试文件；核对正在进行的取消修复域，若需改 A/B 冻结接口标为主管裁决点。建议按“接口准备→状态适配→执行与生命周期→G3”切分，≤5文件/卡；命令尚未存在时标为计划入口。
6. **提案验收表**：至少覆盖终局命中、暂停命中、确认命中、miss 有观察副作用、无匹配、取消、重复续跑、diagnostic-only 八个场景；每场景明确工具执行次数/上下文变化/边界事件/成本与结果分类的预期。

## 核验与停止条件

- 首先读取当前卡池、原提交和复核报告，记录实际 inspected HEAD；设计锚点至少复核8处且给原提交路径/行号。纯文档不跑全量 Go 或真栈，不为凑门槛新增镜像实现测试。
- PROPOSAL.md 标注“提案，未定版”；不得修改 HARNESS_V1_DESIGN.md 或宣告 C 已验收。主管采信后才更新规格、授权接口变更和立 D 实现卡。
- 若 C 已被最终返工/修订，或新 evidence 推翻原依赖：停止旧提案，记录新版本与不成立原因上交。读取能力不足则如实报告，不凭假设画接口。
- 回执含报告 commit、被读 A/C 版本、锚点表、方案推荐与待裁决点、建议卡拆分及覆盖边界。报告走本卡分支，状态按共享协议同步 main。
- 本卡完成只证明提案齐备、锚点可查，不证明 A+C 集成或恢复幂等；不新增用户手测义务。

- 领取：2026-10-09 14:01 +08:00；用户本会话明确“执行 L1-5-ADAPTER-DESIGN-1，按最新卡面和共享协议领取”；origin/main=b07da14ccf31930a318f5cf9c2782011768a1692（重新核对：取消修复已独立领取，文件域不交）；owner=Codex GPT-6 / Windows ZTY / 会话 01a11f3f-41df-7411-acad-4b9a90cb5d0d；执行分支 port/l1-5-adapter-design-1；worktree C:/Users/timoz/.codex/worktrees/l1-5-adapter-design/Vit_DAW；执行领取基线 HEAD=7e230ab92924295cba5f01cd812f7afbb10e4104，status/diff stat 均空；协调 checkout D:/Vit_DAW_worktrees/l1-5-adapter-design-1-coord；领取提交 hash 在回执记录（避免自引用）。本卡零真栈。
- 领取同步确认：6b7c088fa5c0680244e7c812d2f831e30d972f74 已成功普通 fast-forward 推 main，fetch 重读本卡 owner 一致。协调初次目录缺失使 git mv 未完成；没有形成提交或授予所有权，重读取消修复领取 b07da14c 后建 doing 目录、完成本卡领取；未使用 force/rebase/amend。
- 回执：2026-10-09 15:09 +08:00，执行自验完成，待主管设计裁决。报告分支 origin/port/l1-5-adapter-design-1；报告 commit=26ba7edccc7a5f6c4ee61827990895cab37fdb0f，已推送，仅两文件：coord/runs/L1-5-ADAPTER-DESIGN-1/PROPOSAL.md、inspect.go。被读 A=de1d876c，C=c488b811+89fee703；复核报告=7b567089/576c8ca8；取消修复47ce5d75已读diff，未改接口、仍待主管验收。人工返回点锚点表26组；Go AST机械定位41声明、13状态枚举、108生命周期调用点；go run ./coord/runs/L1-5-ADAPTER-DESIGN-1/inspect.go exit 0，必交内容检查exit 0，gofmt -l工具exit 0无输出，提交前git diff --cached --check exit 0。推荐宿主持有单一state+中性会话协议；区分runtime EndTurn与终局T2、延期draft后唯一Return、miss状态回流、已执行工具不重放、同源diagnostic刷新、累计预算与恢复载体。待裁决6项见提案§8；建议按接口准备→状态适配→执行→生命周期→B配合→D入口→G3拆卡，每卡至多5生产/测试文件，测试入口均明确为计划。最后远端核对main=3f6ba5fb，本卡owner一致、C仍done待审无返工ruling、C分支仍89fee703。端测边界：纯提案零生产源码/现行docs/其他卡/ruling/资源记录改动，未运行全量Go/真实栈，不证明A+C集成、恢复幂等或G3，不解锁B/D、不新增手测义务。执行worktree提交后status空；报告只推port分支，本状态提交仅本卡路径。
- 辅助核查：Codex 发卡流已复跑锚点工具（41声明/13状态/108生命周期调用点）、抽核提交前终局副作用及返回后队列补写。提案交付齐备，建议S1方向；§8六项裁决由主管定版，详见 coord/reports/2026-10-09-L1-5-cancel-adapter-audit.md。非接口变更授权、不解锁B/D。
- 验收：（主管设计裁决 / 后续实施卡 / 现行规格更新另记）
